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

func (c *Client) ClusterOptions(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodGet, "/cluster/options", nil, &out)
	return out, err
}

func (c *Client) UpdateClusterOptions(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/options", mapForm(fields), nil)
}

func (c *Client) Pools(ctx context.Context) ([]Pool, error) {
	var out []Pool
	err := c.do(ctx, http.MethodGet, "/pools", nil, &out)
	return out, err
}

func (c *Client) Pool(ctx context.Context, poolid string) (Pool, error) {
	var out Pool
	err := c.do(ctx, http.MethodGet, "/pools/"+url.PathEscape(poolid), nil, &out)
	if err != nil {
		return out, err
	}
	if out.PoolID == "" {
		out.PoolID = poolid
	}
	return out, nil
}

func (c *Client) CreatePool(ctx context.Context, poolid, comment string) error {
	form := url.Values{"poolid": {poolid}}
	if comment != "" {
		form.Set("comment", comment)
	}
	return c.do(ctx, http.MethodPost, "/pools", form, nil)
}

func (c *Client) UpdatePool(ctx context.Context, poolid string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/pools/"+url.PathEscape(poolid), mapForm(fields), nil)
}

func (c *Client) DeletePool(ctx context.Context, poolid string) error {
	return c.do(ctx, http.MethodDelete, "/pools/"+url.PathEscape(poolid), nil, nil)
}

func (c *Client) ACMEAccounts(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/acme/account")
}

func (c *Client) CreateACMEAccount(ctx context.Context, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, "/cluster/acme/account", mapForm(fields), &upid)
	return upid, err
}

func (c *Client) UpdateACMEAccount(ctx context.Context, name string, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPut, "/cluster/acme/account/"+url.PathEscape(name), mapForm(fields), &upid)
	return upid, err
}

func (c *Client) DeleteACMEAccount(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/acme/account/"+url.PathEscape(name), nil, nil)
}

func (c *Client) ACMEPlugins(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/acme/plugins")
}

func (c *Client) CreateACMEPlugin(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/acme/plugins", mapForm(fields), nil)
}

func (c *Client) UpdateACMEPlugin(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/acme/plugins/"+url.PathEscape(id), mapForm(fields), nil)
}

func (c *Client) DeleteACMEPlugin(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/acme/plugins/"+url.PathEscape(id), nil, nil)
}

func (c *Client) NotificationEndpoints(ctx context.Context, kind string) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/notifications/endpoints/"+url.PathEscape(kind))
}

func (c *Client) CreateNotificationEndpoint(ctx context.Context, kind string, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/notifications/endpoints/"+url.PathEscape(kind), mapForm(fields), nil)
}

func (c *Client) UpdateNotificationEndpoint(ctx context.Context, kind, name string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/notifications/endpoints/"+url.PathEscape(kind)+"/"+url.PathEscape(name), mapForm(fields), nil)
}

func (c *Client) DeleteNotificationEndpoint(ctx context.Context, kind, name string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/notifications/endpoints/"+url.PathEscape(kind)+"/"+url.PathEscape(name), nil, nil)
}

func (c *Client) NotificationMatchers(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/notifications/matchers")
}

func (c *Client) CreateNotificationMatcher(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/notifications/matchers", mapForm(fields), nil)
}

func (c *Client) UpdateNotificationMatcher(ctx context.Context, name string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/notifications/matchers/"+url.PathEscape(name), mapForm(fields), nil)
}

func (c *Client) DeleteNotificationMatcher(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/notifications/matchers/"+url.PathEscape(name), nil, nil)
}

func (c *Client) NotificationTargets(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/notifications/targets")
}

func (c *Client) Mappings(ctx context.Context, kind string) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/mapping/"+url.PathEscape(kind))
}

func (c *Client) CreateMapping(ctx context.Context, kind string, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/mapping/"+url.PathEscape(kind), mapForm(fields), nil)
}

func (c *Client) UpdateMapping(ctx context.Context, kind, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/mapping/"+url.PathEscape(kind)+"/"+url.PathEscape(id), mapForm(fields), nil)
}

func (c *Client) DeleteMapping(ctx context.Context, kind, id string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/mapping/"+url.PathEscape(kind)+"/"+url.PathEscape(id), nil, nil)
}

func (c *Client) optionalMaps(ctx context.Context, path string) (FeatureList, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, path, nil, &raw)
	if IsUnavailable(err) {
		return FeatureList{Supported: false}, nil
	}
	if err != nil {
		return FeatureList{}, err
	}
	out, err := decodeFlexibleMaps(raw)
	if err != nil {
		return FeatureList{}, fmt.Errorf("decode data: %w", err)
	}
	return FeatureList{Supported: true, Items: out}, nil
}

func (c *Client) optionalObject(ctx context.Context, path string) (FeatureObject, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, path, nil, &raw)
	if IsUnavailable(err) {
		return FeatureObject{Supported: false}, nil
	}
	if err != nil {
		return FeatureObject{}, err
	}
	item := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if raw[0] == '"' {
			var s string
			if err := json.Unmarshal(raw, &s); err != nil {
				return FeatureObject{}, err
			}
			item["value"] = s
		} else if raw[0] == '[' {
			// Some endpoints answer with a list (IPAM status, OpenID index);
			// keep the object shape and put the rows under "items".
			var items []any
			if err := json.Unmarshal(raw, &items); err != nil {
				return FeatureObject{}, err
			}
			item["items"] = items
		} else if err := json.Unmarshal(raw, &item); err != nil {
			return FeatureObject{}, err
		}
	}
	return FeatureObject{Supported: true, Item: item}, nil
}

func (c *Client) optionalValue(ctx context.Context, path string) (FeatureValue, error) {
	obj, err := c.optionalObject(ctx, path)
	if err != nil || !obj.Supported {
		return FeatureValue{Supported: obj.Supported}, err
	}
	if v, ok := obj.Item["value"].(string); ok {
		return FeatureValue{Supported: true, Value: v}, nil
	}
	if v, ok := obj.Item["url"].(string); ok {
		return FeatureValue{Supported: true, Value: v}, nil
	}
	b, _ := json.Marshal(obj.Item)
	return FeatureValue{Supported: true, Value: string(b)}, nil
}

func (c *Client) MetricServers(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/metrics/server")
}

func (c *Client) CreateMetricServer(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/metrics/server", mapForm(fields), nil)
}

func (c *Client) UpdateMetricServer(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/metrics/server/"+url.PathEscape(id), mapForm(fields), nil)
}

func (c *Client) DeleteMetricServer(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/metrics/server/"+url.PathEscape(id), nil, nil)
}

func (c *Client) ClusterLog(ctx context.Context, max int) (FeatureList, error) {
	path := "/cluster/log"
	if max > 0 {
		path += "?max=" + strconv.Itoa(max)
	}
	return c.optionalMaps(ctx, path)
}

func (c *Client) ClusterJobs(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/jobs")
}

func (c *Client) RealmSyncJobs(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/jobs/realm-sync")
}

func (c *Client) CreateRealmSyncJob(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/jobs/realm-sync", mapForm(fields), nil)
}

func (c *Client) UpdateRealmSyncJob(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/jobs/realm-sync/"+url.PathEscape(id), mapForm(fields), nil)
}

func (c *Client) DeleteRealmSyncJob(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/jobs/realm-sync/"+url.PathEscape(id), nil, nil)
}

func (c *Client) ScheduleAnalyze(ctx context.Context, schedule string) (FeatureList, error) {
	path := "/cluster/jobs/schedule-analyze"
	if strings.TrimSpace(schedule) != "" {
		path += "?schedule=" + url.QueryEscape(schedule)
	}
	return c.optionalMaps(ctx, path)
}

func (c *Client) ACMEDirectories(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/acme/directories")
}

func (c *Client) ACMEMeta(ctx context.Context, directory string) (FeatureObject, error) {
	path := "/cluster/acme/meta"
	if directory != "" {
		path += "?directory=" + url.QueryEscape(directory)
	}
	return c.optionalObject(ctx, path)
}

func (c *Client) ACMETOS(ctx context.Context, directory string) (FeatureValue, error) {
	path := "/cluster/acme/tos"
	if directory != "" {
		path += "?directory=" + url.QueryEscape(directory)
	}
	return c.optionalValue(ctx, path)
}

// ACMEChallengeSchema returns the schema of one DNS plugin API. Proxmox lists
// every plugin's schema in one array ({id, name, type, schema}), so the entry
// for plugin is picked out here; with no plugin the full list is under "items".
func (c *Client) ACMEChallengeSchema(ctx context.Context, plugin string) (FeatureObject, error) {
	obj, err := c.optionalObject(ctx, "/cluster/acme/challenge-schema")
	if err != nil || !obj.Supported || plugin == "" {
		return obj, err
	}
	items, _ := obj.Item["items"].([]any)
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := m["id"].(string); id == plugin {
			return FeatureObject{Supported: true, Item: m}, nil
		}
	}
	return FeatureObject{Supported: true, Item: map[string]any{}}, nil
}

func (c *Client) TestNotificationTarget(ctx context.Context, name string) (DiskRescanResult, error) {
	err := c.do(ctx, http.MethodPost, "/cluster/notifications/targets/"+url.PathEscape(name)+"/test", nil, nil)
	if IsNotFound(err) {
		return DiskRescanResult{Supported: false}, nil
	}
	if err != nil {
		return DiskRescanResult{}, err
	}
	return DiskRescanResult{Supported: true}, nil
}

func (c *Client) CustomCPUModels(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/qemu/custom-cpu-models")
}

func (c *Client) CreateCustomCPUModel(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/qemu/custom-cpu-models", mapForm(fields), nil)
}

func (c *Client) UpdateCustomCPUModel(ctx context.Context, name string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/qemu/custom-cpu-models/"+url.PathEscape(name), mapForm(fields), nil)
}

func (c *Client) DeleteCustomCPUModel(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/qemu/custom-cpu-models/"+url.PathEscape(name), nil, nil)
}

func (c *Client) OpenID(ctx context.Context) (FeatureObject, error) {
	return c.optionalObject(ctx, "/access/openid")
}
