package pve

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) ListUsers(ctx context.Context) ([]AccessUser, error) {
	var out []AccessUser
	err := c.do(ctx, http.MethodGet, "/access/users", nil, &out)
	return out, err
}

func (c *Client) GetUser(ctx context.Context, userid string) (AccessUser, error) {
	var out AccessUser
	err := c.do(ctx, http.MethodGet, "/access/users/"+url.PathEscape(userid), nil, &out)
	if out.UserID == "" {
		out.UserID = userid
	}
	return out, err
}

func (c *Client) CreateUser(ctx context.Context, req AccessUserRequest) error {
	form := userForm(req)
	if req.UserID != "" {
		form.Set("userid", req.UserID)
	}
	return c.do(ctx, http.MethodPost, "/access/users", form, nil)
}

func (c *Client) UpdateUser(ctx context.Context, userid string, req AccessUserRequest) error {
	return c.do(ctx, http.MethodPut, "/access/users/"+url.PathEscape(userid), userForm(req), nil)
}

func (c *Client) DeleteUser(ctx context.Context, userid string) error {
	return c.do(ctx, http.MethodDelete, "/access/users/"+url.PathEscape(userid), nil, nil)
}

func (c *Client) SetUserPassword(ctx context.Context, userid, password string) error {
	form := url.Values{"userid": {userid}, "password": {password}}
	return c.do(ctx, http.MethodPut, "/access/password", form, nil)
}

func userForm(req AccessUserRequest) url.Values {
	form := url.Values{}
	if req.Password != "" {
		form.Set("password", req.Password)
	}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	if req.Email != "" {
		form.Set("email", req.Email)
	}
	if req.Enable != nil {
		form.Set("enable", fmt.Sprintf("%d", *req.Enable))
	}
	if req.Expire != nil {
		form.Set("expire", fmt.Sprintf("%d", *req.Expire))
	}
	if req.Firstname != "" {
		form.Set("firstname", req.Firstname)
	}
	if req.Lastname != "" {
		form.Set("lastname", req.Lastname)
	}
	if req.Groups != "" {
		form.Set("groups", req.Groups)
	}
	return form
}

func (c *Client) ListTokens(ctx context.Context, userid string) ([]AccessToken, error) {
	var out []AccessToken
	err := c.do(ctx, http.MethodGet, "/access/users/"+url.PathEscape(userid)+"/token", nil, &out)
	return out, err
}

func (c *Client) CreateToken(ctx context.Context, userid string, req AccessTokenCreate) (AccessTokenCreate, error) {
	form := url.Values{}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	if req.Expire > 0 {
		form.Set("expire", fmt.Sprintf("%d", req.Expire))
	}
	if req.Privsep != nil {
		form.Set("privsep", fmt.Sprintf("%d", *req.Privsep))
	}
	var out AccessTokenCreate
	err := c.do(ctx, http.MethodPost, "/access/users/"+url.PathEscape(userid)+"/token/"+url.PathEscape(req.TokenID), form, &out)
	if out.TokenID == "" {
		out.TokenID = req.TokenID
	}
	return out, err
}

func (c *Client) UpdateToken(ctx context.Context, userid, tokenid string, req AccessTokenCreate) error {
	form := url.Values{}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	if req.Expire > 0 {
		form.Set("expire", fmt.Sprintf("%d", req.Expire))
	}
	if req.Privsep != nil {
		form.Set("privsep", fmt.Sprintf("%d", *req.Privsep))
	}
	return c.do(ctx, http.MethodPut, "/access/users/"+url.PathEscape(userid)+"/token/"+url.PathEscape(tokenid), form, nil)
}

func (c *Client) DeleteToken(ctx context.Context, userid, tokenid string) error {
	return c.do(ctx, http.MethodDelete, "/access/users/"+url.PathEscape(userid)+"/token/"+url.PathEscape(tokenid), nil, nil)
}

func (c *Client) ListTFA(ctx context.Context, userid string) ([]TFAEntry, error) {
	var out []TFAEntry
	err := c.do(ctx, http.MethodGet, "/access/tfa/"+url.PathEscape(userid), nil, &out)
	return out, err
}

func (c *Client) EnrollTFA(ctx context.Context, userid string, req TFAEnrollRequest) (TFAEnrollResponse, error) {
	form := url.Values{"type": {req.Type}}
	if req.Description != "" {
		form.Set("description", req.Description)
	}
	if req.Password != "" {
		form.Set("password", req.Password)
	}
	var out TFAEnrollResponse
	err := c.do(ctx, http.MethodPost, "/access/tfa/"+url.PathEscape(userid), form, &out)
	return out, err
}

func (c *Client) DeleteTFA(ctx context.Context, userid, id string) error {
	return c.do(ctx, http.MethodDelete, "/access/tfa/"+url.PathEscape(userid)+"/"+url.PathEscape(id), nil, nil)
}

func (c *Client) UnlockTFA(ctx context.Context, userid string) error {
	return c.do(ctx, http.MethodPut, "/access/users/"+url.PathEscape(userid)+"/unlock-tfa", nil, nil)
}

func (c *Client) ListGroups(ctx context.Context) ([]AccessGroup, error) {
	var out []AccessGroup
	err := c.do(ctx, http.MethodGet, "/access/groups", nil, &out)
	return out, err
}

func (c *Client) GetGroup(ctx context.Context, group string) (AccessGroup, error) {
	var out AccessGroup
	err := c.do(ctx, http.MethodGet, "/access/groups/"+url.PathEscape(group), nil, &out)
	if out.Group == "" {
		out.Group = group
	}
	return out, err
}

func (c *Client) CreateGroup(ctx context.Context, group, comment string) error {
	form := url.Values{"group": {group}}
	if comment != "" {
		form.Set("comment", comment)
	}
	return c.do(ctx, http.MethodPost, "/access/groups", form, nil)
}

func (c *Client) UpdateGroup(ctx context.Context, group, comment string) error {
	form := url.Values{}
	if comment != "" {
		form.Set("comment", comment)
	}
	return c.do(ctx, http.MethodPut, "/access/groups/"+url.PathEscape(group), form, nil)
}

func (c *Client) DeleteGroup(ctx context.Context, group string) error {
	return c.do(ctx, http.MethodDelete, "/access/groups/"+url.PathEscape(group), nil, nil)
}

func (c *Client) ListRoles(ctx context.Context) ([]AccessRole, error) {
	var out []AccessRole
	err := c.do(ctx, http.MethodGet, "/access/roles", nil, &out)
	return out, err
}

func (c *Client) CreateRole(ctx context.Context, role, privs string) error {
	form := url.Values{"roleid": {role}}
	if privs != "" {
		form.Set("privs", privs)
	}
	return c.do(ctx, http.MethodPost, "/access/roles", form, nil)
}

func (c *Client) UpdateRole(ctx context.Context, role, privs string) error {
	form := url.Values{}
	if privs != "" {
		form.Set("privs", privs)
	}
	return c.do(ctx, http.MethodPut, "/access/roles/"+url.PathEscape(role), form, nil)
}

func (c *Client) DeleteRole(ctx context.Context, role string) error {
	return c.do(ctx, http.MethodDelete, "/access/roles/"+url.PathEscape(role), nil, nil)
}

func (c *Client) ListACL(ctx context.Context) ([]ACLEntry, error) {
	var out []ACLEntry
	err := c.do(ctx, http.MethodGet, "/access/acl", nil, &out)
	return out, err
}

func (c *Client) UpdateACL(ctx context.Context, req ACLRequest) error {
	form := url.Values{"path": {req.Path}, "roles": {req.Roles}}
	if req.Users != "" {
		form.Set("users", req.Users)
	}
	if req.Groups != "" {
		form.Set("groups", req.Groups)
	}
	if req.Propagate != nil {
		form.Set("propagate", fmt.Sprintf("%d", *req.Propagate))
	}
	if req.Delete {
		form.Set("delete", "1")
	}
	return c.do(ctx, http.MethodPut, "/access/acl", form, nil)
}

func (c *Client) ListDomains(ctx context.Context) ([]Realm, error) {
	var out []Realm
	err := c.do(ctx, http.MethodGet, "/access/domains", nil, &out)
	return out, err
}

func (c *Client) GetDomain(ctx context.Context, realm string) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodGet, "/access/domains/"+url.PathEscape(realm), nil, &out)
	return out, err
}

func (c *Client) CreateDomain(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/access/domains", mapForm(fields), nil)
}

func (c *Client) UpdateDomain(ctx context.Context, realm string, fields map[string]any) error {
	delete(fields, "realm")
	delete(fields, "type")
	return c.do(ctx, http.MethodPut, "/access/domains/"+url.PathEscape(realm), mapForm(fields), nil)
}

func (c *Client) DeleteDomain(ctx context.Context, realm string) error {
	return c.do(ctx, http.MethodDelete, "/access/domains/"+url.PathEscape(realm), nil, nil)
}

func (c *Client) SyncDomain(ctx context.Context, realm string, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, "/access/domains/"+url.PathEscape(realm)+"/sync", mapForm(fields), &upid)
	return upid, err
}

func mapForm(fields map[string]any) url.Values {
	form := url.Values{}
	for k, v := range fields {
		if v == nil {
			continue
		}
		switch t := v.(type) {
		case []string:
			for _, s := range t {
				if s = strings.TrimSpace(s); s != "" {
					form.Add(k, s)
				}
			}
		case []any:
			for _, item := range t {
				s := strings.TrimSpace(fmt.Sprint(item))
				if s != "" && s != "<nil>" {
					form.Add(k, s)
				}
			}
		default:
			s := strings.TrimSpace(fmt.Sprint(v))
			if s == "" || s == "<nil>" {
				continue
			}
			form.Set(k, s)
		}
	}
	return form
}
