package pve

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type FWScope struct {
	Level string
	Node  string
	Kind  string
	VMID  int
	VNet  string
}

func (s FWScope) base(c *Client) string {
	switch s.Level {
	case "node":
		return "/nodes/" + url.PathEscape(s.Node) + "/firewall"
	case "guest":
		return c.guestBase(s.Node, s.Kind, s.VMID) + "/firewall"
	case "vnet":
		return "/cluster/sdn/vnets/" + url.PathEscape(s.VNet) + "/firewall"
	default:
		return "/cluster/firewall"
	}
}

func (c *Client) FWRules(ctx context.Context, scope FWScope) ([]FWRule, error) {
	var out []FWRule
	err := c.do(ctx, http.MethodGet, scope.base(c)+"/rules", nil, &out)
	return out, err
}

func (c *Client) CreateFWRule(ctx context.Context, scope FWScope, req FWRuleRequest) error {
	return c.do(ctx, http.MethodPost, scope.base(c)+"/rules", fwRuleForm(req), nil)
}

func (c *Client) UpdateFWRule(ctx context.Context, scope FWScope, pos int, req FWRuleRequest) error {
	return c.do(ctx, http.MethodPut, scope.base(c)+"/rules/"+strconv.Itoa(pos), fwRuleForm(req), nil)
}

func (c *Client) DeleteFWRule(ctx context.Context, scope FWScope, pos int, digest string) error {
	path := scope.base(c) + "/rules/" + strconv.Itoa(pos)
	if digest != "" {
		path += "?digest=" + url.QueryEscape(digest)
	}
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) FWOptions(ctx context.Context, scope FWScope) (FWOptions, error) {
	var out FWOptions
	err := c.do(ctx, http.MethodGet, scope.base(c)+"/options", nil, &out)
	return out, err
}

func (c *Client) UpdateFWOptions(ctx context.Context, scope FWScope, req FWOptions) error {
	return c.do(ctx, http.MethodPut, scope.base(c)+"/options", fwOptionsForm(req), nil)
}

func (c *Client) FWLog(ctx context.Context, scope FWScope, start int) ([]FWLogLine, error) {
	path := scope.base(c) + "/log"
	if start > 0 {
		path += "?start=" + strconv.Itoa(start)
	}
	var out []FWLogLine
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

func (c *Client) FWAliases(ctx context.Context, scope FWScope) ([]FWAlias, error) {
	var out []FWAlias
	err := c.do(ctx, http.MethodGet, scope.base(c)+"/aliases", nil, &out)
	return out, err
}

func (c *Client) CreateFWAlias(ctx context.Context, scope FWScope, req FWAlias) error {
	form := url.Values{"name": {req.Name}, "cidr": {req.CIDR}}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return c.do(ctx, http.MethodPost, scope.base(c)+"/aliases", form, nil)
}

func (c *Client) UpdateFWAlias(ctx context.Context, scope FWScope, name string, req FWAlias) error {
	form := url.Values{}
	if req.CIDR != "" {
		form.Set("cidr", req.CIDR)
	}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	if req.Name != "" && req.Name != name {
		form.Set("rename", req.Name)
	}
	if req.Digest != "" {
		form.Set("digest", req.Digest)
	}
	return c.do(ctx, http.MethodPut, scope.base(c)+"/aliases/"+url.PathEscape(name), form, nil)
}

func (c *Client) DeleteFWAlias(ctx context.Context, scope FWScope, name, digest string) error {
	path := scope.base(c) + "/aliases/" + url.PathEscape(name)
	if digest != "" {
		path += "?digest=" + url.QueryEscape(digest)
	}
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) FWIPSets(ctx context.Context, scope FWScope) ([]FWIPSet, error) {
	var out []FWIPSet
	err := c.do(ctx, http.MethodGet, scope.base(c)+"/ipset", nil, &out)
	return out, err
}

func (c *Client) CreateFWIPSet(ctx context.Context, scope FWScope, req FWIPSet) error {
	form := url.Values{"name": {req.Name}}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return c.do(ctx, http.MethodPost, scope.base(c)+"/ipset", form, nil)
}

func (c *Client) DeleteFWIPSet(ctx context.Context, scope FWScope, name string) error {
	return c.do(ctx, http.MethodDelete, scope.base(c)+"/ipset/"+url.PathEscape(name), nil, nil)
}

func (c *Client) FWIPSetCIDRs(ctx context.Context, scope FWScope, name string) ([]FWIPSetCIDR, error) {
	var out []FWIPSetCIDR
	err := c.do(ctx, http.MethodGet, scope.base(c)+"/ipset/"+url.PathEscape(name), nil, &out)
	return out, err
}

func (c *Client) CreateFWIPSetCIDR(ctx context.Context, scope FWScope, name string, req FWIPSetCIDR) error {
	form := url.Values{"cidr": {req.CIDR}}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	if req.Nomatch != 0 {
		form.Set("nomatch", strconv.Itoa(req.Nomatch))
	}
	return c.do(ctx, http.MethodPost, scope.base(c)+"/ipset/"+url.PathEscape(name), form, nil)
}

func (c *Client) DeleteFWIPSetCIDR(ctx context.Context, scope FWScope, name, cidr, digest string) error {
	path := scope.base(c) + "/ipset/" + url.PathEscape(name) + "/" + encodeCIDR(cidr)
	if digest != "" {
		path += "?digest=" + url.QueryEscape(digest)
	}
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) FWGroups(ctx context.Context) ([]FWGroup, error) {
	var out []FWGroup
	err := c.do(ctx, http.MethodGet, "/cluster/firewall/groups", nil, &out)
	return out, err
}

func (c *Client) CreateFWGroup(ctx context.Context, req FWGroup) error {
	form := url.Values{"group": {req.Group}}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return c.do(ctx, http.MethodPost, "/cluster/firewall/groups", form, nil)
}

func (c *Client) DeleteFWGroup(ctx context.Context, group string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/firewall/groups/"+url.PathEscape(group), nil, nil)
}

func (c *Client) FWGroupRules(ctx context.Context, group string) ([]FWRule, error) {
	var out []FWRule
	err := c.do(ctx, http.MethodGet, "/cluster/firewall/groups/"+url.PathEscape(group), nil, &out)
	return out, err
}

func (c *Client) CreateFWGroupRule(ctx context.Context, group string, req FWRuleRequest) error {
	return c.do(ctx, http.MethodPost, "/cluster/firewall/groups/"+url.PathEscape(group), fwRuleForm(req), nil)
}

func (c *Client) UpdateFWGroupRule(ctx context.Context, group string, pos int, req FWRuleRequest) error {
	return c.do(ctx, http.MethodPut, "/cluster/firewall/groups/"+url.PathEscape(group)+"/"+strconv.Itoa(pos), fwRuleForm(req), nil)
}

func (c *Client) DeleteFWGroupRule(ctx context.Context, group string, pos int, digest string) error {
	path := "/cluster/firewall/groups/" + url.PathEscape(group) + "/" + strconv.Itoa(pos)
	if digest != "" {
		path += "?digest=" + url.QueryEscape(digest)
	}
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) FWMacros(ctx context.Context) ([]FWMacro, error) {
	var out []FWMacro
	err := c.do(ctx, http.MethodGet, "/cluster/firewall/macros", nil, &out)
	return out, err
}

func (c *Client) FWRefs(ctx context.Context, scope FWScope) (FWRefsResponse, error) {
	var out []FWRef
	err := c.do(ctx, http.MethodGet, scope.base(c)+"/refs", nil, &out)
	if IsNotFound(err) {
		return FWRefsResponse{Supported: false}, nil
	}
	if err != nil {
		return FWRefsResponse{}, err
	}
	return FWRefsResponse{Supported: true, Refs: out}, nil
}

func fwRuleForm(req FWRuleRequest) url.Values {
	form := url.Values{}
	if req.Action != "" {
		form.Set("action", req.Action)
	}
	if req.Type != "" {
		form.Set("type", req.Type)
	}
	if req.Enable != nil {
		form.Set("enable", strconv.Itoa(*req.Enable))
	}
	if req.Source != "" {
		form.Set("source", req.Source)
	}
	if req.Dest != "" {
		form.Set("dest", req.Dest)
	}
	if req.Proto != "" {
		form.Set("proto", req.Proto)
	}
	if req.DPort != "" {
		form.Set("dport", req.DPort)
	}
	if req.SPort != "" {
		form.Set("sport", req.SPort)
	}
	if req.Macro != "" {
		form.Set("macro", req.Macro)
	}
	if req.Iface != "" {
		form.Set("iface", req.Iface)
	}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	if req.Log != "" {
		form.Set("log", req.Log)
	}
	if req.Pos != nil {
		form.Set("pos", strconv.Itoa(*req.Pos))
	}
	if req.Digest != "" {
		form.Set("digest", req.Digest)
	}
	return form
}

func fwOptionsForm(req FWOptions) url.Values {
	form := url.Values{}
	set01(form, "enable", req.Enable)
	if req.PolicyIn != "" {
		form.Set("policy_in", req.PolicyIn)
	}
	if req.PolicyOut != "" {
		form.Set("policy_out", req.PolicyOut)
	}
	if req.LogLevelIn != "" {
		form.Set("log_level_in", req.LogLevelIn)
	}
	if req.LogLevelOut != "" {
		form.Set("log_level_out", req.LogLevelOut)
	}
	set01(form, "ebtables", req.Ebtables)
	set01(form, "ndp", req.NDP)
	set01(form, "dhcp", req.DHCP)
	set01(form, "radv", req.RADV)
	set01(form, "macfilter", req.MacFilter)
	set01(form, "ipfilter", req.IPFilter)
	if req.Digest != "" {
		form.Set("digest", req.Digest)
	}
	return form
}

func set01(form url.Values, key string, v *int) {
	if v != nil {
		form.Set(key, strconv.Itoa(*v))
	}
}

func encodeCIDR(cidr string) string {
	return url.PathEscape(cidr)
}

func ParseFWPos(raw string) (int, error) {
	pos, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid pos")
	}
	return pos, nil
}
