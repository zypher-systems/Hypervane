package poller

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"hypervane/internal/hub"
	"hypervane/internal/pve"
	"hypervane/internal/store"
)

const (
	ringTTL = 30 * time.Minute
	ringMax = 180
)

type ConnectionClient struct {
	ID     string
	Client *pve.Client
}

type ClientProvider func() []ConnectionClient

type Poller struct {
	rdb      *redis.Client
	hub      *hub.Hub
	provider ClientProvider
	store    *store.Store

	mu         sync.Mutex
	watchers   map[string]int
	iopsMu     sync.Mutex
	iopsPrev   map[string]iopsSnap
	iopsPrune  time.Time
	pendingMu  sync.Mutex
	pendingVMs map[string]bool
}

func New(rdb *redis.Client, h *hub.Hub, provider ClientProvider, st *store.Store) *Poller {
	return &Poller{
		rdb:        rdb,
		hub:        h,
		provider:   provider,
		store:      st,
		watchers:   make(map[string]int),
		iopsPrev:   make(map[string]iopsSnap),
		pendingVMs: make(map[string]bool),
	}
}

func (p *Poller) Watch(topic string) {
	p.mu.Lock()
	p.watchers[topic]++
	p.mu.Unlock()
}

func (p *Poller) Unwatch(topic string) {
	p.mu.Lock()
	if p.watchers[topic] > 0 {
		p.watchers[topic]--
	}
	if p.watchers[topic] == 0 {
		delete(p.watchers, topic)
	}
	p.mu.Unlock()
}

func (p *Poller) watching(prefix string) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []string
	for k, n := range p.watchers {
		if n > 0 && len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			out = append(out, k)
		}
	}
	return out
}

func (p *Poller) Run(ctx context.Context) {
	resTick := time.NewTicker(3 * time.Second)
	taskTick := time.NewTicker(2 * time.Second)
	rrdTick := time.NewTicker(30 * time.Second)
	detailTick := time.NewTicker(2 * time.Second)
	iopsTick := time.NewTicker(10 * time.Second)
	pendingTick := time.NewTicker(15 * time.Second)
	defer resTick.Stop()
	defer taskTick.Stop()
	defer rrdTick.Stop()
	defer detailTick.Stop()
	defer iopsTick.Stop()
	defer pendingTick.Stop()

	p.pollResources(ctx)
	p.pollTasks(ctx)
	p.pollIOPS(ctx)
	p.pollPending(ctx)
	p.pollResources(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-resTick.C:
			p.pollResources(ctx)
		case <-taskTick.C:
			p.pollTasks(ctx)
		case <-detailTick.C:
			p.pollWatchedDetails(ctx)
		case <-rrdTick.C:
			p.pollWatchedRRD(ctx)
		case <-iopsTick.C:
			p.pollIOPS(ctx)
		case <-pendingTick.C:
			p.pollPending(ctx) // guest pending-restart flags
		}
	}
}

func (p *Poller) clients() []ConnectionClient {
	if p.provider == nil {
		return nil
	}
	return p.provider()
}

func resourcesKey(id string) string { return "hypervane:" + id + ":resources" }
func statusKey(id string) string    { return "hypervane:" + id + ":cluster_status" }
func tasksKey(id string) string     { return "hypervane:" + id + ":tasks" }

func (p *Poller) pollResources(ctx context.Context) {
	now := time.Now().UnixMilli()
	for _, cc := range p.clients() {
		c := cc.Client
		if c == nil {
			continue
		}
		status, err := c.ClusterStatus(ctx)
		if err != nil {
			log.Printf("poll cluster status %s: %v", cc.ID, err)
			continue
		}
		resources, err := c.ClusterResources(ctx)
		if err != nil {
			log.Printf("poll resources %s: %v", cc.ID, err)
			continue
		}
		p.stampPending(cc.ID, resources)
		p.cacheJSON(ctx, statusKey(cc.ID), status)
		p.cacheJSON(ctx, resourcesKey(cc.ID), resources)
		p.hub.Publish(ctx, "resources", map[string]any{
			"connection": cc.ID,
			"status":     status,
			"resources":  resources,
			"ts":         now,
		})
		for _, r := range resources {
			sample := map[string]any{
				"ts": now, "cpu": r.CPU, "mem": r.Mem, "maxmem": r.MaxMem,
				"disk": r.Disk, "maxdisk": r.MaxDisk, "netin": r.NetIn, "netout": r.NetOut,
			}
			p.pushRing(ctx, ringKey(cc.ID, r), sample)
			if p.store != nil && (r.Type == "qemu" || r.Type == "lxc" || r.Type == "node") {
				if err := p.store.WriteMetricSample(ctx, cc.ID, r.ID, time.Now(), r.CPU, float64(r.Mem), float64(r.MaxMem), float64(r.Disk), float64(r.MaxDisk), float64(r.NetIn), float64(r.NetOut)); err != nil {
					log.Printf("metric history %s/%s: %v", cc.ID, r.ID, err)
				}
			}
		}
	}
}

func (p *Poller) pollTasks(ctx context.Context) {
	conns := p.clients()
	for _, cc := range conns {
		c := cc.Client
		if c == nil {
			continue
		}
		tasks, err := c.ClusterTasks(ctx)
		if err != nil {
			log.Printf("poll tasks %s: %v", cc.ID, err)
			continue
		}
		if len(tasks) > 80 {
			tasks = tasks[:80]
		}
		p.cacheJSON(ctx, tasksKey(cc.ID), tasks)
		p.hub.Publish(ctx, "tasks", map[string]any{"connection": cc.ID, "tasks": tasks})
	}

	for _, topic := range p.watching("task:") {
		upid := topic[len("task:"):]
		node := pve.ParseUPIDNode(upid)
		if node == "" {
			continue
		}
		for _, cc := range conns {
			if cc.Client == nil {
				continue
			}
			st, err := cc.Client.TaskStatus(ctx, node, upid)
			if err != nil {
				continue
			}
			lines, total, err := cc.Client.TaskLogTail(ctx, node, upid, 500)
			if err != nil {
				lines = nil
			}
			p.hub.Publish(ctx, topic, map[string]any{"status": st, "log": lines, "total": total})
			break
		}
	}
}

func (p *Poller) pollWatchedDetails(ctx context.Context) {
	conns := p.clients()
	if len(conns) == 0 {
		return
	}
	for _, topic := range p.watching("node:") {
		node := topic[len("node:"):]
		for _, cc := range conns {
			if cc.Client == nil {
				continue
			}
			st, err := cc.Client.NodeStatus(ctx, node)
			if err != nil {
				continue
			}
			p.hub.Publish(ctx, topic, st)
			break
		}
	}
	for _, topic := range p.watching("guest:") {
		parts := splitN(topic[len("guest:"):], 3)
		if len(parts) != 3 {
			continue
		}
		vmid, _ := strconv.Atoi(parts[2])
		for _, cc := range conns {
			if cc.Client == nil {
				continue
			}
			st, err := cc.Client.GuestStatus(ctx, parts[0], parts[1], vmid)
			if err != nil {
				continue
			}
			p.hub.Publish(ctx, topic, st)
			break
		}
	}
}

func (p *Poller) pollWatchedRRD(ctx context.Context) {
	conns := p.clients()
	if len(conns) == 0 {
		return
	}
	for _, topic := range p.watching("rrd:") {
		rest := topic[len("rrd:"):]
		parts := splitN(rest, 3)
		if len(parts) != 3 {
			continue
		}
		kind, id, tf := parts[0], parts[1], parts[2]
		for _, cc := range conns {
			if cc.Client == nil {
				continue
			}
			var (
				data []pve.RRDPoint
				err  error
			)
			switch kind {
			case "node":
				data, err = cc.Client.NodeRRD(ctx, id, tf)
			case "qemu", "lxc":
				np := splitN(id, 2)
				if len(np) != 2 {
					continue
				}
				vmid, _ := strconv.Atoi(np[1])
				data, err = cc.Client.GuestRRD(ctx, np[0], kind, vmid, tf)
			case "storage":
				np := splitN(id, 2)
				if len(np) != 2 {
					continue
				}
				data, err = cc.Client.StorageRRD(ctx, np[0], np[1], tf)
			}
			if err != nil {
				continue
			}
			p.hub.Publish(ctx, topic, data)
			break
		}
	}
}

func (p *Poller) CachedResources(ctx context.Context, connectionID string) (status []pve.ClusterStatus, resources []pve.Resource, err error) {
	if err = p.loadJSON(ctx, statusKey(connectionID), &status); err != nil && err != redis.Nil {
		return
	}
	if err = p.loadJSON(ctx, resourcesKey(connectionID), &resources); err != nil && err != redis.Nil {
		return
	}
	err = nil
	return
}

func (p *Poller) CachedTasks(ctx context.Context, connectionID string) ([]pve.Task, error) {
	var tasks []pve.Task
	if err := p.loadJSON(ctx, tasksKey(connectionID), &tasks); err != nil && err != redis.Nil {
		return nil, err
	}
	return tasks, nil
}

func (p *Poller) Ring(ctx context.Context, key string) ([]map[string]any, error) {
	vals, err := p.rdb.LRange(ctx, key, 0, ringMax-1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(vals))
	for i := len(vals) - 1; i >= 0; i-- {
		var m map[string]any
		if json.Unmarshal([]byte(vals[i]), &m) == nil {
			out = append(out, m)
		}
	}
	return out, nil
}

func ringKey(connectionID string, r pve.Resource) string {
	return "hypervane:" + connectionID + ":ring:" + r.Type + ":" + r.ID
}

func (p *Poller) pushRing(ctx context.Context, key string, sample map[string]any) {
	raw, err := json.Marshal(sample)
	if err != nil {
		return
	}
	pipe := p.rdb.TxPipeline()
	pipe.LPush(ctx, key, raw)
	pipe.LTrim(ctx, key, 0, ringMax-1)
	pipe.Expire(ctx, key, ringTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("ring push: %v", err)
	}
}

func (p *Poller) cacheJSON(ctx context.Context, key string, v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	if err := p.rdb.Set(ctx, key, raw, 2*time.Minute).Err(); err != nil {
		log.Printf("cache %s: %v", key, err)
	}
}

func (p *Poller) loadJSON(ctx context.Context, key string, dest any) error {
	raw, err := p.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

func splitN(s string, n int) []string {
	out := make([]string, 0, n)
	start := 0
	for i := 0; i < len(s) && len(out) < n-1; i++ {
		if s[i] == ':' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
