package pve

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (c *Client) CloneGuest(ctx context.Context, node, kind string, vmid int, req CloneRequest) (string, error) {
	form := url.Values{"newid": {strconv.Itoa(req.NewID)}}
	if req.Name != "" {
		if kind == "lxc" {
			form.Set("hostname", req.Name)
		} else {
			form.Set("name", req.Name)
		}
	}
	// Cloning to the guest's own node needs no target, and Proxmox only
	// accepts another node when the guest is on shared storage.
	if req.Target != "" && req.Target != node {
		form.Set("target", req.Target)
	}
	if req.Full {
		form.Set("full", "1")
	}
	// Proxmox rejects a storage on linked clones: they stay beside the template.
	if req.Storage != "" && req.Full {
		form.Set("storage", req.Storage)
	}
	if req.SnapName != "" {
		form.Set("snapname", req.SnapName)
	}
	var upid string
	err := c.do(ctx, http.MethodPost, c.guestBase(node, kind, vmid)+"/clone", form, &upid)
	return upid, err
}

func (c *Client) ConvertTemplate(ctx context.Context, node, kind string, vmid int) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.guestBase(node, kind, vmid)+"/template", url.Values{}, &upid)
	return upid, err
}

func (c *Client) MigrateGuest(ctx context.Context, node, kind string, vmid int, req MigrateRequest) (string, error) {
	form := url.Values{"target": {req.Target}}
	if req.Online {
		form.Set("online", "1")
	}
	if req.WithLocalDisks {
		form.Set("with-local-disks", "1")
	}
	if req.TargetStorage != "" {
		form.Set("targetstorage", req.TargetStorage)
	}
	if req.MigrationNetwork != "" {
		form.Set("migration_network", req.MigrationNetwork)
	}
	var upid string
	err := c.do(ctx, http.MethodPost, c.guestBase(node, kind, vmid)+"/migrate", form, &upid)
	return upid, err
}

func (c *Client) RemoteMigrateGuest(ctx context.Context, node, kind string, vmid int, req RemoteMigrateRequest) (string, error) {
	form := url.Values{}
	if req.Remote != "" {
		form.Set("remote", req.Remote)
	}
	if req.TargetEndpoint != "" {
		form.Set("target-endpoint", req.TargetEndpoint)
	}
	if req.Target != "" {
		form.Set("target", req.Target)
	}
	if req.TargetVMID > 0 {
		form.Set("target-vmid", strconv.Itoa(req.TargetVMID))
	}
	if req.TargetStorage != "" {
		form.Set("target-storage", req.TargetStorage)
	}
	if req.TargetBridge != "" {
		form.Set("target-bridge", req.TargetBridge)
	}
	if req.Online != nil && *req.Online {
		form.Set("online", "1")
	}
	if req.Delete != nil && *req.Delete {
		form.Set("delete", "1")
	}
	var upid string
	err := c.do(ctx, http.MethodPost, c.guestBase(node, kind, vmid)+"/remote_migrate", form, &upid)
	if IsNotFound(err) {
		return "", fmt.Errorf("remote_migrate is not available on this cluster")
	}
	return upid, err
}

func (c *Client) ClusterRemotes(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/sdn/remotes")
}

func (c *Client) CreateClusterRemote(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/sdn/remotes", mapForm(fields), nil)
}

func (c *Client) UpdateClusterRemote(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/sdn/remotes/"+url.PathEscape(id), mapForm(fields), nil)
}

func (c *Client) DeleteClusterRemote(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/sdn/remotes/"+url.PathEscape(id), nil, nil)
}

func (c *Client) ResizeDisk(ctx context.Context, node, kind string, vmid int, req DiskResizeRequest) (string, error) {
	form := url.Values{"disk": {req.Disk}, "size": {req.Size}}
	var upid string
	err := c.do(ctx, http.MethodPut, c.guestBase(node, kind, vmid)+"/resize", form, &upid)
	return upid, err
}

func (c *Client) MoveDisk(ctx context.Context, node, kind string, vmid int, req DiskMoveRequest) (string, error) {
	form := url.Values{"storage": {req.Storage}}
	path := c.guestBase(node, kind, vmid) + "/move_disk"
	if kind == "lxc" {
		form.Set("volume", req.Disk)
		path = c.guestBase(node, kind, vmid) + "/move_volume"
	} else {
		form.Set("disk", req.Disk)
	}
	if req.Format != "" {
		form.Set("format", req.Format)
	}
	if req.Delete {
		form.Set("delete", "1")
	}
	if req.Digest != "" {
		form.Set("digest", req.Digest)
	}
	var upid string
	err := c.do(ctx, http.MethodPost, path, form, &upid)
	return upid, err
}

func (c *Client) UnlinkDisk(ctx context.Context, node, kind string, vmid int, idlist string) error {
	form := url.Values{"idlist": {idlist}}
	return c.do(ctx, http.MethodPut, c.guestBase(node, kind, vmid)+"/unlink", form, nil)
}

func (c *Client) RescanDisks(ctx context.Context, node, kind string, vmid int) (DiskRescanResult, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.guestBase(node, kind, vmid)+"/rescan", url.Values{}, &upid)
	if IsNotFound(err) {
		return DiskRescanResult{Supported: false}, nil
	}
	if err != nil {
		return DiskRescanResult{}, err
	}
	return DiskRescanResult{Supported: true, UPID: upid}, nil
}

func (c *Client) CloudInit(ctx context.Context, node, kind string, vmid int) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodGet, c.guestBase(node, kind, vmid)+"/cloudinit", nil, &out)
	return out, err
}

func (c *Client) UpdateCloudInit(ctx context.Context, node, kind string, vmid int, cfg CloudInitConfig) error {
	fields := map[string]any{}
	if cfg.CIUser != "" {
		fields["ciuser"] = cfg.CIUser
	}
	if cfg.CIPassword != "" {
		fields["cipassword"] = cfg.CIPassword
	}
	if cfg.SSHKeys != "" {
		fields["sshkeys"] = EncodeSSHKeys(cfg.SSHKeys)
	}
	if cfg.IPConfig0 != "" {
		fields["ipconfig0"] = cfg.IPConfig0
	}
	if cfg.Nameserver != "" {
		fields["nameserver"] = cfg.Nameserver
	}
	if cfg.SearchDomain != "" {
		fields["searchdomain"] = cfg.SearchDomain
	}
	if cfg.CICustom != "" {
		fields["cicustom"] = cfg.CICustom
	}
	return c.UpdateGuestConfig(ctx, node, kind, vmid, fields, nil, cfg.Digest)
}

func (c *Client) RevertPending(ctx context.Context, node, kind string, vmid int, digest string) error {
	path := c.guestBase(node, kind, vmid) + "/pending"
	if digest != "" {
		path += "?digest=" + url.QueryEscape(digest)
	}
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) GuestFeature(ctx context.Context, node, kind, feature string, vmid int) (GuestFeatureResult, error) {
	switch feature {
	case "snapshot", "clone", "copy":
	default:
		return GuestFeatureResult{Supported: false, HasFeature: true}, nil
	}
	path := c.guestBase(node, kind, vmid) + "/feature?feature=" + url.QueryEscape(feature)
	var out map[string]any
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	if IsNotFound(err) {
		return GuestFeatureResult{Supported: false, HasFeature: true}, nil
	}
	if err != nil {
		return GuestFeatureResult{}, err
	}
	res := GuestFeatureResult{Supported: true, HasFeature: truthy(out["hasFeature"])}
	if nodes, ok := out["nodes"].([]any); ok {
		for _, n := range nodes {
			res.Nodes = append(res.Nodes, fmt.Sprint(n))
		}
	}
	return res, nil
}

func (c *Client) UpdateSnapshot(ctx context.Context, node, kind, name string, vmid int, description string) error {
	form := url.Values{}
	if description != "" {
		form.Set("description", description)
	}
	return c.do(ctx, http.MethodPut, c.guestBase(node, kind, vmid)+"/snapshot/"+url.PathEscape(name)+"/config", form, nil)
}

func (c *Client) CloudInitDump(ctx context.Context, node, kind, typ string, vmid int) (CloudInitDumpResult, error) {
	switch typ {
	case "user", "network", "meta":
	default:
		typ = "user"
	}
	path := c.guestBase(node, kind, vmid) + "/cloudinit/dump?type=" + url.QueryEscape(typ)
	var out any
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	if IsNotFound(err) {
		return CloudInitDumpResult{Supported: false}, nil
	}
	if err != nil {
		return CloudInitDumpResult{}, err
	}
	dump := ""
	if s, ok := out.(string); ok {
		dump = s
	} else if out != nil {
		dump = fmt.Sprint(out)
	}
	return CloudInitDumpResult{Supported: true, Dump: dump}, nil
}

func (c *Client) RegenerateCloudInit(ctx context.Context, node, kind string, vmid int) (DiskRescanResult, error) {
	var upid string
	err := c.do(ctx, http.MethodPut, c.guestBase(node, kind, vmid)+"/cloudinit", url.Values{}, &upid)
	if IsNotFound(err) {
		return DiskRescanResult{Supported: false}, nil
	}
	if err != nil {
		return DiskRescanResult{}, err
	}
	return DiskRescanResult{Supported: true, UPID: upid}, nil
}

func (c *Client) ImportMetadata(ctx context.Context, node, storage, volume string) (ImportMetadataResult, error) {
	q := url.Values{"volume": {volume}}
	path := "/nodes/" + url.PathEscape(node) + "/storage/" + url.PathEscape(storage) + "/import-metadata?" + q.Encode()
	var out map[string]any
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	if IsNotFound(err) {
		return ImportMetadataResult{Supported: false}, nil
	}
	if err != nil {
		return ImportMetadataResult{}, err
	}
	return ImportMetadataResult{Supported: true, Data: out}, nil
}

func (c *Client) QueryURLMetadata(ctx context.Context, node, rawURL string) (ImportMetadataResult, error) {
	q := url.Values{"url": {rawURL}}
	path := "/nodes/" + url.PathEscape(node) + "/query-url-metadata?" + q.Encode()
	var out map[string]any
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	if IsNotFound(err) {
		return ImportMetadataResult{Supported: false}, nil
	}
	if err != nil {
		return ImportMetadataResult{}, err
	}
	return ImportMetadataResult{Supported: true, Data: out}, nil
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case int:
		return t != 0
	case string:
		return t == "1" || strings.EqualFold(t, "true")
	default:
		return false
	}
}

func (c *Client) NodeHardware(ctx context.Context, node, kind string) (FeatureList, error) {
	switch kind {
	case "pci", "usb":
	default:
		return FeatureList{Supported: false}, nil
	}
	path := "/nodes/" + url.PathEscape(node) + "/hardware/" + url.PathEscape(kind)
	var out []map[string]any
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	if IsNotFound(err) {
		return FeatureList{Supported: false}, nil
	}
	if err != nil {
		return FeatureList{}, err
	}
	return FeatureList{Supported: true, Items: out}, nil
}

func (c *Client) NodeMdev(ctx context.Context, node, pciID string) (FeatureList, error) {
	path := "/nodes/" + url.PathEscape(node) + "/hardware/pci/" + url.PathEscape(pciID) + "/mdev"
	var out []map[string]any
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	if IsNotFound(err) {
		return FeatureList{Supported: false}, nil
	}
	if err != nil {
		return FeatureList{}, err
	}
	return FeatureList{Supported: true, Items: out}, nil
}

func (c *Client) QEMUCapabilities(ctx context.Context, node string) (NodeCapabilities, error) {
	var machines []map[string]any
	var cpus []map[string]any
	base := "/nodes/" + url.PathEscape(node) + "/capabilities/qemu"
	_ = c.do(ctx, http.MethodGet, base+"/machines", nil, &machines)
	_ = c.do(ctx, http.MethodGet, base+"/cpu", nil, &cpus)
	return NodeCapabilities{Machines: machines, CPUs: cpus}, nil
}

func (c *Client) DownloadURL(ctx context.Context, node, storage string, req DownloadURLRequest) (string, error) {
	form := url.Values{
		"url":      {req.URL},
		"filename": {req.Filename},
		"content":  {fallback(req.Content, "iso")},
	}
	if req.Checksum != "" {
		form.Set("checksum", req.Checksum)
	}
	path := "/nodes/" + url.PathEscape(node) + "/storage/" + url.PathEscape(storage) + "/download-url"
	var upid string
	err := c.do(ctx, http.MethodPost, path, form, &upid)
	return upid, err
}

func (c *Client) Upload(ctx context.Context, node, storage, content, filename string, body io.Reader) error {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		var err error
		defer func() {
			_ = mw.Close()
			_ = pw.CloseWithError(err)
		}()
		if err = mw.WriteField("content", content); err != nil {
			return
		}
		part, e := mw.CreateFormFile("filename", filename)
		if e != nil {
			err = e
			return
		}
		_, err = io.Copy(part, body)
	}()

	full, err := url.Parse(c.base.String() + "/nodes/" + url.PathEscape(node) + "/storage/" + url.PathEscape(storage) + "/upload")
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, full.String(), pr)
	if err != nil {
		return err
	}
	c.Authorize(req)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	client := c.longClient()
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		return &apiError{Status: res.StatusCode, Message: strings.TrimSpace(string(raw))}
	}
	return nil
}

func (c *Client) longClient() *http.Client {
	if t, ok := c.http.Transport.(*http.Transport); ok {
		return &http.Client{Timeout: 0, Transport: t.Clone()}
	}
	return &http.Client{Timeout: 0, Transport: c.http.Transport}
}

func (c *Client) AgentGet(ctx context.Context, node string, vmid int, command string) (any, error) {
	var out any
	err := c.do(ctx, http.MethodGet, c.guestBase(node, "qemu", vmid)+"/agent/"+command, nil, &out)
	return out, err
}

func (c *Client) AgentPost(ctx context.Context, node string, vmid int, command string, form url.Values) (any, error) {
	var out any
	if form == nil {
		form = url.Values{}
	}
	err := c.do(ctx, http.MethodPost, c.guestBase(node, "qemu", vmid)+"/agent/"+command, form, &out)
	return out, err
}

func (c *Client) BulkGuestAction(ctx context.Context, action string, req BulkRequest) ([]string, error) {
	ids := make([]string, 0, len(req.Guests))
	for _, g := range req.Guests {
		ids = append(ids, strconv.Itoa(g.VMID))
	}
	form := url.Values{"vms": {strings.Join(ids, ",")}}
	if req.Target != "" {
		form.Set("target", req.Target)
	}
	var upid string
	err := c.do(ctx, http.MethodPost, "/cluster/bulk-action/guest/"+action, form, &upid)
	if err == nil {
		if upid != "" {
			return []string{upid}, nil
		}
		return nil, nil
	}
	if apiErr, ok := IsAPIError(err); ok && apiErr.Status != 404 && apiErr.Status != 501 {
		return nil, err
	}

	online := true
	if req.Online != nil {
		online = *req.Online
	}
	withDisks := true
	if req.WithLocalDisks != nil {
		withDisks = *req.WithLocalDisks
	}

	upids := make([]string, 0, len(req.Guests))
	for _, g := range req.Guests {
		kind := kindOrQEMU(g.Type)
		var (
			u   string
			err error
		)
		switch action {
		case "migrate":
			u, err = c.MigrateGuest(ctx, g.Node, kind, g.VMID, MigrateRequest{
				Target:           req.Target,
				Online:           online,
				WithLocalDisks:   withDisks,
				TargetStorage:    req.TargetStorage,
				MigrationNetwork: req.MigrationNetwork,
			})
		case "start", "shutdown", "stop", "suspend":
			u, err = c.GuestPower(ctx, g.Node, kind, action, g.VMID)
		default:
			return nil, fmt.Errorf("unknown bulk action %s", action)
		}
		if err != nil {
			return upids, err
		}
		if u != "" {
			upids = append(upids, u)
		}
	}
	return upids, nil
}

func kindOrQEMU(t string) string {
	if t == "lxc" {
		return "lxc"
	}
	return "qemu"
}
