package pve

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// ClusterConfig returns the corosync totem section. GET /cluster/config is
// only a directory index; the cluster name and link settings live under totem.
// A standalone node has no corosync.conf and reports clustered=false.
func (c *Client) ClusterConfig(ctx context.Context) (map[string]any, error) {
	var totem map[string]any
	err := c.do(ctx, http.MethodGet, "/cluster/config/totem", nil, &totem)
	if IsNotFound(err) {
		return map[string]any{"supported": false}, nil
	}
	if err != nil {
		if apiErr, ok := IsAPIError(err); ok && apiErr.Status < 500 {
			return nil, err
		}
		return map[string]any{"supported": true, "clustered": false}, nil
	}
	out := map[string]any{"supported": true, "clustered": len(totem) > 0}
	if len(totem) > 0 {
		out["totem"] = totem
		if name, ok := totem["cluster_name"]; ok {
			out["cluster_name"] = name
		}
	}
	return out, nil
}

func (c *Client) CreateCluster(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/config", mapForm(fields), nil)
}

func (c *Client) ClusterJoinInfo(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodGet, "/cluster/config/join", nil, &out)
	if IsNotFound(err) {
		return map[string]any{"supported": false}, nil
	}
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	out["supported"] = true
	return out, nil
}

func (c *Client) JoinCluster(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/config/join", mapForm(fields), nil)
}

func (c *Client) ClusterConfigNodes(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/config/nodes")
}

func (c *Client) AddClusterNode(ctx context.Context, node string, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/config/nodes/"+url.PathEscape(node), mapForm(fields), nil)
}

func (c *Client) DeleteClusterNode(ctx context.Context, node string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/config/nodes/"+url.PathEscape(node), nil, nil)
}

func (c *Client) ClusterQDevice(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/config/qdevice")
}

func (c *Client) CreateClusterQDevice(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/config/qdevice", mapForm(fields), nil)
}

func (c *Client) DeleteClusterQDevice(ctx context.Context) error {
	return c.do(ctx, http.MethodDelete, "/cluster/config/qdevice", nil, nil)
}

func (c *Client) UpdateClusterTotem(ctx context.Context, fields map[string]any) error {
	err := c.do(ctx, http.MethodPut, "/cluster/config/totem", mapForm(fields), nil)
	if IsNotFound(err) {
		return fmt.Errorf("cluster totem edit is not available on this cluster")
	}
	return err
}
