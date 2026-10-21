package httpapi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/websocket"

	"hypervane/internal/auth"
	"hypervane/internal/config"
	"hypervane/internal/crypto"
	"hypervane/internal/hub"
	"hypervane/internal/poller"
	"hypervane/internal/pve"
	"hypervane/internal/scripts"
	"hypervane/internal/store"
)

const sessionCookie = "hv_session"

type Server struct {
	cfg    config.Config
	store  *store.Store
	box    *crypto.Box
	hub    *hub.Hub
	poller *poller.Poller

	mu        sync.RWMutex
	clients   map[string]*pve.Client
	defaultID string

	insightCache ttlCache

	scriptsOnce sync.Once
	scriptJobs  *scripts.Jobs
	images      *scripts.Images

	// upgradeMu guards one-time root tickets used to start a node upgrade.
	upgradeMu     sync.Mutex
	upgradeGrants map[string]upgradeGrant
	upgradeFails  map[string][]time.Time
}

func New(cfg config.Config, st *store.Store, box *crypto.Box, h *hub.Hub, p *poller.Poller) *Server {
	return &Server{cfg: cfg, store: st, box: box, hub: h, poller: p}
}

func (s *Server) SetPoller(p *poller.Poller) {
	s.poller = p
}

func (s *Server) putClient(id string, c *pve.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clients == nil {
		s.clients = map[string]*pve.Client{}
	}
	s.clients[id] = c
	if s.defaultID == "" {
		s.defaultID = id
	}
}

func (s *Server) removeClient(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, id)
	if s.defaultID == id {
		s.defaultID = ""
		for k := range s.clients {
			s.defaultID = k
			break
		}
	}
}

func (s *Server) Client() *pve.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.defaultID != "" {
		if c := s.clients[s.defaultID]; c != nil {
			return c
		}
	}
	for _, c := range s.clients {
		return c
	}
	return nil
}

func (s *Server) ClientByID(id string) *pve.Client {
	if id == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clients[id]
}

func (s *Server) DefaultConnectionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.defaultID
}

func (s *Server) Clients() []poller.ConnectionClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]poller.ConnectionClient, 0, len(s.clients))
	for id, c := range s.clients {
		out = append(out, poller.ConnectionClient{ID: id, Client: c})
	}
	return out
}

func (s *Server) RestoreClient(ctx context.Context) error {
	id, err := auth.NewUUID()
	if err != nil {
		return err
	}
	if err := s.store.MigrateClusterConfig(ctx, id); err != nil {
		return err
	}
	conns, err := s.store.ListConnections(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.clients = map[string]*pve.Client{}
	s.defaultID = ""
	s.mu.Unlock()
	for _, cfg := range conns {
		if !cfg.Enabled {
			continue
		}
		client, err := s.clientFromStore(cfg)
		if err != nil {
			log.Printf("restore connection %s: %v", cfg.ID, err)
			continue
		}
		s.putClient(cfg.ID, client)
		if cfg.Name == "Default" {
			// Prefer the live cluster name over the migration placeholder.
			if probe, err := probeExisting(ctx, client); err == nil && probe.ClusterName != "" {
				cfg.Name = probe.ClusterName
				if err := s.store.UpdateConnection(ctx, cfg); err != nil {
					log.Printf("rename migrated connection %s: %v", cfg.ID, err)
				}
			}
		}
	}
	return nil
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-Hypervane-Connection"},
		AllowCredentials: true,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		r.Get("/auth/me", s.handleMe)
		r.Get("/auth/realms", s.handleAuthRealms)
		r.Post("/auth/setup", s.handleSetup)
		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/logout", s.handleLogout)

		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Use(s.auditMutations)
			r.Post("/storage/{storage}/upload", s.handleUpload)
			r.Get("/storage/{storage}/file-restore/download", s.handleFileRestoreDownload)
			r.Get("/storage/{storage}/download", s.handleVolumeDownload)
			r.Get("/ws", s.handleWS)
			r.Get("/nodes/{node}/console", s.handleNodeConsole)
			r.Get("/guests/{vmid}/console", s.handleGuestConsole)

			r.Group(func(r chi.Router) {
				r.Use(middleware.Timeout(60 * time.Second))
				s.timedRoutes(r)
			})
		})
	})
	return r
}

type ctxKey int

const (
	userKey    ctxKey = 1
	sessionKey ctxKey = 2
)

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, sess, err := s.sessionUserFromRequest(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), userKey, user)
		ctx = context.WithValue(ctx, sessionKey, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) userFromRequest(r *http.Request) (store.User, error) {
	user, _, err := s.sessionUserFromRequest(r)
	return user, err
}

func (s *Server) sessionUserFromRequest(r *http.Request) (store.User, store.Session, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return store.User{}, store.Session{}, store.ErrNotFound
	}
	sess, err := s.store.SessionByTokenHash(r.Context(), store.HashToken(c.Value))
	if err != nil {
		return store.User{}, store.Session{}, err
	}
	user, err := s.store.UserByID(r.Context(), sess.UserID)
	if err != nil {
		return store.User{}, store.Session{}, err
	}
	return user, sess, nil
}

func (s *Server) setSession(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.CookieSecure,
		Expires:  expires,
	})
}

func (s *Server) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// timedRoutes are the authenticated API routes that run under the 60s
// timeout. Router mounts them behind requireAuth.
func (s *Server) timedRoutes(r chi.Router) {
	r.Get("/connections", s.handleListVisibleConnections)
	r.Get("/inventory", s.handleInventory)
	r.Route("/admin/connections", func(r chi.Router) {
		r.Use(s.requireLocalAdmin)
		r.Get("/", s.handleAdminListConnections)
		r.Post("/", s.handleCreateConnection)
		r.Post("/test", s.handleTestConnection)
		r.Put("/{id}", s.handleUpdateConnection)
		r.Delete("/{id}", s.handleDeleteConnection)
		r.Post("/{id}/test", s.handleTestSavedConnection)
	})
	r.Get("/cluster/summary", s.handleSummary)
	r.Get("/insights", s.handleInsights)
	r.Get("/cluster/resources", s.handleResources)
	r.Get("/cluster/nextid", s.handleNextID)
	r.Get("/metrics/ring", s.handleRing)
	r.Get("/metrics/guest-iops-latest", s.handleGuestIOPSLatest)
	r.Get("/metrics/history", s.handleMetricHistory)
	r.Get("/audit", s.handleAudit)
	r.Get("/mcp", s.handleMCP)
	r.Post("/mcp", s.handleMCP)
	r.Get("/scripts", s.handleScriptCatalog)
	r.Get("/scripts/jobs", s.handleScriptJobs)
	r.Get("/scripts/jobs/{job}", s.handleScriptJob)
	r.Post("/scripts/{id}/plan", s.handleScriptPlan)
	r.Post("/scripts/{id}/run", s.handleScriptRun)
	r.Get("/media", s.handleMedia)
	r.Get("/nodes", s.handleNodes)
	r.Get("/nodes/{node}", s.handleNode)
	r.Get("/nodes/{node}/rrd", s.handleNodeRRD)
	r.Get("/nodes/{node}/io-rrd", s.handleNodeIORRD)
	r.Get("/nodes/{node}/capabilities", s.handleCapabilities)
	r.Get("/nodes/{node}/hardware/pci/{id}/mdev", s.handleNodeMdev)
	r.Get("/nodes/{node}/hardware/{kind}", s.handleNodeHardware)
	r.Get("/nodes/{node}/report", s.handleNodeReport)
	r.Post("/nodes/{node}/wakeonlan", s.handleWakeOnLAN)
	r.Get("/nodes/{node}/config", s.handleNodeConfig)
	r.Put("/nodes/{node}/config", s.handleUpdateNodeConfig)
	r.Get("/nodes/{node}/aplinfo", s.handleAPLInfo)
	r.Post("/nodes/{node}/guests/{action}", s.handleNodeGuestAll)
	r.Get("/nodes/{node}/network", s.handleNodeNetwork)
	r.Post("/nodes/{node}/network", s.handleCreateNodeNetwork)
	r.Put("/nodes/{node}/network/apply", s.handleApplyNodeNetwork)
	r.Delete("/nodes/{node}/network/revert", s.handleRevertNodeNetwork)
	r.Put("/nodes/{node}/network/{iface}", s.handleUpdateNodeNetwork)
	r.Delete("/nodes/{node}/network/{iface}", s.handleDeleteNodeNetwork)
	r.Get("/nodes/{node}/disks", s.handleNodeDisks)
	r.Get("/nodes/{node}/disks/smart", s.handleNodeDiskSMART)
	r.Post("/nodes/{node}/disks/wipe", s.handleWipeNodeDisk)
	r.Post("/nodes/{node}/disks/initgpt", s.handleInitGPT)
	r.Post("/nodes/{node}/disks/zfs/import", s.handleZFSImport)
	r.Get("/nodes/{node}/disks/zfs/{name}", s.handleZFSStatus)
	r.Post("/nodes/{node}/disks/zfs/{name}/scrub", s.handleZFSScrub)
	r.Get("/nodes/{node}/disks/{kind}", s.handleNodeDiskBackends)
	r.Post("/nodes/{node}/disks/{kind}", s.handleCreateNodeDiskBackend)
	r.Delete("/nodes/{node}/disks/{kind}/{name}", s.handleDeleteNodeDiskBackend)
	r.Get("/nodes/{node}/apt", s.handleAPTUpdates)
	r.Post("/nodes/{node}/apt/update", s.handleAPTUpdate)
	r.Get("/nodes/{node}/apt/versions", s.handleAPTVersions)
	r.Get("/nodes/{node}/apt/changelog", s.handleAPTChangelog)
	r.Get("/nodes/{node}/apt/repositories", s.handleAPTRepositories)
	r.Post("/nodes/{node}/apt/repositories", s.handleUpdateAPTRepository)
	r.Post("/nodes/{node}/apt/upgrade-session", s.handleAPTUpgradeSession)
	r.Get("/nodes/{node}/services", s.handleNodeServices)
	r.Post("/nodes/{node}/services/{service}/{action}", s.handleNodeServiceAction)
	r.Get("/nodes/{node}/dns", s.handleNodeDNS)
	r.Put("/nodes/{node}/dns", s.handleUpdateNodeDNS)
	r.Get("/nodes/{node}/timezone", s.handleNodeTime)
	r.Put("/nodes/{node}/timezone", s.handleUpdateNodeTime)
	r.Get("/nodes/{node}/hosts", s.handleNodeHosts)
	r.Put("/nodes/{node}/hosts", s.handleUpdateNodeHosts)
	r.Get("/nodes/{node}/journal", s.handleNodeJournal)
	r.Get("/nodes/{node}/subscription", s.handleNodeSubscription)
	r.Put("/nodes/{node}/subscription", s.handleSetNodeSubscription)
	r.Get("/nodes/{node}/certificates", s.handleNodeCertificates)
	r.Post("/nodes/{node}/certificates", s.handleUploadNodeCertificate)
	r.Delete("/nodes/{node}/certificates", s.handleDeleteNodeCertificate)
	r.Post("/nodes/{node}/certificates/acme", s.handleOrderACMECertificate)
	r.Put("/nodes/{node}/certificates/acme", s.handleRenewACMECertificate)
	r.Post("/nodes/{node}/execute", s.handleNodeExecute)
	r.Post("/nodes/{node}/status/{command}", s.handleNodeCommand)
	r.Get("/datacenter/options", s.handleClusterOptions)
	r.Put("/datacenter/options", s.handleUpdateClusterOptions)
	r.Get("/datacenter/pools", s.handlePools)
	r.Post("/datacenter/pools", s.handleCreatePool)
	r.Get("/datacenter/pools/{poolid}", s.handlePool)
	r.Put("/datacenter/pools/{poolid}", s.handleUpdatePool)
	r.Delete("/datacenter/pools/{poolid}", s.handleDeletePool)
	r.Get("/datacenter/acme/accounts", s.handleACMEAccounts)
	r.Post("/datacenter/acme/accounts", s.handleCreateACMEAccount)
	r.Put("/datacenter/acme/accounts/{name}", s.handleUpdateACMEAccount)
	r.Delete("/datacenter/acme/accounts/{name}", s.handleDeleteACMEAccount)
	r.Get("/datacenter/acme/plugins", s.handleACMEPlugins)
	r.Post("/datacenter/acme/plugins", s.handleCreateACMEPlugin)
	r.Put("/datacenter/acme/plugins/{id}", s.handleUpdateACMEPlugin)
	r.Delete("/datacenter/acme/plugins/{id}", s.handleDeleteACMEPlugin)
	r.Get("/datacenter/notifications/endpoints/{kind}", s.handleNotificationEndpoints)
	r.Post("/datacenter/notifications/endpoints/{kind}", s.handleCreateNotificationEndpoint)
	r.Put("/datacenter/notifications/endpoints/{kind}/{name}", s.handleUpdateNotificationEndpoint)
	r.Delete("/datacenter/notifications/endpoints/{kind}/{name}", s.handleDeleteNotificationEndpoint)
	r.Get("/datacenter/notifications/matchers", s.handleNotificationMatchers)
	r.Post("/datacenter/notifications/matchers", s.handleCreateNotificationMatcher)
	r.Put("/datacenter/notifications/matchers/{name}", s.handleUpdateNotificationMatcher)
	r.Delete("/datacenter/notifications/matchers/{name}", s.handleDeleteNotificationMatcher)
	r.Get("/datacenter/notifications/targets", s.handleNotificationTargets)
	r.Get("/datacenter/cluster", s.handleClusterConfig)
	r.Post("/datacenter/cluster", s.handleCreateCluster)
	r.Put("/datacenter/cluster/totem", s.handleUpdateClusterTotem)
	r.Get("/datacenter/remotes", s.handleClusterRemotes)
	r.Post("/datacenter/remotes", s.handleCreateClusterRemote)
	r.Put("/datacenter/remotes/{id}", s.handleUpdateClusterRemote)
	r.Delete("/datacenter/remotes/{id}", s.handleDeleteClusterRemote)
	r.Get("/datacenter/cluster/join", s.handleClusterJoinInfo)
	r.Post("/datacenter/cluster/join", s.handleJoinCluster)
	r.Get("/datacenter/cluster/nodes", s.handleClusterConfigNodes)
	r.Post("/datacenter/cluster/nodes", s.handleAddClusterNode)
	r.Delete("/datacenter/cluster/nodes/{node}", s.handleDeleteClusterNode)
	r.Get("/datacenter/cluster/qdevice", s.handleClusterQDevice)
	r.Post("/datacenter/cluster/qdevice", s.handleCreateClusterQDevice)
	r.Delete("/datacenter/cluster/qdevice", s.handleDeleteClusterQDevice)
	r.Get("/datacenter/mapping/{kind}", s.handleMappings)
	r.Post("/datacenter/mapping/{kind}", s.handleCreateMapping)
	r.Put("/datacenter/mapping/{kind}/{id}", s.handleUpdateMapping)
	r.Delete("/datacenter/mapping/{kind}/{id}", s.handleDeleteMapping)
	r.Get("/datacenter/metrics/servers", s.handleMetricServers)
	r.Post("/datacenter/metrics/servers", s.handleCreateMetricServer)
	r.Put("/datacenter/metrics/servers/{id}", s.handleUpdateMetricServer)
	r.Delete("/datacenter/metrics/servers/{id}", s.handleDeleteMetricServer)
	r.Get("/datacenter/log", s.handleClusterLog)
	r.Get("/datacenter/jobs", s.handleClusterJobs)
	r.Get("/datacenter/jobs/schedule-analyze", s.handleScheduleAnalyze)
	r.Get("/datacenter/jobs/realm-sync", s.handleRealmSyncJobs)
	r.Post("/datacenter/jobs/realm-sync", s.handleCreateRealmSyncJob)
	r.Put("/datacenter/jobs/realm-sync/{id}", s.handleUpdateRealmSyncJob)
	r.Delete("/datacenter/jobs/realm-sync/{id}", s.handleDeleteRealmSyncJob)
	r.Get("/datacenter/acme/directories", s.handleACMEDirectories)
	r.Get("/datacenter/acme/meta", s.handleACMEMeta)
	r.Get("/datacenter/acme/tos", s.handleACMETOS)
	r.Get("/datacenter/acme/challenge-schema", s.handleACMEChallengeSchema)
	r.Post("/datacenter/notifications/targets/{name}/test", s.handleTestNotificationTarget)
	r.Get("/datacenter/cpu-models", s.handleCustomCPUModels)
	r.Post("/datacenter/cpu-models", s.handleCreateCustomCPUModel)
	r.Put("/datacenter/cpu-models/{name}", s.handleUpdateCustomCPUModel)
	r.Delete("/datacenter/cpu-models/{name}", s.handleDeleteCustomCPUModel)
	r.Get("/nodes/{node}/vzdump/defaults", s.handleVZDumpDefaults)
	r.Put("/nodes/{node}/vzdump/defaults", s.handleUpdateVZDumpDefaults)
	r.Get("/sdn", s.handleSDNStatus)
	r.Put("/sdn/apply", s.handleApplySDN)
	r.Delete("/sdn/rollback", s.handleRollbackSDN)
	r.Get("/sdn/dry-run", s.handleSDNDryRun)
	r.Post("/sdn/lock", s.handleLockSDN)
	r.Delete("/sdn/lock", s.handleUnlockSDN)
	r.Get("/sdn/zones", s.handleSDNZones)
	r.Post("/sdn/zones", s.handleCreateSDNZone)
	r.Put("/sdn/zones/{zone}", s.handleUpdateSDNZone)
	r.Delete("/sdn/zones/{zone}", s.handleDeleteSDNZone)
	r.Get("/sdn/vnets", s.handleSDNVNets)
	r.Post("/sdn/vnets", s.handleCreateSDNVNet)
	r.Put("/sdn/vnets/{vnet}", s.handleUpdateSDNVNet)
	r.Delete("/sdn/vnets/{vnet}", s.handleDeleteSDNVNet)
	r.Get("/sdn/vnets/{vnet}/subnets", s.handleSDNSubnets)
	r.Post("/sdn/vnets/{vnet}/subnets", s.handleCreateSDNSubnet)
	r.Put("/sdn/vnets/{vnet}/subnets", s.handleUpdateSDNSubnet)
	r.Delete("/sdn/vnets/{vnet}/subnets", s.handleDeleteSDNSubnet)
	r.Get("/sdn/vnets/{vnet}/ips", s.handleSDNVNetIPs)
	r.Post("/sdn/vnets/{vnet}/ips", s.handleCreateSDNVNetIP)
	r.Delete("/sdn/vnets/{vnet}/ips", s.handleDeleteSDNVNetIP)
	r.Get("/sdn/vnets/{vnet}/firewall/rules", s.handleVNetFWRules)
	r.Post("/sdn/vnets/{vnet}/firewall/rules", s.handleCreateVNetFWRule)
	r.Put("/sdn/vnets/{vnet}/firewall/rules/{pos}", s.handleUpdateVNetFWRule)
	r.Delete("/sdn/vnets/{vnet}/firewall/rules/{pos}", s.handleDeleteVNetFWRule)
	r.Get("/sdn/vnets/{vnet}/firewall/options", s.handleVNetFWOptions)
	r.Put("/sdn/vnets/{vnet}/firewall/options", s.handleUpdateVNetFWOptions)
	r.Get("/sdn/vnets/{vnet}/firewall/log", s.handleVNetFWLog)
	r.Get("/sdn/controllers", s.handleSDNControllers)
	r.Post("/sdn/controllers", s.handleCreateSDNController)
	r.Put("/sdn/controllers/{id}", s.handleUpdateSDNController)
	r.Delete("/sdn/controllers/{id}", s.handleDeleteSDNController)
	r.Get("/sdn/ipams", s.handleSDNIPAMs)
	r.Post("/sdn/ipams", s.handleCreateSDNIPAM)
	r.Get("/sdn/ipams/{id}/status", s.handleSDNIPAMStatus)
	r.Put("/sdn/ipams/{id}", s.handleUpdateSDNIPAM)
	r.Delete("/sdn/ipams/{id}", s.handleDeleteSDNIPAM)
	r.Get("/sdn/dns", s.handleSDNDNS)
	r.Post("/sdn/dns", s.handleCreateSDNDNS)
	r.Put("/sdn/dns/{id}", s.handleUpdateSDNDNS)
	r.Delete("/sdn/dns/{id}", s.handleDeleteSDNDNS)
	r.Get("/sdn/fabrics", s.handleSDNFabrics)
	r.Post("/sdn/fabrics", s.handleCreateSDNFabric)
	r.Put("/sdn/fabrics/{id}", s.handleUpdateSDNFabric)
	r.Delete("/sdn/fabrics/{id}", s.handleDeleteSDNFabric)
	r.Get("/sdn/fabrics/{id}/nodes", s.handleSDNFabricNodes)
	r.Post("/sdn/fabrics/{id}/nodes", s.handleCreateSDNFabricNode)
	r.Put("/sdn/fabrics/{id}/nodes/{node}", s.handleUpdateSDNFabricNode)
	r.Delete("/sdn/fabrics/{id}/nodes/{node}", s.handleDeleteSDNFabricNode)
	r.Get("/sdn/fabrics/{id}/routemaps", s.handleSDNFabricRouteMaps)
	r.Post("/sdn/fabrics/{id}/routemaps", s.handleCreateSDNFabricRouteMap)
	r.Delete("/sdn/fabrics/{id}/routemaps/{name}", s.handleDeleteSDNFabricRouteMap)
	r.Get("/sdn/fabrics/{id}/prefixlists", s.handleSDNFabricPrefixLists)
	r.Post("/sdn/fabrics/{id}/prefixlists", s.handleCreateSDNFabricPrefixList)
	r.Delete("/sdn/fabrics/{id}/prefixlists/{name}", s.handleDeleteSDNFabricPrefixList)
	r.Get("/sdn/nodes/{node}/fabrics/{id}/interfaces", s.handleSDNFabricInterfaces)
	r.Get("/sdn/route-maps", s.handleSDNRouteMaps)
	r.Post("/sdn/route-maps", s.handleCreateSDNRouteMap)
	r.Delete("/sdn/route-maps/{id}", s.handleDeleteSDNRouteMap)
	r.Get("/sdn/route-maps/{id}/entries", s.handleSDNRouteMapEntries)
	r.Post("/sdn/route-maps/{id}/entries", s.handleCreateSDNRouteMapEntry)
	r.Put("/sdn/route-maps/{id}/entries/{order}", s.handleUpdateSDNRouteMapEntry)
	r.Delete("/sdn/route-maps/{id}/entries/{order}", s.handleDeleteSDNRouteMapEntry)
	r.Get("/sdn/prefix-lists", s.handleSDNPrefixLists)
	r.Post("/sdn/prefix-lists", s.handleCreateSDNPrefixList)
	r.Put("/sdn/prefix-lists/{id}", s.handleUpdateSDNPrefixList)
	r.Delete("/sdn/prefix-lists/{id}", s.handleDeleteSDNPrefixList)
	r.Get("/sdn/prefix-lists/{id}/entries", s.handleSDNPrefixListEntries)
	r.Post("/sdn/prefix-lists/{id}/entries", s.handleCreateSDNPrefixListEntry)
	r.Put("/sdn/prefix-lists/{id}/entries/{seq}", s.handleUpdateSDNPrefixListEntry)
	r.Delete("/sdn/prefix-lists/{id}/entries/{seq}", s.handleDeleteSDNPrefixListEntry)
	r.Get("/sdn/nodes/{node}", s.handleNodeSDN)
	r.Get("/ceph", s.handleCephStatus)
	r.Get("/ceph/flags", s.handleCephFlags)
	r.Put("/ceph/flags", s.handleUpdateCephFlags)
	r.Get("/ceph/config", s.handleCephConfig)
	r.Get("/ceph/crush", s.handleCephCrush)
	r.Put("/ceph/crush", s.handleUpdateCephCrush)
	r.Get("/ceph/rules", s.handleCephRules)
	r.Post("/ceph/rules", s.handleCreateCephRule)
	r.Get("/ceph/log", s.handleCephLog)
	r.Get("/ceph/metadata", s.handleCephMetadata)
	r.Get("/ceph/cmd-safety", s.handleCephCmdSafety)
	r.Get("/ceph/releases", s.handleCephReleases)
	r.Post("/ceph/restart-bulk", s.handleCephRestartBulk)
	r.Post("/ceph/restart", s.handleRestartCeph)
	r.Post("/ceph/start", s.handleStartCeph)
	r.Post("/ceph/stop", s.handleStopCeph)
	r.Get("/ceph/rgw", s.handleCephRGW)
	r.Post("/ceph/rgw", s.handleCreateCephRGW)
	r.Get("/ceph/rgw/users", s.handleCephRGWUsers)
	r.Post("/ceph/rgw/users", s.handleCreateCephRGWUser)
	r.Delete("/ceph/rgw/users/{uid}", s.handleDeleteCephRGWUser)
	r.Delete("/ceph/rgw/{id}", s.handleDeleteCephRGW)
	r.Get("/ceph/mons", s.handleCephMons)
	r.Post("/ceph/mons", s.handleCreateCephMon)
	r.Delete("/ceph/mons/{id}", s.handleDeleteCephMon)
	r.Get("/ceph/mgrs", s.handleCephMgrs)
	r.Post("/ceph/mgrs", s.handleCreateCephMgr)
	r.Delete("/ceph/mgrs/{id}", s.handleDeleteCephMgr)
	r.Get("/ceph/mds", s.handleCephMDS)
	r.Post("/ceph/mds", s.handleCreateCephMDS)
	r.Delete("/ceph/mds/{id}", s.handleDeleteCephMDS)
	r.Get("/ceph/osds", s.handleCephOSDs)
	r.Post("/ceph/osds/{osdid}/in", s.handleCephOSDIn)
	r.Post("/ceph/osds/{osdid}/out", s.handleCephOSDOut)
	r.Post("/ceph/osds/{osdid}/scrub", s.handleCephOSDScrub)
	r.Post("/ceph/osds/{osdid}/destroy", s.handleDestroyCephOSD)
	r.Get("/ceph/osds/{osdid}/metadata", s.handleCephOSDMetadata)
	r.Get("/ceph/osds/{osdid}/lv-info", s.handleCephOSDLVInfo)
	r.Get("/ceph/pools", s.handleCephPools)
	r.Post("/ceph/pools", s.handleCreateCephPool)
	r.Get("/ceph/pools/{pool}/status", s.handleCephPoolStatus)
	r.Put("/ceph/pools/{pool}", s.handleUpdateCephPool)
	r.Delete("/ceph/pools/{pool}", s.handleDeleteCephPool)
	r.Get("/ceph/fs", s.handleCephFS)
	r.Post("/ceph/fs", s.handleCreateCephFS)
	r.Get("/nodes/{node}/ceph", s.handleCephStatus)
	r.Post("/nodes/{node}/ceph/restart-bulk", s.handleNodeCephRestartBulk)
	r.Post("/nodes/{node}/ceph/install", s.handleInstallCeph)
	r.Post("/nodes/{node}/ceph/init", s.handleInitCeph)
	r.Post("/nodes/{node}/ceph/osd", s.handleCreateNodeCephOSD)
	r.Get("/nodes/{node}/ceph/log", s.handleCephLog)

	r.Post("/guests", s.handleCreateGuest)
	r.Post("/guests/bulk/{action}", s.handleBulk)
	r.Get("/guests/{vmid}", s.handleGuest)
	r.Put("/guests/{vmid}", s.handleUpdateGuest)
	r.Delete("/guests/{vmid}", s.handleDeleteGuest)
	r.Post("/guests/{vmid}/status/{action}", s.handleGuestPower)
	r.Get("/guests/{vmid}/rrd", s.handleGuestRRD)
	r.Get("/guests/{vmid}/io-rrd", s.handleGuestIORRD)
	r.Get("/guests/{vmid}/snapshots", s.handleSnapshots)
	r.Post("/guests/{vmid}/snapshots", s.handleCreateSnapshot)
	r.Put("/guests/{vmid}/snapshots/{name}", s.handleUpdateSnapshot)
	r.Delete("/guests/{vmid}/snapshots/{name}", s.handleDeleteSnapshot)
	r.Post("/guests/{vmid}/snapshots/{name}/rollback", s.handleRollbackSnapshot)
	r.Delete("/guests/{vmid}/pending", s.handleRevertPending)
	r.Get("/guests/{vmid}/feature", s.handleGuestFeature)
	r.Post("/guests/{vmid}/clone", s.handleClone)
	r.Post("/guests/{vmid}/template", s.handleTemplate)
	r.Post("/guests/{vmid}/migrate", s.handleMigrate)
	r.Post("/guests/{vmid}/remote-migrate", s.handleRemoteMigrate)
	r.Post("/guests/{vmid}/disks/resize", s.handleResizeDisk)
	r.Post("/guests/{vmid}/disks/move", s.handleMoveDisk)
	r.Post("/guests/{vmid}/disks/unlink", s.handleUnlinkDisk)
	r.Post("/guests/{vmid}/disks/rescan", s.handleRescanDisks)
	r.Get("/guests/{vmid}/cloudinit", s.handleGetCloudInit)
	r.Put("/guests/{vmid}/cloudinit", s.handlePutCloudInit)
	r.Get("/guests/{vmid}/cloudinit/dump", s.handleCloudInitDump)
	r.Post("/guests/{vmid}/cloudinit/regenerate", s.handleRegenerateCloudInit)
	r.Get("/guests/{vmid}/agent", s.handleAgent)
	r.Post("/guests/{vmid}/agent/fstrim", s.handleAgentFstrim)
	r.Post("/guests/{vmid}/agent/password", s.handleAgentPassword)
	r.Get("/guests/{vmid}/agent/{command}", s.handleAgentCommand)
	r.Post("/guests/{vmid}/agent/{command}", s.handleAgentCommandPost)
	r.Post("/guests/{vmid}/sendkey", s.handleSendKey)
	r.Post("/guests/{vmid}/monitor", s.handleQEMUMonitor)
	r.Post("/guests/{vmid}/backup", s.handleGuestBackup)

	r.Get("/backups/jobs", s.handleBackupJobs)
	r.Post("/backups/jobs", s.handleCreateBackupJob)
	r.Get("/backups/jobs/{id}", s.handleBackupJob)
	r.Put("/backups/jobs/{id}", s.handleUpdateBackupJob)
	r.Delete("/backups/jobs/{id}", s.handleDeleteBackupJob)
	r.Get("/backups/jobs/{id}/volumes", s.handleJobVolumes)
	r.Get("/backups/coverage", s.handleCoverage)
	r.Post("/backups/restore", s.handleRestore)
	r.Get("/backups/extractconfig", s.handleExtractConfig)

	r.Get("/replication", s.handleReplicationJobs)
	r.Post("/replication", s.handleCreateReplication)
	r.Get("/replication/{id}", s.handleReplicationJob)
	r.Put("/replication/{id}", s.handleUpdateReplication)
	r.Delete("/replication/{id}", s.handleDeleteReplication)
	r.Get("/replication/{id}/status", s.handleReplicationStatus)
	r.Get("/replication/{id}/log", s.handleReplicationLog)
	r.Post("/replication/{id}/schedule-now", s.handleReplicationNow)

	r.Get("/storage", s.handleStorages)
	r.Post("/storage", s.handleCreateStorage)
	r.Get("/storage/{storage}", s.handleStorage)
	r.Put("/storage/{storage}", s.handleUpdateStorage)
	r.Delete("/storage/{storage}", s.handleDeleteStorage)
	r.Get("/storage/scan/{kind}", s.handleScanStorage)
	r.Get("/storage/{storage}/content", s.handleStorageContent)
	r.Put("/storage/{storage}/content/notes", s.handleUpdateVolumeNotes)
	r.Delete("/storage/{storage}/content", s.handleDeleteVolume)
	r.Post("/storage/{storage}/oci-pull", s.handleOCIPull)
	r.Get("/storage/{storage}/identity", s.handleStorageIdentity)
	r.Put("/storage/{storage}/identity", s.handleUpdateStorageIdentity)
	r.Get("/storage/{storage}/rrd", s.handleStorageRRD)
	r.Post("/storage/{storage}/download-url", s.handleDownloadURL)
	r.Get("/storage/{storage}/import-metadata", s.handleImportMetadata)
	r.Get("/storage/{storage}/query-url-metadata", s.handleQueryURLMetadata)
	r.Get("/storage/{storage}/file-restore", s.handleFileRestoreList)
	r.Get("/storage/{storage}/prune", s.handlePrunePreview)
	r.Delete("/storage/{storage}/prune", s.handlePrune)

	r.Get("/ha/status", s.handleHAStatus)
	r.Post("/ha/arm", s.handleArmHA)
	r.Post("/ha/disarm", s.handleDisarmHA)
	r.Get("/ha/resources", s.handleHAResources)
	r.Post("/ha/resources", s.handleCreateHAResource)
	r.Post("/ha/resources/{sid}/migrate", s.handleHAMigrate)
	r.Post("/ha/resources/{sid}/relocate", s.handleHARelocate)
	r.Get("/ha/resources/{sid}", s.handleHAResource)
	r.Put("/ha/resources/{sid}", s.handleUpdateHAResource)
	r.Delete("/ha/resources/{sid}", s.handleDeleteHAResource)
	r.Get("/ha/groups", s.handleHAGroups)
	r.Post("/ha/groups", s.handleCreateHAGroup)
	r.Get("/ha/groups/{group}", s.handleHAGroup)
	r.Put("/ha/groups/{group}", s.handleUpdateHAGroup)
	r.Delete("/ha/groups/{group}", s.handleDeleteHAGroup)
	r.Get("/firewall/dc/rules", s.handleDCFWRules)
	r.Post("/firewall/dc/rules", s.handleCreateDCFWRule)
	r.Put("/firewall/dc/rules/{pos}", s.handleUpdateDCFWRule)
	r.Delete("/firewall/dc/rules/{pos}", s.handleDeleteDCFWRule)
	r.Get("/firewall/dc/options", s.handleDCFWOptions)
	r.Put("/firewall/dc/options", s.handleUpdateDCFWOptions)
	r.Get("/firewall/dc/log", s.handleDCFWLog)
	r.Get("/firewall/dc/aliases", s.handleDCFWAliases)
	r.Post("/firewall/dc/aliases", s.handleCreateDCFWAlias)
	r.Put("/firewall/dc/aliases/{name}", s.handleUpdateDCFWAlias)
	r.Delete("/firewall/dc/aliases/{name}", s.handleDeleteDCFWAlias)
	r.Get("/firewall/dc/ipset", s.handleDCFWIPSets)
	r.Post("/firewall/dc/ipset", s.handleCreateDCFWIPSet)
	r.Delete("/firewall/dc/ipset/{name}", s.handleDeleteDCFWIPSet)
	r.Get("/firewall/dc/ipset/{name}", s.handleDCFWIPSetCIDRs)
	r.Post("/firewall/dc/ipset/{name}", s.handleCreateDCFWIPSetCIDR)
	r.Delete("/firewall/dc/ipset/{name}/cidr", s.handleDeleteDCFWIPSetCIDR)
	r.Get("/firewall/dc/macros", s.handleDCFWMacros)
	r.Get("/firewall/dc/refs", s.handleDCFWRefs)
	r.Get("/firewall/dc/groups", s.handleFWGroups)
	r.Post("/firewall/dc/groups", s.handleCreateFWGroup)
	r.Delete("/firewall/dc/groups/{group}", s.handleDeleteFWGroup)
	r.Get("/firewall/dc/groups/{group}", s.handleFWGroupRules)
	r.Post("/firewall/dc/groups/{group}", s.handleCreateFWGroupRule)
	r.Put("/firewall/dc/groups/{group}/{pos}", s.handleUpdateFWGroupRule)
	r.Delete("/firewall/dc/groups/{group}/{pos}", s.handleDeleteFWGroupRule)
	r.Get("/firewall/nodes/{node}/rules", s.handleNodeFWRules)
	r.Post("/firewall/nodes/{node}/rules", s.handleCreateNodeFWRule)
	r.Put("/firewall/nodes/{node}/rules/{pos}", s.handleUpdateNodeFWRule)
	r.Delete("/firewall/nodes/{node}/rules/{pos}", s.handleDeleteNodeFWRule)
	r.Get("/firewall/nodes/{node}/options", s.handleNodeFWOptions)
	r.Put("/firewall/nodes/{node}/options", s.handleUpdateNodeFWOptions)
	r.Get("/firewall/nodes/{node}/log", s.handleNodeFWLog)
	r.Get("/firewall/guests/{vmid}/rules", s.handleGuestFWRules)
	r.Post("/firewall/guests/{vmid}/rules", s.handleCreateGuestFWRule)
	r.Put("/firewall/guests/{vmid}/rules/{pos}", s.handleUpdateGuestFWRule)
	r.Delete("/firewall/guests/{vmid}/rules/{pos}", s.handleDeleteGuestFWRule)
	r.Get("/firewall/guests/{vmid}/options", s.handleGuestFWOptions)
	r.Put("/firewall/guests/{vmid}/options", s.handleUpdateGuestFWOptions)
	r.Get("/firewall/guests/{vmid}/log", s.handleGuestFWLog)
	r.Get("/firewall/guests/{vmid}/aliases", s.handleGuestFWAliases)
	r.Post("/firewall/guests/{vmid}/aliases", s.handleCreateGuestFWAlias)
	r.Put("/firewall/guests/{vmid}/aliases/{name}", s.handleUpdateGuestFWAlias)
	r.Delete("/firewall/guests/{vmid}/aliases/{name}", s.handleDeleteGuestFWAlias)
	r.Get("/firewall/guests/{vmid}/ipset", s.handleGuestFWIPSets)
	r.Post("/firewall/guests/{vmid}/ipset", s.handleCreateGuestFWIPSet)
	r.Delete("/firewall/guests/{vmid}/ipset/{name}", s.handleDeleteGuestFWIPSet)
	r.Get("/firewall/guests/{vmid}/ipset/{name}", s.handleGuestFWIPSetCIDRs)
	r.Post("/firewall/guests/{vmid}/ipset/{name}", s.handleCreateGuestFWIPSetCIDR)
	r.Delete("/firewall/guests/{vmid}/ipset/{name}/cidr", s.handleDeleteGuestFWIPSetCIDR)
	r.Get("/firewall/guests/{vmid}/refs", s.handleGuestFWRefs)

	r.Get("/ha/rules", s.handleHARules)
	r.Post("/ha/rules", s.handleCreateHARule)
	r.Put("/ha/rules/{rule}", s.handleUpdateHARule)
	r.Delete("/ha/rules/{rule}", s.handleDeleteHARule)

	r.Get("/access/users", s.handleAccessUsers)
	r.Post("/access/users", s.handleCreateAccessUser)
	r.Get("/access/users/{userid}", s.handleAccessUser)
	r.Put("/access/users/{userid}", s.handleUpdateAccessUser)
	r.Delete("/access/users/{userid}", s.handleDeleteAccessUser)
	r.Put("/access/users/{userid}/password", s.handleAccessPassword)
	r.Get("/access/users/{userid}/tokens", s.handleAccessTokens)
	r.Post("/access/users/{userid}/tokens", s.handleCreateAccessToken)
	r.Put("/access/users/{userid}/tokens/{tokenid}", s.handleUpdateAccessToken)
	r.Delete("/access/users/{userid}/tokens/{tokenid}", s.handleDeleteAccessToken)
	r.Get("/access/users/{userid}/tfa", s.handleAccessTFA)
	r.Post("/access/users/{userid}/tfa", s.handleEnrollAccessTFA)
	r.Delete("/access/users/{userid}/tfa/{id}", s.handleDeleteAccessTFA)
	r.Put("/access/users/{userid}/unlock-tfa", s.handleUnlockAccessTFA)
	r.Get("/access/openid", s.handleOpenID)
	r.Get("/access/groups", s.handleAccessGroups)
	r.Post("/access/groups", s.handleCreateAccessGroup)
	r.Get("/access/groups/{group}", s.handleAccessGroup)
	r.Put("/access/groups/{group}", s.handleUpdateAccessGroup)
	r.Delete("/access/groups/{group}", s.handleDeleteAccessGroup)
	r.Get("/access/roles", s.handleAccessRoles)
	r.Post("/access/roles", s.handleCreateAccessRole)
	r.Put("/access/roles/{role}", s.handleUpdateAccessRole)
	r.Delete("/access/roles/{role}", s.handleDeleteAccessRole)
	r.Get("/access/acl", s.handleAccessACL)
	r.Put("/access/acl", s.handleUpdateAccessACL)
	r.Get("/access/realms", s.handleAccessRealms)
	r.Post("/access/realms", s.handleCreateAccessRealm)
	r.Get("/access/realms/{realm}", s.handleAccessRealm)
	r.Put("/access/realms/{realm}", s.handleUpdateAccessRealm)
	r.Delete("/access/realms/{realm}", s.handleDeleteAccessRealm)
	r.Post("/access/realms/{realm}/sync", s.handleSyncAccessRealm)

	r.Get("/tasks", s.handleTasks)
	r.Get("/tasks/{upid}", s.handleTask)
	r.Delete("/tasks/{upid}", s.handleStopTask)
}

func (s *Server) requireLocalAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := r.Context().Value(userKey).(store.User)
		if user.Kind != store.UserKindLocal {
			writeError(w, http.StatusForbidden, "local admin required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// urlParam returns a chi path parameter decoded exactly once. chi matches on the
// escaped path whenever the request has one, so IDs the UI encodes (UPIDs,
// "vm:100", "root@pam", CIDRs) would otherwise reach the Proxmox client still
// percent-encoded and be escaped a second time.
func urlParam(r *http.Request, key string) string {
	v := chi.URLParam(r, key)
	if r.URL.RawPath == "" || !strings.Contains(v, "%") {
		return v
	}
	if dec, err := url.PathUnescape(v); err == nil {
		return dec
	}
	return v
}

func (s *Server) requestConnectionID(r *http.Request) string {
	user, _ := r.Context().Value(userKey).(store.User)
	sess, _ := r.Context().Value(sessionKey).(store.Session)
	if user.Kind == store.UserKindPVE && sess.ConnectionID != "" {
		return sess.ConnectionID
	}
	if id := strings.TrimSpace(r.Header.Get("X-Hypervane-Connection")); id != "" {
		return id
	}
	if id := strings.TrimSpace(r.URL.Query().Get("connection")); id != "" {
		return id
	}
	return s.DefaultConnectionID()
}

func (s *Server) pveOrError(w http.ResponseWriter, r *http.Request) *pve.Client {
	id := s.requestConnectionID(r)
	base := s.ClientByID(id)
	if base == nil {
		base = s.Client()
	}
	if base == nil {
		writeError(w, http.StatusServiceUnavailable, "cluster is not configured")
		return nil
	}
	user, _ := r.Context().Value(userKey).(store.User)
	if user.Kind != store.UserKindPVE {
		return base
	}
	sess, _ := r.Context().Value(sessionKey).(store.Session)
	c, err := s.ticketClient(r.Context(), sess, user)
	if err != nil {
		s.clearSession(w)
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return nil
	}
	return c
}

func (s *Server) ticketClient(ctx context.Context, sess store.Session, user store.User) (*pve.Client, error) {
	base := s.ClientByID(sess.ConnectionID)
	if base == nil {
		base = s.Client()
	}
	if base == nil {
		return nil, store.ErrNotFound
	}
	if !sess.HasTicket() {
		return nil, store.ErrNotFound
	}
	ticket, err := s.box.Open(sess.TicketCiphertext, sess.TicketNonce)
	if err != nil {
		return nil, err
	}
	var csrf []byte
	if len(sess.CSRFCiphertext) > 0 {
		csrf, err = s.box.Open(sess.CSRFCiphertext, sess.CSRFNonce)
		if err != nil {
			return nil, err
		}
	}
	needRefresh := sess.TicketExpiresAt != nil && time.Until(*sess.TicketExpiresAt) < 5*time.Minute
	if needRefresh {
		refreshed, err := base.RefreshTicket(ctx, user.PVEUserID, string(ticket))
		if err != nil || refreshed.Ticket == "" || refreshed.NeedTFA {
			if err == nil {
				err = store.ErrNotFound
			}
			return nil, err
		}
		if err := s.sealTicket(&sess, refreshed.Ticket, refreshed.CSRF); err != nil {
			return nil, err
		}
		if err := s.store.UpdateSessionTicket(ctx, sess.ID, sess); err != nil {
			return nil, err
		}
		ticket = []byte(refreshed.Ticket)
		csrf = []byte(refreshed.CSRF)
	}
	return base.WithTicket(string(ticket), string(csrf)), nil
}

func (s *Server) requestACL(r *http.Request) (pve.ACL, bool) {
	user, _ := r.Context().Value(userKey).(store.User)
	if user.Kind != store.UserKindPVE {
		return nil, false
	}
	sess, _ := r.Context().Value(sessionKey).(store.Session)
	c, err := s.ticketClient(r.Context(), sess, user)
	if err != nil {
		return pve.ACL{}, true
	}
	raw, err := c.Permissions(r.Context())
	if err != nil {
		return pve.ACL{}, true
	}
	return pve.NormalizePermissions(raw), true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writePVEError(w http.ResponseWriter, err error) {
	if apiErr, ok := pve.IsAPIError(err); ok {
		writeError(w, apiErr.Status, apiErr.Message)
		return
	}
	log.Printf("pve: %v", err)
	writeError(w, http.StatusBadGateway, err.Error())
}

func decodeJSON(r *http.Request, dest any) error {
	return json.NewDecoder(r.Body).Decode(dest)
}

func queryFlag(r *http.Request, key string) bool {
	v := strings.ToLower(strings.TrimSpace(r.URL.Query().Get(key)))
	return v == "1" || v == "true" || v == "yes"
}

func upgrader() websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
}

func queryDefault(r *http.Request, key, fallback string) string {
	if v := r.URL.Query().Get(key); v != "" {
		return v
	}
	return fallback
}

func kindFromType(t string) string {
	switch t {
	case "qemu", "lxc":
		return t
	default:
		return "qemu"
	}
}
