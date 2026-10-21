package pve

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func nodePath(node, suffix string) string {
	return "/nodes/" + url.PathEscape(node) + suffix
}

func (c *Client) NodeNetwork(ctx context.Context, node string) ([]NetIface, error) {
	var out []NetIface
	err := c.do(ctx, http.MethodGet, nodePath(node, "/network"), nil, &out)
	return out, err
}

func (c *Client) CreateNodeNetwork(ctx context.Context, node string, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, nodePath(node, "/network"), mapForm(fields), nil)
}

func (c *Client) UpdateNodeNetwork(ctx context.Context, node, iface string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, nodePath(node, "/network/"+url.PathEscape(iface)), mapForm(fields), nil)
}

func (c *Client) DeleteNodeNetwork(ctx context.Context, node, iface, digest string) error {
	path := nodePath(node, "/network/"+url.PathEscape(iface))
	if digest != "" {
		path += "?digest=" + url.QueryEscape(digest)
	}
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) ApplyNodeNetwork(ctx context.Context, node string) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPut, nodePath(node, "/network"), nil, &upid)
	return upid, err
}

func (c *Client) RevertNodeNetwork(ctx context.Context, node string) error {
	return c.do(ctx, http.MethodDelete, nodePath(node, "/network"), nil, nil)
}

func (c *Client) NodeDisks(ctx context.Context, node string) ([]DiskInfo, error) {
	var out []DiskInfo
	err := c.do(ctx, http.MethodGet, nodePath(node, "/disks/list"), nil, &out)
	return out, err
}

func (c *Client) NodeDiskSMART(ctx context.Context, node, disk string) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodGet, nodePath(node, "/disks/smart?disk="+url.QueryEscape(disk)), nil, &out)
	return out, err
}

func (c *Client) WipeNodeDisk(ctx context.Context, node, disk string) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPut, nodePath(node, "/disks/wipedisk"), url.Values{"disk": {disk}}, &upid)
	return upid, err
}

func (c *Client) NodeDiskBackends(ctx context.Context, node, kind string) ([]map[string]any, error) {
	path, err := diskKindPath(kind)
	if err != nil {
		return nil, err
	}
	// LVM answers with a tree ({leaf, children: [volume groups]}), the other
	// backends with a list.
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, nodePath(node, path), nil, &raw); err != nil {
		return nil, err
	}
	out, err := decodeFlexibleMaps(raw)
	if err != nil {
		return nil, fmt.Errorf("decode data: %w", err)
	}
	return out, nil
}

func (c *Client) CreateNodeDiskBackend(ctx context.Context, node, kind string, fields map[string]any) (string, error) {
	path, err := diskKindPath(kind)
	if err != nil {
		return "", err
	}
	if _, ok := fields["add_storage"]; !ok {
		fields["add_storage"] = 1
	}
	var upid string
	err = c.do(ctx, http.MethodPost, nodePath(node, path), mapForm(fields), &upid)
	return upid, err
}

func (c *Client) DeleteNodeDiskBackend(ctx context.Context, node, kind, name string) (string, error) {
	path, err := diskKindPath(kind)
	if err != nil {
		return "", err
	}
	var upid string
	err = c.do(ctx, http.MethodDelete, nodePath(node, path+"/"+url.PathEscape(name)), nil, &upid)
	return upid, err
}

func diskKindPath(kind string) (string, error) {
	switch kind {
	case "lvm":
		return "/disks/lvm", nil
	case "lvmthin":
		return "/disks/lvmthin", nil
	case "directory", "dir":
		return "/disks/directory", nil
	case "zfs":
		return "/disks/zfs", nil
	default:
		return "", fmt.Errorf("unsupported disk backend %s", kind)
	}
}

func (c *Client) CreateStorage(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/storage", mapForm(fields), nil)
}

func (c *Client) APTUpdates(ctx context.Context, node string) ([]map[string]any, error) {
	var out []map[string]any
	err := c.do(ctx, http.MethodGet, nodePath(node, "/apt/update"), nil, &out)
	return out, err
}

func (c *Client) APTUpdate(ctx context.Context, node string) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, nodePath(node, "/apt/update"), nil, &upid)
	return upid, err
}

func (c *Client) APTVersions(ctx context.Context, node string) ([]map[string]any, error) {
	var out []map[string]any
	err := c.do(ctx, http.MethodGet, nodePath(node, "/apt/versions"), nil, &out)
	return out, err
}

func (c *Client) APTChangelog(ctx context.Context, node, name string) (string, error) {
	var out string
	err := c.do(ctx, http.MethodGet, nodePath(node, "/apt/changelog?name="+url.QueryEscape(name)), nil, &out)
	return out, err
}

func (c *Client) APTRepositories(ctx context.Context, node string) (APTRepositories, error) {
	var out APTRepositories
	err := c.do(ctx, http.MethodGet, nodePath(node, "/apt/repositories"), nil, &out)
	return out, err
}

func (c *Client) UpdateAPTRepository(ctx context.Context, node string, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, nodePath(node, "/apt/repositories"), mapForm(fields), nil)
}

func (c *Client) NodeServices(ctx context.Context, node string) ([]NodeService, error) {
	var out []NodeService
	err := c.do(ctx, http.MethodGet, nodePath(node, "/services"), nil, &out)
	return out, err
}

func (c *Client) NodeServiceAction(ctx context.Context, node, service, action string) (string, error) {
	switch action {
	case "start", "stop", "restart", "reload":
	default:
		return "", fmt.Errorf("unsupported service action")
	}
	var upid string
	err := c.do(ctx, http.MethodPost, nodePath(node, "/services/"+url.PathEscape(service)+"/"+action), nil, &upid)
	return upid, err
}

func (c *Client) NodeDNS(ctx context.Context, node string) (NodeDNS, error) {
	var out NodeDNS
	err := c.do(ctx, http.MethodGet, nodePath(node, "/dns"), nil, &out)
	return out, err
}

func (c *Client) UpdateNodeDNS(ctx context.Context, node string, req NodeDNS) error {
	form := url.Values{}
	if req.Search != "" {
		form.Set("search", req.Search)
	}
	if req.DNS1 != "" {
		form.Set("dns1", req.DNS1)
	}
	if req.DNS2 != "" {
		form.Set("dns2", req.DNS2)
	}
	if req.DNS3 != "" {
		form.Set("dns3", req.DNS3)
	}
	return c.do(ctx, http.MethodPut, nodePath(node, "/dns"), form, nil)
}

func (c *Client) NodeTime(ctx context.Context, node string) (NodeTime, error) {
	var out NodeTime
	err := c.do(ctx, http.MethodGet, nodePath(node, "/time"), nil, &out)
	return out, err
}

func (c *Client) UpdateNodeTime(ctx context.Context, node, timezone string) error {
	return c.do(ctx, http.MethodPut, nodePath(node, "/time"), url.Values{"timezone": {timezone}}, nil)
}

func (c *Client) NodeHosts(ctx context.Context, node string) (NodeHosts, error) {
	var out NodeHosts
	err := c.do(ctx, http.MethodGet, nodePath(node, "/hosts"), nil, &out)
	return out, err
}

func (c *Client) UpdateNodeHosts(ctx context.Context, node string, req NodeHosts) error {
	form := url.Values{"data": {req.Data}}
	if req.Digest != "" {
		form.Set("digest", req.Digest)
	}
	return c.do(ctx, http.MethodPost, nodePath(node, "/hosts"), form, nil)
}

func (c *Client) NodeJournal(ctx context.Context, node string, start, last int) ([]JournalLine, error) {
	q := url.Values{}
	if start > 0 {
		q.Set("start", strconv.Itoa(start))
	}
	if last > 0 {
		q.Set("lastentries", strconv.Itoa(last))
	}
	path := nodePath(node, "/journal")
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var raw any
	err := c.do(ctx, http.MethodGet, path, nil, &raw)
	if IsNotFound(err) {
		return c.nodeSyslog(ctx, node, start)
	}
	if err != nil {
		return nil, err
	}
	return normalizeJournal(raw), nil
}

func (c *Client) nodeSyslog(ctx context.Context, node string, start int) ([]JournalLine, error) {
	path := nodePath(node, "/syslog")
	if start > 0 {
		path += "?start=" + strconv.Itoa(start)
	}
	var out []JournalLine
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

func normalizeJournal(raw any) []JournalLine {
	switch v := raw.(type) {
	case []any:
		out := make([]JournalLine, 0, len(v))
		for i, item := range v {
			switch line := item.(type) {
			case string:
				out = append(out, JournalLine{N: i, T: line})
			case map[string]any:
				n, _ := line["n"].(float64)
				t, _ := line["t"].(string)
				if t == "" {
					t = fmt.Sprint(line)
				}
				out = append(out, JournalLine{N: int(n), T: t})
			}
		}
		return out
	default:
		return nil
	}
}

func (c *Client) NodeSubscription(ctx context.Context, node string) (NodeSubscription, error) {
	var out NodeSubscription
	err := c.do(ctx, http.MethodGet, nodePath(node, "/subscription"), nil, &out)
	return out, err
}

func (c *Client) SetNodeSubscription(ctx context.Context, node, key string) error {
	return c.do(ctx, http.MethodPut, nodePath(node, "/subscription"), url.Values{"key": {key}}, nil)
}

func (c *Client) NodeCertificates(ctx context.Context, node string) ([]CertInfo, error) {
	var out []CertInfo
	err := c.do(ctx, http.MethodGet, nodePath(node, "/certificates/info"), nil, &out)
	return out, err
}

func (c *Client) UploadNodeCertificate(ctx context.Context, node string, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, nodePath(node, "/certificates/custom"), mapForm(fields), nil)
}

func (c *Client) DeleteNodeCertificate(ctx context.Context, node string) error {
	return c.do(ctx, http.MethodDelete, nodePath(node, "/certificates/custom"), nil, nil)
}

func (c *Client) OrderACMECertificate(ctx context.Context, node string, force bool) (string, error) {
	form := url.Values{}
	if force {
		form.Set("force", "1")
	}
	var upid string
	err := c.do(ctx, http.MethodPost, nodePath(node, "/certificates/acme/certificate"), form, &upid)
	return upid, err
}

func (c *Client) RenewACMECertificate(ctx context.Context, node string) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPut, nodePath(node, "/certificates/acme/certificate"), nil, &upid)
	return upid, err
}

func (c *Client) NodeCommand(ctx context.Context, node, command string) (string, error) {
	if command != "reboot" && command != "shutdown" {
		return "", fmt.Errorf("unsupported node command")
	}
	var upid string
	err := c.do(ctx, http.MethodPost, nodePath(node, "/status"), url.Values{"command": {command}}, &upid)
	return upid, err
}

func (c *Client) NodeReport(ctx context.Context, node string) (NodeReport, error) {
	var text string
	err := c.do(ctx, http.MethodGet, nodePath(node, "/report"), nil, &text)
	if IsNotFound(err) {
		return NodeReport{Supported: false}, nil
	}
	if err != nil {
		return NodeReport{}, err
	}
	return NodeReport{Supported: true, Report: text}, nil
}

func (c *Client) WakeOnLAN(ctx context.Context, node string) (WakeOnLANResult, error) {
	var mac string
	err := c.do(ctx, http.MethodPost, nodePath(node, "/wakeonlan"), url.Values{}, &mac)
	if IsNotFound(err) {
		return WakeOnLANResult{Supported: false}, nil
	}
	if err != nil {
		return WakeOnLANResult{}, err
	}
	return WakeOnLANResult{Supported: true, MAC: mac}, nil
}

func (c *Client) NodeConfig(ctx context.Context, node string) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodGet, nodePath(node, "/config"), nil, &out)
	if out == nil {
		out = map[string]any{}
	}
	return out, err
}

func (c *Client) UpdateNodeConfig(ctx context.Context, node string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, nodePath(node, "/config"), mapForm(fields), nil)
}

func (c *Client) InitGPT(ctx context.Context, node, disk string) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, nodePath(node, "/disks/initgpt"), url.Values{"disk": {disk}}, &upid)
	return upid, err
}

func (c *Client) ZFSStatus(ctx context.Context, node, name string) (ZFSPoolStatus, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodGet, nodePath(node, "/disks/zfs/"+url.PathEscape(name)), nil, &out)
	if IsNotFound(err) {
		return ZFSPoolStatus{Supported: false}, nil
	}
	if err != nil {
		return ZFSPoolStatus{}, err
	}
	return ZFSPoolStatus{Supported: true, Pool: out}, nil
}

func (c *Client) ZFSScrub(ctx context.Context, node, name string) (DiskRescanResult, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, nodePath(node, "/disks/zfs/"+url.PathEscape(name)), nil, &upid)
	if IsNotFound(err) {
		return DiskRescanResult{Supported: false}, nil
	}
	if err != nil {
		return DiskRescanResult{}, err
	}
	return DiskRescanResult{Supported: true, UPID: upid}, nil
}

func (c *Client) ZFSImport(ctx context.Context, node string, fields map[string]any) (DiskRescanResult, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, nodePath(node, "/disks/zfs/import"), mapForm(fields), &upid)
	if IsNotFound(err) {
		return DiskRescanResult{Supported: false}, nil
	}
	if err != nil {
		return DiskRescanResult{}, err
	}
	return DiskRescanResult{Supported: true, UPID: upid}, nil
}

func (c *Client) NodeGuestAll(ctx context.Context, node, action string, opts map[string]any) (string, error) {
	switch action {
	case "startall", "stopall", "suspendall", "migrateall":
	default:
		return "", fmt.Errorf("unsupported node guest action")
	}
	if action == "migrateall" {
		target, _ := opts["target"].(string)
		if strings.TrimSpace(target) == "" {
			return "", fmt.Errorf("target is required")
		}
	}
	if opts == nil {
		opts = map[string]any{}
	}
	var upid string
	err := c.do(ctx, http.MethodPost, nodePath(node, "/"+action), mapForm(opts), &upid)
	return upid, err
}

func (c *Client) NodeExecute(ctx context.Context, node string, commands any) ([]map[string]any, error) {
	raw, err := json.Marshal(commands)
	if err != nil {
		return nil, err
	}
	form := url.Values{"commands": {string(raw)}}
	var out []map[string]any
	err = c.do(ctx, http.MethodPost, nodePath(node, "/execute"), form, &out)
	return out, err
}

func (c *Client) APLInfo(ctx context.Context, node string) (FeatureList, error) {
	var out []map[string]any
	err := c.do(ctx, http.MethodGet, nodePath(node, "/aplinfo"), nil, &out)
	if IsNotFound(err) {
		return FeatureList{Supported: false}, nil
	}
	if err != nil {
		return FeatureList{}, err
	}
	return FeatureList{Supported: true, Items: out}, nil
}
