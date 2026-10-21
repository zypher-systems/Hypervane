package pve

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type TLSMode string

const (
	TLSSkipVerify TLSMode = "skip-verify"
	TLSPin        TLSMode = "pin"
)

type Options struct {
	BaseURL string
	Token   string
	Ticket  string
	CSRF    string
	TLSMode TLSMode
	TLSPin  string
	Timeout time.Duration
	Client  *http.Client
}

type Client struct {
	base   *url.URL
	token  string
	ticket string
	csrf   string
	http   *http.Client
	tlsPin string
}

type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("pve %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("pve %d", e.Status)
}

func New(opts Options) (*Client, error) {
	raw := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if raw == "" {
		return nil, fmt.Errorf("pve url is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("pve url: %w", err)
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/api2/json"
	} else if !strings.HasSuffix(u.Path, "/api2/json") {
		u.Path = strings.TrimRight(u.Path, "/") + "/api2/json"
	}
	if strings.TrimSpace(opts.Token) == "" && strings.TrimSpace(opts.Ticket) == "" {
		return nil, fmt.Errorf("pve token or ticket is required")
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Second
	}

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	switch opts.TLSMode {
	case TLSPin:
		pin := strings.ToLower(strings.ReplaceAll(opts.TLSPin, ":", ""))
		if pin == "" {
			return nil, fmt.Errorf("tls pin is required when tls mode is pin")
		}
		tlsCfg.InsecureSkipVerify = true
		tlsCfg.VerifyConnection = func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return fmt.Errorf("no peer certificate")
			}
			sum := sha256.Sum256(cs.PeerCertificates[0].Raw)
			got := hex.EncodeToString(sum[:])
			if got != pin {
				return fmt.Errorf("tls pin mismatch")
			}
			return nil
		}
	default:
		tlsCfg.InsecureSkipVerify = true
	}

	httpClient := opts.Client
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: tlsCfg,
			},
		}
	}

	return &Client{
		base:   u,
		token:  strings.TrimSpace(opts.Token),
		ticket: strings.TrimSpace(opts.Ticket),
		csrf:   strings.TrimSpace(opts.CSRF),
		http:   httpClient,
		tlsPin: opts.TLSPin,
	}, nil
}

func (c *Client) WithTicket(ticket, csrf string) *Client {
	if c == nil {
		return nil
	}
	clone := *c
	clone.token = ""
	clone.ticket = strings.TrimSpace(ticket)
	clone.csrf = strings.TrimSpace(csrf)
	return &clone
}

func (c *Client) Authorize(req *http.Request) {
	if c == nil || req == nil {
		return
	}
	if c.ticket != "" {
		req.Header.Set("Cookie", "PVEAuthCookie="+c.ticket)
		if c.csrf != "" {
			req.Header.Set("CSRFPreventionToken", c.csrf)
		}
		return
	}
	if c.token != "" {
		req.Header.Set("Authorization", "PVEAPIToken="+c.token)
	}
}

func (c *Client) BaseURL() string {
	return c.base.String()
}

func (c *Client) Version(ctx context.Context) (Version, error) {
	var v Version
	err := c.do(ctx, http.MethodGet, "/version", nil, &v)
	return v, err
}

func (c *Client) Permissions(ctx context.Context) (map[string]any, error) {
	var perms map[string]any
	err := c.do(ctx, http.MethodGet, "/access/permissions", nil, &perms)
	return perms, err
}

func (c *Client) ClusterStatus(ctx context.Context) ([]ClusterStatus, error) {
	var out []ClusterStatus
	err := c.do(ctx, http.MethodGet, "/cluster/status", nil, &out)
	return out, err
}

func (c *Client) ClusterResources(ctx context.Context) ([]Resource, error) {
	var out []Resource
	err := c.do(ctx, http.MethodGet, "/cluster/resources", nil, &out)
	return out, err
}

func (c *Client) NextID(ctx context.Context) (int, error) {
	var raw any
	if err := c.do(ctx, http.MethodGet, "/cluster/nextid", nil, &raw); err != nil {
		return 0, err
	}
	switch v := raw.(type) {
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("unexpected nextid type %T", raw)
	}
}

func (c *Client) NodeStatus(ctx context.Context, node string) (NodeStatus, error) {
	var out NodeStatus
	err := c.do(ctx, http.MethodGet, "/nodes/"+url.PathEscape(node)+"/status", nil, &out)
	return out, err
}

func (c *Client) NodeRRD(ctx context.Context, node, timeframe string) ([]RRDPoint, error) {
	q := url.Values{"timeframe": {timeframe}, "cf": {"AVERAGE"}}
	var out []RRDPoint
	err := c.do(ctx, http.MethodGet, "/nodes/"+url.PathEscape(node)+"/rrddata?"+q.Encode(), nil, &out)
	return out, err
}

func (c *Client) guestBase(node, kind string, vmid int) string {
	return "/nodes/" + url.PathEscape(node) + "/" + kind + "/" + strconv.Itoa(vmid)
}

func (c *Client) GuestStatus(ctx context.Context, node, kind string, vmid int) (GuestStatus, error) {
	var out GuestStatus
	err := c.do(ctx, http.MethodGet, c.guestBase(node, kind, vmid)+"/status/current", nil, &out)
	return out, err
}

func (c *Client) GuestConfig(ctx context.Context, node, kind string, vmid int) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodGet, c.guestBase(node, kind, vmid)+"/config", nil, &out)
	return out, err
}

func (c *Client) GuestPending(ctx context.Context, node, kind string, vmid int) ([]PendingChange, error) {
	var out []PendingChange
	err := c.do(ctx, http.MethodGet, c.guestBase(node, kind, vmid)+"/pending", nil, &out)
	return out, err
}

func (c *Client) UpdateGuestConfig(ctx context.Context, node, kind string, vmid int, fields map[string]any, deleteKeys []string, digest string) error {
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, fmt.Sprint(v))
	}
	if len(deleteKeys) > 0 {
		form.Set("delete", strings.Join(deleteKeys, ","))
	}
	if digest != "" {
		form.Set("digest", digest)
	}
	return c.do(ctx, http.MethodPut, c.guestBase(node, kind, vmid)+"/config", form, nil)
}

func (c *Client) GuestPower(ctx context.Context, node, kind, action string, vmid int) (string, error) {
	form := url.Values{}
	pathAction := action
	if action == "hibernate" {
		if kind != "qemu" {
			return "", fmt.Errorf("hibernate is only available for qemu")
		}
		// PVE has no /status/hibernate. Suspend-to-disk is POST /status/suspend?todisk=1.
		pathAction = "suspend"
		form.Set("todisk", "1")
	}
	var upid string
	err := c.do(ctx, http.MethodPost, c.guestBase(node, kind, vmid)+"/status/"+pathAction, form, &upid)
	return upid, err
}

func (c *Client) DeleteGuest(ctx context.Context, node, kind string, vmid int, opts DeleteGuestOptions) (string, error) {
	q := url.Values{}
	if opts.Purge {
		q.Set("purge", "1")
	}
	if opts.Force {
		q.Set("force", "1")
	}
	if opts.DestroyUnreferencedDisks {
		q.Set("destroy-unreferenced-disks", "1")
	}
	path := c.guestBase(node, kind, vmid)
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var upid string
	err := c.do(ctx, http.MethodDelete, path, nil, &upid)
	return upid, err
}

func (c *Client) CreateQEMU(ctx context.Context, req CreateGuestRequest) (string, error) {
	form := url.Values{}
	form.Set("vmid", strconv.Itoa(req.VMID))
	for k, v := range req.CreateArgs {
		if k == "" || k == "vmid" {
			continue
		}
		setFormAny(form, k, v)
	}
	if req.Name != "" {
		form.Set("name", req.Name)
	}
	importing := len(req.ImportFrom) > 0
	if req.Memory > 0 {
		form.Set("memory", strconv.Itoa(req.Memory))
	} else if form.Get("memory") == "" && !importing {
		form.Set("memory", strconv.Itoa(nonzero(req.Memory, 2048)))
	}
	if req.Cores > 0 {
		form.Set("cores", strconv.Itoa(req.Cores))
	} else if form.Get("cores") == "" && !importing {
		form.Set("cores", strconv.Itoa(nonzero(req.Cores, 2)))
	}
	if req.SCSIHW != "" {
		form.Set("scsihw", req.SCSIHW)
	} else if form.Get("scsihw") == "" && !importing {
		form.Set("scsihw", "virtio-scsi-pci")
	}
	if len(req.Nets) > 0 {
		for slot, val := range req.Nets {
			if slot != "" && val != "" {
				form.Set(slot, val)
			}
		}
	} else if form.Get("net0") == "" && (!importing || req.Bridge != "") {
		form.Set("net0", "virtio,bridge="+fallback(req.Bridge, "vmbr0"))
	}
	if importing {
		dest := fallback(req.Storage, "local")
		for slot, src := range req.ImportFrom {
			if slot == "" || src == "" {
				continue
			}
			store := fallback(req.ImportDest[slot], dest)
			val := fmt.Sprintf("%s:0,import-from=%s", store, src)
			if req.DiskOptions != "" {
				val += "," + req.DiskOptions
			}
			form.Set(slot, val)
		}
		if req.ImportWorkingStorage != "" {
			form.Set("import-working-storage", req.ImportWorkingStorage)
		}
	} else if req.Storage != "" && req.DiskGB > 0 {
		form.Set("scsi0", fmt.Sprintf("%s:%d", req.Storage, req.DiskGB))
	}
	if req.ISO != "" {
		form.Set("ide2", req.ISO+",media=cdrom")
	}
	if req.OSType != "" {
		form.Set("ostype", req.OSType)
	}
	if req.Bios != "" {
		form.Set("bios", req.Bios)
	}
	if req.Machine != "" {
		form.Set("machine", req.Machine)
	}
	if req.CPU != "" {
		form.Set("cpu", req.CPU)
	}
	if req.Sockets > 0 {
		form.Set("sockets", strconv.Itoa(req.Sockets))
	}
	if req.VGA != "" {
		form.Set("vga", req.VGA)
	}
	if req.Tablet != nil {
		if *req.Tablet {
			form.Set("tablet", "1")
		} else {
			form.Set("tablet", "0")
		}
	}
	if req.Args != "" {
		form.Set("args", req.Args)
	}
	if req.Agent {
		form.Set("agent", "1")
	}
	if req.CIUser != "" {
		form.Set("ciuser", req.CIUser)
	}
	if req.SSHKeys != "" {
		form.Set("sshkeys", EncodeSSHKeys(req.SSHKeys))
	}
	if req.IPConfig0 != "" {
		form.Set("ipconfig0", req.IPConfig0)
	}
	var upid string
	err := c.do(ctx, http.MethodPost, "/nodes/"+url.PathEscape(req.Node)+"/qemu", form, &upid)
	return upid, err
}

func (c *Client) CreateLXC(ctx context.Context, req CreateGuestRequest) (string, error) {
	form := url.Values{}
	form.Set("vmid", strconv.Itoa(req.VMID))
	if req.Name != "" {
		form.Set("hostname", req.Name)
	}
	form.Set("memory", strconv.Itoa(nonzero(req.Memory, 512)))
	form.Set("cores", strconv.Itoa(nonzero(req.Cores, 1)))
	form.Set("net0", "name=eth0,bridge="+fallback(req.Bridge, "vmbr0")+",ip=dhcp")
	if req.Storage != "" && req.DiskGB > 0 {
		form.Set("rootfs", fmt.Sprintf("%s:%d", req.Storage, req.DiskGB))
	}
	if req.Template != "" {
		form.Set("ostemplate", req.Template)
	}
	if req.Password != "" {
		form.Set("password", req.Password)
	}
	unpriv := true
	if req.Unprivileged != nil {
		unpriv = *req.Unprivileged
	}
	if unpriv {
		form.Set("unprivileged", "1")
	}
	if req.SSHKeys != "" {
		// Containers take plain OpenSSH keys, under a different name than QEMU.
		form.Set("ssh-public-keys", req.SSHKeys)
	}
	if req.IPConfig0 != "" {
		form.Set("ipconfig0", req.IPConfig0)
	}
	var upid string
	err := c.do(ctx, http.MethodPost, "/nodes/"+url.PathEscape(req.Node)+"/lxc", form, &upid)
	return upid, err
}

func (c *Client) Snapshots(ctx context.Context, node, kind string, vmid int) ([]Snapshot, error) {
	var out []Snapshot
	err := c.do(ctx, http.MethodGet, c.guestBase(node, kind, vmid)+"/snapshot", nil, &out)
	return out, err
}

func (c *Client) CreateSnapshot(ctx context.Context, node, kind string, vmid int, req SnapshotRequest) (string, error) {
	form := url.Values{"snapname": {req.Name}}
	if req.Description != "" {
		form.Set("description", req.Description)
	}
	if req.VMState {
		form.Set("vmstate", "1")
	}
	var upid string
	err := c.do(ctx, http.MethodPost, c.guestBase(node, kind, vmid)+"/snapshot", form, &upid)
	return upid, err
}

func (c *Client) DeleteSnapshot(ctx context.Context, node, kind, name string, vmid int) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodDelete, c.guestBase(node, kind, vmid)+"/snapshot/"+url.PathEscape(name), nil, &upid)
	return upid, err
}

func (c *Client) RollbackSnapshot(ctx context.Context, node, kind, name string, vmid int) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.guestBase(node, kind, vmid)+"/snapshot/"+url.PathEscape(name)+"/rollback", url.Values{}, &upid)
	return upid, err
}

func (c *Client) GuestRRD(ctx context.Context, node, kind string, vmid int, timeframe string) ([]RRDPoint, error) {
	q := url.Values{"timeframe": {timeframe}, "cf": {"AVERAGE"}}
	var out []RRDPoint
	err := c.do(ctx, http.MethodGet, c.guestBase(node, kind, vmid)+"/rrddata?"+q.Encode(), nil, &out)
	return out, err
}

func (c *Client) Storages(ctx context.Context) ([]Storage, error) {
	var out []Storage
	err := c.do(ctx, http.MethodGet, "/storage", nil, &out)
	return out, err
}

func (c *Client) StorageContent(ctx context.Context, node, storage, content string) ([]StorageContent, error) {
	q := url.Values{}
	if content != "" {
		q.Set("content", content)
	}
	path := "/nodes/" + url.PathEscape(node) + "/storage/" + url.PathEscape(storage) + "/content"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var out []StorageContent
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

func (c *Client) DeleteVolume(ctx context.Context, node, storage, volume string) (string, error) {
	var upid string
	path := "/nodes/" + url.PathEscape(node) + "/storage/" + url.PathEscape(storage) + "/content/" + url.PathEscape(volume)
	err := c.do(ctx, http.MethodDelete, path, nil, &upid)
	return upid, err
}

func (c *Client) StorageRRD(ctx context.Context, node, storage, timeframe string) ([]RRDPoint, error) {
	q := url.Values{"timeframe": {timeframe}, "cf": {"AVERAGE"}}
	path := "/nodes/" + url.PathEscape(node) + "/storage/" + url.PathEscape(storage) + "/rrddata?" + q.Encode()
	var out []RRDPoint
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

func (c *Client) ClusterTasks(ctx context.Context) ([]Task, error) {
	var out []Task
	err := c.do(ctx, http.MethodGet, "/cluster/tasks", nil, &out)
	return out, err
}

type NodeTaskQuery struct {
	VMID       string
	Source     string
	Start      int
	Limit      int
	UserFilter string
	TypeFilter string
}

func (c *Client) NodeTasks(ctx context.Context, node string, opts NodeTaskQuery) ([]Task, error) {
	q := url.Values{}
	if opts.Source != "" {
		q.Set("source", opts.Source)
	}
	if opts.VMID != "" {
		q.Set("vmid", opts.VMID)
	}
	if opts.Start > 0 {
		q.Set("start", strconv.Itoa(opts.Start))
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.UserFilter != "" {
		q.Set("userfilter", opts.UserFilter)
	}
	if opts.TypeFilter != "" {
		q.Set("typefilter", opts.TypeFilter)
	}
	path := "/nodes/" + url.PathEscape(node) + "/tasks"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var out []Task
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

func (c *Client) TaskStatus(ctx context.Context, node, upid string) (TaskStatus, error) {
	var out TaskStatus
	err := c.do(ctx, http.MethodGet, "/nodes/"+url.PathEscape(node)+"/tasks/"+url.PathEscape(upid)+"/status", nil, &out)
	return out, err
}

func (c *Client) WaitTask(ctx context.Context, node, upid string) (TaskStatus, error) {
	for {
		if err := ctx.Err(); err != nil {
			return TaskStatus{}, err
		}
		st, err := c.TaskStatus(ctx, node, upid)
		if err != nil {
			return st, err
		}
		if strings.EqualFold(st.Status, "stopped") {
			if st.ExitStatus != "" && st.ExitStatus != "OK" {
				return st, fmt.Errorf("task %s: %s", upid, st.ExitStatus)
			}
			return st, nil
		}
		select {
		case <-ctx.Done():
			return st, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (c *Client) TaskLog(ctx context.Context, node, upid string, start int) ([]TaskLogLine, error) {
	q := url.Values{"start": {strconv.Itoa(start)}}
	var out []TaskLogLine
	err := c.do(ctx, http.MethodGet, "/nodes/"+url.PathEscape(node)+"/tasks/"+url.PathEscape(upid)+"/log?"+q.Encode(), nil, &out)
	return out, err
}

// TaskLogTail returns the last n lines of a task log and the log's total line
// count. Proxmox serves only 50 lines unless asked for more, which cut long
// logs (a backup of many guests) off after the first entries.
func (c *Client) TaskLogTail(ctx context.Context, node, upid string, n int) ([]TaskLogLine, int, error) {
	if n <= 0 {
		n = 500
	}
	base := "/nodes/" + url.PathEscape(node) + "/tasks/" + url.PathEscape(upid) + "/log?"
	page := func(start, limit int) ([]TaskLogLine, int, error) {
		q := url.Values{"start": {strconv.Itoa(start)}, "limit": {strconv.Itoa(limit)}}
		raw, err := c.send(ctx, http.MethodGet, base+q.Encode(), nil)
		if err != nil {
			return nil, 0, err
		}
		var env struct {
			Data  []TaskLogLine `json:"data"`
			Total int           `json:"total"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, 0, fmt.Errorf("decode data: %w", err)
		}
		return env.Data, env.Total, nil
	}
	lines, total, err := page(0, n)
	if err != nil {
		return nil, 0, err
	}
	if total > len(lines) && total > n {
		lines, total, err = page(total-n, n)
		if err != nil {
			return nil, 0, err
		}
	}
	if lines == nil {
		lines = []TaskLogLine{}
	}
	return lines, total, nil
}

func (c *Client) StopTask(ctx context.Context, node, upid string) error {
	return c.do(ctx, http.MethodDelete, "/nodes/"+url.PathEscape(node)+"/tasks/"+url.PathEscape(upid), nil, nil)
}

func (c *Client) VNCProxy(ctx context.Context, node, kind string, vmid int) (VNCProxy, error) {
	form := url.Values{"websocket": {"1"}}
	var out VNCProxy
	err := c.do(ctx, http.MethodPost, c.guestBase(node, kind, vmid)+"/vncproxy", form, &out)
	return out, err
}

func (c *Client) TermProxy(ctx context.Context, node, kind string, vmid int) (TermProxy, error) {
	var out TermProxy
	path := "/nodes/" + url.PathEscape(node) + "/termproxy"
	if kind != "node" {
		path = c.guestBase(node, kind, vmid) + "/termproxy"
	}
	err := c.do(ctx, http.MethodPost, path, url.Values{}, &out)
	return out, err
}

func (c *Client) NodeTermProxy(ctx context.Context, node string) (TermProxy, error) {
	return c.TermProxy(ctx, node, "node", 0)
}

// NodeCommandTermProxy opens a node terminal that runs one of Proxmox's
// built-in shell commands. "upgrade" runs pveupgrade, which is what the stock
// UI's Upgrade button does. Proxmox runs it only for root@pam; anyone else,
// API tokens included, gets a login prompt instead.
func (c *Client) NodeCommandTermProxy(ctx context.Context, node, cmd string) (TermProxy, error) {
	var out TermProxy
	err := c.do(ctx, http.MethodPost, "/nodes/"+url.PathEscape(node)+"/termproxy", url.Values{"cmd": {cmd}}, &out)
	return out, err
}

func (c *Client) NodeVNCShell(ctx context.Context, node string) (VNCProxy, error) {
	form := url.Values{"websocket": {"1"}}
	var out VNCProxy
	err := c.do(ctx, http.MethodPost, "/nodes/"+url.PathEscape(node)+"/vncshell", form, &out)
	return out, err
}

func (c *Client) NodeSpiceShell(ctx context.Context, node string) (SpiceProxy, error) {
	form := url.Values{"websocket": {"1"}}
	var out SpiceProxy
	err := c.do(ctx, http.MethodPost, "/nodes/"+url.PathEscape(node)+"/spiceshell", form, &out)
	if out.Ticket == "" && out.Password != "" {
		out.Ticket = out.Password
	}
	if out.Port == "" && out.TLSPort > 0 {
		out.Port = strconv.Itoa(out.TLSPort)
	}
	return out, err
}

func (c *Client) SpiceProxy(ctx context.Context, node string, vmid int) (SpiceProxy, error) {
	form := url.Values{"websocket": {"1"}}
	var out SpiceProxy
	err := c.do(ctx, http.MethodPost, c.guestBase(node, "qemu", vmid)+"/spiceproxy", form, &out)
	if out.Ticket == "" && out.Password != "" {
		out.Ticket = out.Password
	}
	if out.Port == "" && out.TLSPort > 0 {
		out.Port = strconv.Itoa(out.TLSPort)
	}
	return out, err
}

func (c *Client) SendKey(ctx context.Context, node string, vmid int, key string) error {
	form := url.Values{"key": {key}}
	return c.do(ctx, http.MethodPut, c.guestBase(node, "qemu", vmid)+"/sendkey", form, nil)
}

func (c *Client) QEMUMonitor(ctx context.Context, node string, vmid int, command string) (string, error) {
	form := url.Values{"command": {command}}
	var out string
	err := c.do(ctx, http.MethodPost, c.guestBase(node, "qemu", vmid)+"/monitor", form, &out)
	return out, err
}

func (c *Client) WebsocketURL(path string, query url.Values) string {
	u := *c.base
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = strings.TrimSuffix(u.Path, "/api2/json") + "/api2/json" + path
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) Token() string {
	return c.token
}

func (c *Client) Ticket() string {
	return c.ticket
}

func (c *Client) CSRF() string {
	return c.csrf
}

func (c *Client) HTTP() *http.Client {
	return c.http
}

func (c *Client) do(ctx context.Context, method, path string, body url.Values, dest any) error {
	raw, err := c.send(ctx, method, path, body)
	if err != nil {
		return err
	}
	if dest == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, dest); err != nil {
		return fmt.Errorf("decode data: %w", err)
	}
	return nil
}

// send performs one authorized request and returns the raw response body.
func (c *Client) send(ctx context.Context, method, path string, body url.Values) ([]byte, error) {
	full, err := url.Parse(c.base.String() + path)
	if err != nil {
		return nil, err
	}
	u := *full

	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(body.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return nil, err
	}
	c.Authorize(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, parseAPIError(res.StatusCode, raw)
	}
	return raw, nil
}

func nonzero(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

func fallback(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func setFormAny(form url.Values, key string, v any) {
	if v == nil {
		return
	}
	switch t := v.(type) {
	case string:
		if t != "" {
			form.Set(key, t)
		}
	case bool:
		if t {
			form.Set(key, "1")
		} else {
			form.Set(key, "0")
		}
	case int:
		form.Set(key, strconv.Itoa(t))
	case int64:
		form.Set(key, strconv.FormatInt(t, 10))
	case float64:
		if t == float64(int64(t)) {
			form.Set(key, strconv.FormatInt(int64(t), 10))
		} else {
			form.Set(key, strconv.FormatFloat(t, 'f', -1, 64))
		}
	case json.Number:
		form.Set(key, t.String())
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s != "" && s != "<nil>" {
			form.Set(key, s)
		}
	}
}

func ParseUPIDNode(upid string) string {
	node, _, _ := ParseUPID(upid)
	return node
}

// ParseUPID reads UPID:<node>:<pid>:<pstart>:<starttime>:<type>:<id>:...
func ParseUPID(upid string) (node, typ, id string) {
	parts := strings.Split(upid, ":")
	if len(parts) < 2 {
		return "", "", ""
	}
	node = parts[1]
	if len(parts) >= 7 {
		typ, id = parts[5], parts[6]
	}
	return node, typ, id
}

func IsAPIError(err error) (*apiError, bool) {
	e, ok := err.(*apiError)
	return e, ok
}

func IsNotFound(err error) bool {
	e, ok := IsAPIError(err)
	return ok && e.Status == http.StatusNotFound
}

// IsForbidden reports a 403: the caller may not see or do this.
func IsForbidden(err error) bool {
	e, ok := IsAPIError(err)
	return ok && e.Status == http.StatusForbidden
}

func IsUnavailable(err error) bool {
	e, ok := IsAPIError(err)
	return ok && (e.Status == http.StatusNotFound || e.Status == http.StatusNotImplemented)
}

func parseAPIError(status int, raw []byte) error {
	msg := strings.TrimSpace(string(raw))
	var parsed struct {
		Message string         `json:"message"`
		Errors  map[string]any `json:"errors"`
	}
	if json.Unmarshal(raw, &parsed) == nil {
		if parsed.Message != "" {
			msg = parsed.Message
		}
		if len(parsed.Errors) > 0 {
			b, _ := json.Marshal(parsed.Errors)
			detail := string(b)
			if msg == "" || msg == string(raw) {
				msg = detail
			} else {
				msg = strings.TrimSpace(msg) + " " + detail
			}
		}
	}
	return &apiError{Status: status, Message: msg}
}

// EncodeSSHKeys encodes OpenSSH public keys the way Proxmox stores a QEMU
// guest's sshkeys: percent-encoded, keeping only - _ . ! ~ * ' ( ) and
// alphanumerics, which is all its "urlencoded" format accepts. Proxmox decodes
// without turning + into a space, so form-style encoding breaks the keys.
// Already-encoded input is decoded first, so it is never encoded twice.
func EncodeSSHKeys(keys string) string {
	if dec, err := url.PathUnescape(keys); err == nil && strings.Contains(keys, "%") {
		keys = dec
	}
	keys = strings.TrimSpace(strings.ReplaceAll(keys, "\r\n", "\n"))
	var b strings.Builder
	for i := 0; i < len(keys); i++ {
		ch := keys[i]
		switch {
		case 'a' <= ch && ch <= 'z', 'A' <= ch && ch <= 'Z', '0' <= ch && ch <= '9',
			strings.IndexByte("-_.!~*'()", ch) >= 0:
			b.WriteByte(ch)
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}
