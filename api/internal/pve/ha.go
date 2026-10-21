package pve

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func HASid(kind string, vmid int) string {
	if kind == "lxc" || kind == "ct" {
		return fmt.Sprintf("ct:%d", vmid)
	}
	return fmt.Sprintf("vm:%d", vmid)
}

func (c *Client) HACurrent(ctx context.Context) ([]HACurrent, error) {
	var out []HACurrent
	err := c.do(ctx, http.MethodGet, "/cluster/ha/status/current", nil, &out)
	return out, err
}

func (c *Client) HAManagerStatus(ctx context.Context) (any, error) {
	var out any
	err := c.do(ctx, http.MethodGet, "/cluster/ha/status/manager_status", nil, &out)
	return out, err
}

func (c *Client) ArmHA(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/cluster/ha/status/arm-ha", nil, nil)
}

func (c *Client) DisarmHA(ctx context.Context, resourceMode string) error {
	form := url.Values{}
	if resourceMode != "" {
		form.Set("resource-mode", resourceMode)
	}
	return c.do(ctx, http.MethodPost, "/cluster/ha/status/disarm-ha", form, nil)
}

func (c *Client) HAResources(ctx context.Context) ([]HAResource, error) {
	var out []HAResource
	err := c.do(ctx, http.MethodGet, "/cluster/ha/resources", nil, &out)
	return out, err
}

func (c *Client) HAResource(ctx context.Context, sid string) (HAResource, error) {
	var out HAResource
	err := c.do(ctx, http.MethodGet, "/cluster/ha/resources/"+url.PathEscape(sid), nil, &out)
	return out, err
}

func (c *Client) CreateHAResource(ctx context.Context, req HAResourceRequest) error {
	sid := req.SID
	if sid == "" && req.VMID > 0 {
		sid = HASid(req.Type, req.VMID)
	}
	if sid == "" {
		return fmt.Errorf("sid or vmid is required")
	}
	form := haResourceForm(req)
	form.Set("sid", sid)
	return c.do(ctx, http.MethodPost, "/cluster/ha/resources", form, nil)
}

func (c *Client) UpdateHAResource(ctx context.Context, sid string, req HAResourceRequest) error {
	return c.do(ctx, http.MethodPut, "/cluster/ha/resources/"+url.PathEscape(sid), haResourceForm(req), nil)
}

func (c *Client) DeleteHAResource(ctx context.Context, sid string, purge bool) error {
	path := "/cluster/ha/resources/" + url.PathEscape(sid)
	if purge {
		path += "?purge=1"
	}
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) HAMigrate(ctx context.Context, sid, node string) (string, error) {
	return c.haMove(ctx, sid, "migrate", node)
}

func (c *Client) HARelocate(ctx context.Context, sid, node string) (string, error) {
	return c.haMove(ctx, sid, "relocate", node)
}

func (c *Client) haMove(ctx context.Context, sid, action, node string) (string, error) {
	form := url.Values{"node": {node}}
	var upid string
	err := c.do(ctx, http.MethodPost, "/cluster/ha/resources/"+url.PathEscape(sid)+"/"+action, form, &upid)
	return upid, err
}

func (c *Client) HAGroups(ctx context.Context) ([]HAGroup, error) {
	var out []HAGroup
	err := c.do(ctx, http.MethodGet, "/cluster/ha/groups", nil, &out)
	return out, err
}

func (c *Client) HAGroup(ctx context.Context, group string) (HAGroup, error) {
	var out HAGroup
	err := c.do(ctx, http.MethodGet, "/cluster/ha/groups/"+url.PathEscape(group), nil, &out)
	return out, err
}

func (c *Client) CreateHAGroup(ctx context.Context, req HAGroupRequest) error {
	form := haGroupForm(req)
	if req.Group != "" {
		form.Set("group", req.Group)
	}
	return c.do(ctx, http.MethodPost, "/cluster/ha/groups", form, nil)
}

func (c *Client) UpdateHAGroup(ctx context.Context, group string, req HAGroupRequest) error {
	return c.do(ctx, http.MethodPut, "/cluster/ha/groups/"+url.PathEscape(group), haGroupForm(req), nil)
}

func (c *Client) DeleteHAGroup(ctx context.Context, group string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/ha/groups/"+url.PathEscape(group), nil, nil)
}

func (c *Client) HARules(ctx context.Context) ([]HARule, error) {
	var out []HARule
	err := c.do(ctx, http.MethodGet, "/cluster/ha/rules", nil, &out)
	return out, err
}

func (c *Client) CreateHARule(ctx context.Context, req HARuleRequest) error {
	form := haRuleForm(req)
	if req.Rule != "" {
		form.Set("rule", req.Rule)
	}
	return c.do(ctx, http.MethodPost, "/cluster/ha/rules", form, nil)
}

func (c *Client) UpdateHARule(ctx context.Context, rule string, req HARuleRequest) error {
	return c.do(ctx, http.MethodPut, "/cluster/ha/rules/"+url.PathEscape(rule), haRuleForm(req), nil)
}

func (c *Client) DeleteHARule(ctx context.Context, rule string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/ha/rules/"+url.PathEscape(rule), nil, nil)
}

func haResourceForm(req HAResourceRequest) url.Values {
	form := url.Values{}
	if req.State != "" {
		form.Set("state", req.State)
	}
	if req.Group != "" {
		form.Set("group", req.Group)
	}
	if req.MaxRestart != nil {
		form.Set("max_restart", strconv.Itoa(*req.MaxRestart))
	}
	if req.MaxRelocate != nil {
		form.Set("max_relocate", strconv.Itoa(*req.MaxRelocate))
	}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return form
}

func haGroupForm(req HAGroupRequest) url.Values {
	form := url.Values{}
	if req.Nodes != "" {
		form.Set("nodes", req.Nodes)
	}
	form.Set("restricted", bool01(req.Restricted))
	form.Set("nofailback", bool01(req.NoFailback))
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return form
}

func haRuleForm(req HARuleRequest) url.Values {
	form := url.Values{}
	if req.Type != "" {
		form.Set("type", req.Type)
	}
	if req.Resources != "" {
		form.Set("resources", req.Resources)
	}
	if req.Nodes != "" {
		form.Set("nodes", req.Nodes)
	}
	if req.Affinity != "" {
		form.Set("affinity", req.Affinity)
	}
	form.Set("strict", bool01(req.Strict))
	form.Set("disable", bool01(req.Disable))
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return form
}

func NormalizeHASid(sid, kind string, vmid int) string {
	sid = strings.TrimSpace(sid)
	if sid != "" {
		return sid
	}
	return HASid(kind, vmid)
}

func bool01(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
