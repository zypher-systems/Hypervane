package pve

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func (c *Client) SDNStatus(ctx context.Context) (SDNStatus, error) {
	var raw any
	err := c.do(ctx, http.MethodGet, "/cluster/sdn", nil, &raw)
	if IsNotFound(err) {
		return SDNStatus{Supported: false}, nil
	}
	if err != nil {
		return SDNStatus{}, err
	}
	switch v := raw.(type) {
	case map[string]any:
		return SDNStatus{Supported: true, Data: v}, nil
	case []any:
		return SDNStatus{Supported: true, Data: map[string]any{"index": v}}, nil
	default:
		return SDNStatus{Supported: true, Data: map[string]any{"value": v}}, nil
	}
}

func (c *Client) ApplySDN(ctx context.Context, fields map[string]any) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPut, "/cluster/sdn", mapForm(fields), &upid)
	return upid, err
}

func (c *Client) RollbackSDN(ctx context.Context, fields map[string]any) error {
	err := c.do(ctx, http.MethodDelete, "/cluster/sdn"+sdnQuery(fields), nil, nil)
	if err == nil {
		return nil
	}
	if IsNotFound(err) {
		err = c.do(ctx, http.MethodPost, "/cluster/sdn/rollback", mapForm(fields), nil)
		if IsNotFound(err) {
			return fmt.Errorf("sdn rollback is not supported on this cluster")
		}
	}
	return err
}

func sdnQuery(fields map[string]any) string {
	q := mapForm(fields)
	if encoded := q.Encode(); encoded != "" {
		return "?" + encoded
	}
	return ""
}

func (c *Client) SDNDryRun(ctx context.Context, node string) (FeatureObject, error) {
	path := "/cluster/sdn/dry-run"
	if node != "" {
		path += "?node=" + url.QueryEscape(node)
	}
	return c.optionalObject(ctx, path)
}

func (c *Client) LockSDN(ctx context.Context, allowPending bool) (FeatureValue, error) {
	fields := map[string]any{}
	if allowPending {
		fields["allow-pending"] = 1
	}
	var token string
	err := c.do(ctx, http.MethodPost, "/cluster/sdn/lock", mapForm(fields), &token)
	if IsNotFound(err) {
		return FeatureValue{Supported: false}, nil
	}
	if err != nil {
		return FeatureValue{}, err
	}
	return FeatureValue{Supported: true, Value: token}, nil
}

func (c *Client) UnlockSDN(ctx context.Context, token string, force bool) (DiskRescanResult, error) {
	fields := map[string]any{}
	if token != "" {
		fields["lock-token"] = token
	}
	if force {
		fields["force"] = 1
	}
	err := c.do(ctx, http.MethodDelete, "/cluster/sdn/lock"+sdnQuery(fields), nil, nil)
	if IsNotFound(err) {
		return DiskRescanResult{Supported: false}, nil
	}
	if err != nil {
		return DiskRescanResult{}, err
	}
	return DiskRescanResult{Supported: true}, nil
}

func (c *Client) SDNIPAMStatus(ctx context.Context, id string) (FeatureObject, error) {
	return c.optionalObject(ctx, "/cluster/sdn/ipams/"+url.PathEscape(id)+"/status")
}

func (c *Client) SDNFabricInterfaces(ctx context.Context, node, fabric string) (FeatureList, error) {
	return c.optionalMaps(ctx, "/nodes/"+url.PathEscape(node)+"/sdn/fabrics/"+url.PathEscape(fabric)+"/interfaces")
}

func (c *Client) CreateSDNVNetIP(ctx context.Context, vnet string, fields map[string]any) (DiskRescanResult, error) {
	err := c.do(ctx, http.MethodPost, "/cluster/sdn/vnets/"+url.PathEscape(vnet)+"/ips", mapForm(fields), nil)
	if IsNotFound(err) {
		return DiskRescanResult{Supported: false}, nil
	}
	if err != nil {
		return DiskRescanResult{}, err
	}
	return DiskRescanResult{Supported: true}, nil
}

func (c *Client) DeleteSDNVNetIP(ctx context.Context, vnet string, fields map[string]any) (DiskRescanResult, error) {
	err := c.do(ctx, http.MethodDelete, "/cluster/sdn/vnets/"+url.PathEscape(vnet)+"/ips"+sdnQuery(fields), nil, nil)
	if IsNotFound(err) {
		return DiskRescanResult{Supported: false}, nil
	}
	if err != nil {
		return DiskRescanResult{}, err
	}
	return DiskRescanResult{Supported: true}, nil
}

func (c *Client) SDNZones(ctx context.Context) ([]SDNZone, error) {
	var out []SDNZone
	err := c.do(ctx, http.MethodGet, "/cluster/sdn/zones", nil, &out)
	return out, err
}

func (c *Client) CreateSDNZone(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/sdn/zones", mapForm(fields), nil)
}

func (c *Client) UpdateSDNZone(ctx context.Context, zone string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/sdn/zones/"+url.PathEscape(zone), mapForm(fields), nil)
}

func (c *Client) DeleteSDNZone(ctx context.Context, zone string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/sdn/zones/"+url.PathEscape(zone), nil, nil)
}

func (c *Client) SDNVNets(ctx context.Context) ([]SDNVNet, error) {
	var out []SDNVNet
	err := c.do(ctx, http.MethodGet, "/cluster/sdn/vnets", nil, &out)
	return out, err
}

func (c *Client) CreateSDNVNet(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/sdn/vnets", mapForm(fields), nil)
}

func (c *Client) UpdateSDNVNet(ctx context.Context, vnet string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/sdn/vnets/"+url.PathEscape(vnet), mapForm(fields), nil)
}

func (c *Client) DeleteSDNVNet(ctx context.Context, vnet string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/sdn/vnets/"+url.PathEscape(vnet), nil, nil)
}

func (c *Client) SDNSubnets(ctx context.Context, vnet string) ([]SDNSubnet, error) {
	var out []SDNSubnet
	err := c.do(ctx, http.MethodGet, "/cluster/sdn/vnets/"+url.PathEscape(vnet)+"/subnets", nil, &out)
	return out, err
}

func (c *Client) CreateSDNSubnet(ctx context.Context, vnet string, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/sdn/vnets/"+url.PathEscape(vnet)+"/subnets", mapForm(fields), nil)
}

func (c *Client) UpdateSDNSubnet(ctx context.Context, vnet, subnet string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/sdn/vnets/"+url.PathEscape(vnet)+"/subnets/"+encodeCIDR(subnet), mapForm(fields), nil)
}

func (c *Client) DeleteSDNSubnet(ctx context.Context, vnet, subnet string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/sdn/vnets/"+url.PathEscape(vnet)+"/subnets/"+encodeCIDR(subnet), nil, nil)
}

func (c *Client) SDNVNetIPs(ctx context.Context, vnet string) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/sdn/vnets/"+url.PathEscape(vnet)+"/ips")
}

func (c *Client) SDNControllers(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/sdn/controllers")
}

func (c *Client) CreateSDNController(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/sdn/controllers", mapForm(fields), nil)
}

func (c *Client) UpdateSDNController(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/sdn/controllers/"+url.PathEscape(id), mapForm(fields), nil)
}

func (c *Client) DeleteSDNController(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/sdn/controllers/"+url.PathEscape(id), nil, nil)
}

func (c *Client) SDNIPAMs(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/sdn/ipams")
}

func (c *Client) CreateSDNIPAM(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/sdn/ipams", mapForm(fields), nil)
}

func (c *Client) UpdateSDNIPAM(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/sdn/ipams/"+url.PathEscape(id), mapForm(fields), nil)
}

func (c *Client) DeleteSDNIPAM(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/sdn/ipams/"+url.PathEscape(id), nil, nil)
}

func (c *Client) SDNDNS(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/sdn/dns")
}

func (c *Client) CreateSDNDNS(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/sdn/dns", mapForm(fields), nil)
}

func (c *Client) UpdateSDNDNS(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/sdn/dns/"+url.PathEscape(id), mapForm(fields), nil)
}

func (c *Client) DeleteSDNDNS(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/sdn/dns/"+url.PathEscape(id), nil, nil)
}

func (c *Client) SDNFabrics(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/sdn/fabrics")
}

func (c *Client) CreateSDNFabric(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/sdn/fabrics", mapForm(fields), nil)
}

func (c *Client) UpdateSDNFabric(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/sdn/fabrics/"+url.PathEscape(id), mapForm(fields), nil)
}

func (c *Client) DeleteSDNFabric(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/sdn/fabrics/"+url.PathEscape(id), nil, nil)
}

func (c *Client) SDNFabricNodes(ctx context.Context, id string) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/sdn/fabrics/"+url.PathEscape(id)+"/nodes")
}

func (c *Client) CreateSDNFabricNode(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/sdn/fabrics/"+url.PathEscape(id)+"/nodes", mapForm(fields), nil)
}

func (c *Client) UpdateSDNFabricNode(ctx context.Context, id, node string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/sdn/fabrics/"+url.PathEscape(id)+"/nodes/"+url.PathEscape(node), mapForm(fields), nil)
}

func (c *Client) DeleteSDNFabricNode(ctx context.Context, id, node string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/sdn/fabrics/"+url.PathEscape(id)+"/nodes/"+url.PathEscape(node), nil, nil)
}

func (c *Client) SDNFabricRouteMaps(ctx context.Context, id string) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/sdn/fabrics/"+url.PathEscape(id)+"/routemaps")
}

func (c *Client) CreateSDNFabricRouteMap(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/sdn/fabrics/"+url.PathEscape(id)+"/routemaps", mapForm(fields), nil)
}

func (c *Client) DeleteSDNFabricRouteMap(ctx context.Context, id, name string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/sdn/fabrics/"+url.PathEscape(id)+"/routemaps/"+url.PathEscape(name), nil, nil)
}

func (c *Client) SDNRouteMaps(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/sdn/route-maps")
}

func (c *Client) CreateSDNRouteMap(ctx context.Context, fields map[string]any) error {
	err := c.do(ctx, http.MethodPost, "/cluster/sdn/route-maps", mapForm(fields), nil)
	if IsNotFound(err) {
		id, _ := fields["id"].(string)
		if id == "" {
			return err
		}
		entry := map[string]any{"action": "permit", "order": 0}
		return c.CreateSDNRouteMapEntry(ctx, id, entry)
	}
	return err
}

func (c *Client) DeleteSDNRouteMap(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodDelete, "/cluster/sdn/route-maps/"+url.PathEscape(id), nil, nil)
	if !IsNotFound(err) {
		return err
	}
	entries, listErr := c.SDNRouteMapEntries(ctx, id)
	if listErr != nil {
		return listErr
	}
	if !entries.Supported {
		return err
	}
	for _, item := range entries.Items {
		order := fmt.Sprint(item["order"])
		if order == "" || order == "<nil>" {
			continue
		}
		if delErr := c.DeleteSDNRouteMapEntry(ctx, id, order); delErr != nil {
			return delErr
		}
	}
	return nil
}

func (c *Client) SDNRouteMapEntries(ctx context.Context, id string) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/sdn/route-maps/entries/"+url.PathEscape(id))
}

func (c *Client) CreateSDNRouteMapEntry(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/sdn/route-maps/entries/"+url.PathEscape(id)+"/entry", mapForm(fields), nil)
}

func (c *Client) UpdateSDNRouteMapEntry(ctx context.Context, id, order string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/sdn/route-maps/entries/"+url.PathEscape(id)+"/entry/"+url.PathEscape(order), mapForm(fields), nil)
}

func (c *Client) DeleteSDNRouteMapEntry(ctx context.Context, id, order string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/sdn/route-maps/entries/"+url.PathEscape(id)+"/entry/"+url.PathEscape(order), nil, nil)
}

func (c *Client) SDNPrefixLists(ctx context.Context) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/sdn/prefix-lists")
}

func (c *Client) CreateSDNPrefixList(ctx context.Context, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/sdn/prefix-lists", mapForm(fields), nil)
}

func (c *Client) UpdateSDNPrefixList(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/sdn/prefix-lists/"+url.PathEscape(id), mapForm(fields), nil)
}

func (c *Client) DeleteSDNPrefixList(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/sdn/prefix-lists/"+url.PathEscape(id), nil, nil)
}

func (c *Client) SDNPrefixListEntries(ctx context.Context, id string) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/sdn/prefix-lists/"+url.PathEscape(id)+"/entries")
}

func (c *Client) CreateSDNPrefixListEntry(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/sdn/prefix-lists/"+url.PathEscape(id)+"/entries", mapForm(fields), nil)
}

func (c *Client) UpdateSDNPrefixListEntry(ctx context.Context, id, seq string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/cluster/sdn/prefix-lists/"+url.PathEscape(id)+"/entries/"+url.PathEscape(seq), mapForm(fields), nil)
}

func (c *Client) DeleteSDNPrefixListEntry(ctx context.Context, id, seq string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/sdn/prefix-lists/"+url.PathEscape(id)+"/entries/"+url.PathEscape(seq), nil, nil)
}

func (c *Client) SDNFabricPrefixLists(ctx context.Context, id string) (FeatureList, error) {
	return c.optionalMaps(ctx, "/cluster/sdn/fabrics/"+url.PathEscape(id)+"/prefixlists")
}

func (c *Client) CreateSDNFabricPrefixList(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, http.MethodPost, "/cluster/sdn/fabrics/"+url.PathEscape(id)+"/prefixlists", mapForm(fields), nil)
}

func (c *Client) DeleteSDNFabricPrefixList(ctx context.Context, id, name string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/sdn/fabrics/"+url.PathEscape(id)+"/prefixlists/"+url.PathEscape(name), nil, nil)
}

func (c *Client) NodeSDN(ctx context.Context, node string) (NodeSDNRuntime, error) {
	base := "/nodes/" + url.PathEscape(node) + "/sdn"
	zones, err := c.optionalMaps(ctx, base)
	if err != nil {
		return NodeSDNRuntime{}, err
	}
	if !zones.Supported {
		zones, err = c.optionalMaps(ctx, base+"/zones")
		if err != nil {
			return NodeSDNRuntime{}, err
		}
	}
	if !zones.Supported {
		return NodeSDNRuntime{Supported: false}, nil
	}
	neighbors, err := c.optionalMaps(ctx, base+"/neighbors")
	if err != nil {
		return NodeSDNRuntime{}, err
	}
	routes, err := c.optionalMaps(ctx, base+"/routes")
	if err != nil {
		return NodeSDNRuntime{}, err
	}
	return NodeSDNRuntime{
		Supported: true,
		Zones:     zones.Items,
		Neighbors: neighbors.Items,
		Routes:    routes.Items,
	}, nil
}
