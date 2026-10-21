package pve

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (c *Client) cephNode(node, suffix string) string {
	return "/nodes/" + url.PathEscape(node) + "/ceph" + suffix
}

func isCephMissing(err error) bool {
	if err == nil {
		return false
	}
	if IsNotFound(err) {
		return true
	}
	e, ok := IsAPIError(err)
	if !ok {
		return false
	}
	msg := strings.ToLower(e.Message)
	return strings.Contains(msg, "not installed") ||
		strings.Contains(msg, "not initialized") ||
		strings.Contains(msg, "no ceph") ||
		strings.Contains(msg, "ceph configuration") ||
		strings.Contains(msg, "pveceph") && strings.Contains(msg, "not")
}

func asMap(raw any) map[string]any {
	switch v := raw.(type) {
	case map[string]any:
		return v
	case []any:
		return map[string]any{"index": v}
	case nil:
		return map[string]any{}
	default:
		return map[string]any{"value": v}
	}
}

func (c *Client) cephGet(ctx context.Context, node, clusterPath, nodeSuffix string, dest any) error {
	if clusterPath != "" {
		err := c.do(ctx, http.MethodGet, clusterPath, nil, dest)
		if err == nil {
			return nil
		}
		if !IsNotFound(err) && !isCephMissing(err) {
			return err
		}
	}
	return c.do(ctx, http.MethodGet, c.cephNode(node, nodeSuffix), nil, dest)
}

func (c *Client) CephStatus(ctx context.Context, node string) (CephStatus, error) {
	var raw any
	err := c.cephGet(ctx, node, "/cluster/ceph/status", "/status", &raw)
	if isCephMissing(err) {
		return CephStatus{Supported: false}, nil
	}
	if err != nil {
		return CephStatus{}, err
	}
	return CephStatus{Supported: true, Data: asMap(raw)}, nil
}

func (c *Client) CephMetadata(ctx context.Context, node string) (CephStatus, error) {
	var raw any
	err := c.cephGet(ctx, node, "/cluster/ceph/metadata", "/metadata", &raw)
	if isCephMissing(err) {
		return CephStatus{Supported: false}, nil
	}
	if err != nil {
		return CephStatus{}, err
	}
	return CephStatus{Supported: true, Data: asMap(raw)}, nil
}

func (c *Client) CephFlags(ctx context.Context, node string) (CephFlags, error) {
	var flags map[string]any
	err := c.cephGet(ctx, node, "/cluster/ceph/flags", "/flags", &flags)
	if isCephMissing(err) {
		return CephFlags{Supported: false}, nil
	}
	if err != nil {
		return CephFlags{}, err
	}
	return CephFlags{Supported: true, Flags: flags}, nil
}

func (c *Client) UpdateCephFlags(ctx context.Context, node string, fields map[string]any) error {
	err := c.do(ctx, http.MethodPut, c.cephNode(node, "/flags"), mapForm(fields), nil)
	if IsNotFound(err) {
		return c.do(ctx, http.MethodPut, "/cluster/ceph/flags", mapForm(fields), nil)
	}
	return err
}

func (c *Client) CephConfig(ctx context.Context, node string) (CephStatus, error) {
	var raw any
	err := c.do(ctx, http.MethodGet, c.cephNode(node, "/config"), nil, &raw)
	if isCephMissing(err) {
		return CephStatus{Supported: false}, nil
	}
	if err != nil {
		return CephStatus{}, err
	}
	return CephStatus{Supported: true, Data: asMap(raw)}, nil
}

func (c *Client) CephCrush(ctx context.Context, node string) (CephStatus, error) {
	var raw any
	err := c.do(ctx, http.MethodGet, c.cephNode(node, "/crush"), nil, &raw)
	if isCephMissing(err) {
		return CephStatus{Supported: false}, nil
	}
	if err != nil {
		return CephStatus{}, err
	}
	return CephStatus{Supported: true, Data: asMap(raw)}, nil
}

func (c *Client) UpdateCephCrush(ctx context.Context, node string, fields map[string]any) error {
	err := c.do(ctx, http.MethodPut, c.cephNode(node, "/crush"), mapForm(fields), nil)
	if isCephMissing(err) {
		return fmt.Errorf("ceph crush edit is not available on this cluster")
	}
	return err
}

func (c *Client) CephRules(ctx context.Context, node string) (FeatureList, error) {
	return c.optionalMaps(ctx, c.cephNode(node, "/rules"))
}

func (c *Client) CreateCephRule(ctx context.Context, node string, fields map[string]any) error {
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/rules"), mapForm(fields), nil)
	if isCephMissing(err) {
		return fmt.Errorf("ceph crush rules are not available on this cluster")
	}
	return err
}

func (c *Client) CephLog(ctx context.Context, node string) ([]FWLogLine, error) {
	var out []FWLogLine
	err := c.do(ctx, http.MethodGet, c.cephNode(node, "/log"), nil, &out)
	if isCephMissing(err) {
		return []FWLogLine{}, nil
	}
	return out, err
}

func (c *Client) RestartCeph(ctx context.Context, node string) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/restart"), nil, &upid)
	return upid, err
}

func (c *Client) CephReleases(ctx context.Context, node string) (FeatureList, error) {
	return c.optionalMaps(ctx, c.cephNode(node, "/releases"))
}

func (c *Client) CephRestartBulk(ctx context.Context, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, "/cluster/ceph/restart-bulk", mapForm(fields), &upid)
	if IsNotFound(err) {
		return "", fmt.Errorf("ceph restart-bulk is not available on this cluster")
	}
	return upid, err
}

func (c *Client) NodeCephRestartBulk(ctx context.Context, node string, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/restart-bulk"), mapForm(fields), &upid)
	if IsNotFound(err) {
		return "", fmt.Errorf("node ceph restart-bulk is not available on this node")
	}
	return upid, err
}

func (c *Client) InstallCeph(ctx context.Context, node string, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/install"), mapForm(fields), &upid)
	if IsNotFound(err) {
		return "", fmt.Errorf("ceph install is not available on this node; install ceph packages with apt")
	}
	return upid, err
}

func (c *Client) InitCeph(ctx context.Context, node string, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/init"), mapForm(fields), &upid)
	return upid, err
}

func (c *Client) CephMons(ctx context.Context, node string) ([]CephMon, error) {
	var out []CephMon
	err := c.do(ctx, http.MethodGet, c.cephNode(node, "/mon"), nil, &out)
	if isCephMissing(err) {
		return []CephMon{}, nil
	}
	return out, err
}

func (c *Client) CreateCephMon(ctx context.Context, node, id string, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/mon/"+url.PathEscape(id)), mapForm(fields), &upid)
	return upid, err
}

func (c *Client) DeleteCephMon(ctx context.Context, node, id string) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodDelete, c.cephNode(node, "/mon/"+url.PathEscape(id)), nil, &upid)
	return upid, err
}

func (c *Client) CephMgrs(ctx context.Context, node string) ([]CephMgr, error) {
	var out []CephMgr
	err := c.do(ctx, http.MethodGet, c.cephNode(node, "/mgr"), nil, &out)
	if isCephMissing(err) {
		return []CephMgr{}, nil
	}
	return out, err
}

func (c *Client) CreateCephMgr(ctx context.Context, node, id string, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/mgr/"+url.PathEscape(id)), mapForm(fields), &upid)
	return upid, err
}

func (c *Client) DeleteCephMgr(ctx context.Context, node, id string) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodDelete, c.cephNode(node, "/mgr/"+url.PathEscape(id)), nil, &upid)
	return upid, err
}

func (c *Client) CephMDS(ctx context.Context, node string) ([]CephMDS, error) {
	var out []CephMDS
	err := c.do(ctx, http.MethodGet, c.cephNode(node, "/mds"), nil, &out)
	if isCephMissing(err) {
		return []CephMDS{}, nil
	}
	return out, err
}

func (c *Client) CreateCephMDS(ctx context.Context, node, name string, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/mds/"+url.PathEscape(name)), mapForm(fields), &upid)
	return upid, err
}

func (c *Client) DeleteCephMDS(ctx context.Context, node, name string) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodDelete, c.cephNode(node, "/mds/"+url.PathEscape(name)), nil, &upid)
	return upid, err
}

func (c *Client) CephOSDs(ctx context.Context, node string) ([]CephOSD, error) {
	var out []CephOSD
	err := c.do(ctx, http.MethodGet, c.cephNode(node, "/osd"), nil, &out)
	if isCephMissing(err) {
		return []CephOSD{}, nil
	}
	for i := range out {
		if out[i].OSDID == 0 && out[i].ID != 0 {
			out[i].OSDID = out[i].ID
		}
	}
	return out, err
}

func (c *Client) CreateCephOSD(ctx context.Context, node string, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/osd"), mapForm(fields), &upid)
	return upid, err
}

func (c *Client) CephOSDIn(ctx context.Context, node string, osdid int) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/osd/"+strconv.Itoa(osdid)+"/in"), nil, &upid)
	return upid, err
}

func (c *Client) CephOSDOut(ctx context.Context, node string, osdid int) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/osd/"+strconv.Itoa(osdid)+"/out"), nil, &upid)
	return upid, err
}

func (c *Client) CephOSDScrub(ctx context.Context, node string, osdid int, deep bool) (string, error) {
	var upid string
	fields := map[string]any{}
	if deep {
		fields["deep"] = 1
	}
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/osd/"+strconv.Itoa(osdid)+"/scrub"), mapForm(fields), &upid)
	return upid, err
}

func (c *Client) DestroyCephOSD(ctx context.Context, node string, osdid int, cleanup bool) (string, error) {
	var upid string
	path := c.cephNode(node, "/osd/"+strconv.Itoa(osdid))
	if cleanup {
		path += "?cleanup=1"
	}
	err := c.do(ctx, http.MethodDelete, path, nil, &upid)
	return upid, err
}

func (c *Client) CephPools(ctx context.Context, node string) ([]CephPool, error) {
	var out []CephPool
	err := c.do(ctx, http.MethodGet, c.cephNode(node, "/pool"), nil, &out)
	if isCephMissing(err) {
		return []CephPool{}, nil
	}
	return out, err
}

func (c *Client) CreateCephPool(ctx context.Context, node string, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/pool"), mapForm(fields), &upid)
	return upid, err
}

func (c *Client) UpdateCephPool(ctx context.Context, node, pool string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, c.cephNode(node, "/pool/"+url.PathEscape(pool)), mapForm(fields), nil)
}

func (c *Client) DeleteCephPool(ctx context.Context, node, pool string) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodDelete, c.cephNode(node, "/pool/"+url.PathEscape(pool)), nil, &upid)
	return upid, err
}

func (c *Client) CephFSList(ctx context.Context, node string) (FeatureList, error) {
	return c.optionalMaps(ctx, c.cephNode(node, "/fs"))
}

func (c *Client) CreateCephFS(ctx context.Context, node, name string, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/fs/"+url.PathEscape(name)), mapForm(fields), &upid)
	if IsNotFound(err) {
		return "", fmt.Errorf("cephfs is not available on this cluster")
	}
	return upid, err
}

func (c *Client) CephRGW(ctx context.Context, node string) (FeatureList, error) {
	return c.optionalMaps(ctx, c.cephNode(node, "/radosgw"))
}

func (c *Client) CreateCephRGW(ctx context.Context, node, id string, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/radosgw/"+url.PathEscape(id)), mapForm(fields), &upid)
	if IsNotFound(err) {
		return "", fmt.Errorf("ceph rgw is not available on this cluster")
	}
	return upid, err
}

func (c *Client) DeleteCephRGW(ctx context.Context, node, id string) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodDelete, c.cephNode(node, "/radosgw/"+url.PathEscape(id)), nil, &upid)
	return upid, err
}

func (c *Client) CephRGWUsers(ctx context.Context, node string) (FeatureList, error) {
	return c.optionalMaps(ctx, c.cephNode(node, "/radosgw/users"))
}

func (c *Client) CreateCephRGWUser(ctx context.Context, node string, fields map[string]any) error {
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/radosgw/users"), mapForm(fields), nil)
	if isCephMissing(err) {
		return fmt.Errorf("ceph rgw users are not available on this cluster")
	}
	return err
}

func (c *Client) DeleteCephRGWUser(ctx context.Context, node, uid string) error {
	err := c.do(ctx, http.MethodDelete, c.cephNode(node, "/radosgw/users/"+url.PathEscape(uid)), nil, nil)
	if isCephMissing(err) {
		return fmt.Errorf("ceph rgw users are not available on this cluster")
	}
	return err
}

func (c *Client) StartCeph(ctx context.Context, node, service string) (string, error) {
	fields := map[string]any{}
	if service != "" {
		fields["service"] = service
	}
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/start"), mapForm(fields), &upid)
	return upid, err
}

func (c *Client) StopCeph(ctx context.Context, node, service string) (string, error) {
	fields := map[string]any{}
	if service != "" {
		fields["service"] = service
	}
	var upid string
	err := c.do(ctx, http.MethodPost, c.cephNode(node, "/stop"), mapForm(fields), &upid)
	return upid, err
}

func (c *Client) CephCmdSafety(ctx context.Context, node, service, id, action string) (CephCmdSafety, error) {
	q := url.Values{}
	q.Set("service", service)
	q.Set("id", id)
	q.Set("action", action)
	var raw map[string]any
	err := c.do(ctx, http.MethodGet, c.cephNode(node, "/cmd-safety?"+q.Encode()), nil, &raw)
	if IsNotFound(err) || isCephMissing(err) {
		return CephCmdSafety{Supported: false}, nil
	}
	if err != nil {
		return CephCmdSafety{}, err
	}
	safe := false
	switch v := raw["safe"].(type) {
	case bool:
		safe = v
	case float64:
		safe = v != 0
	case int:
		safe = v != 0
	}
	status, _ := raw["status"].(string)
	return CephCmdSafety{Supported: true, Safe: safe, Status: status}, nil
}

func (c *Client) CephOSDMetadata(ctx context.Context, node string, osdid int) (FeatureObject, error) {
	return c.optionalObject(ctx, c.cephNode(node, "/osd/"+strconv.Itoa(osdid)+"/metadata"))
}

func (c *Client) CephOSDLVInfo(ctx context.Context, node string, osdid int) (FeatureObject, error) {
	return c.optionalObject(ctx, c.cephNode(node, "/osd/"+strconv.Itoa(osdid)+"/lv-info"))
}

func (c *Client) CephPoolStatus(ctx context.Context, node, pool string) (FeatureObject, error) {
	return c.optionalObject(ctx, c.cephNode(node, "/pool/"+url.PathEscape(pool)+"/status"))
}
