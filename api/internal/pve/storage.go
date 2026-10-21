package pve

import (
	"context"
	"net/http"
	"net/url"
)

func (c *Client) Storage(ctx context.Context, storage string) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodGet, "/storage/"+url.PathEscape(storage), nil, &out)
	return out, err
}

func (c *Client) UpdateStorage(ctx context.Context, storage string, fields map[string]any) error {
	return c.do(ctx, http.MethodPut, "/storage/"+url.PathEscape(storage), mapForm(fields), nil)
}

func (c *Client) DeleteStorage(ctx context.Context, storage string) error {
	return c.do(ctx, http.MethodDelete, "/storage/"+url.PathEscape(storage), nil, nil)
}

func (c *Client) UpdateVolumeNotes(ctx context.Context, node, storage, volume, notes string) (DiskRescanResult, error) {
	form := url.Values{"notes": {notes}}
	path := "/nodes/" + url.PathEscape(node) + "/storage/" + url.PathEscape(storage) + "/content/" + url.PathEscape(volume)
	err := c.do(ctx, http.MethodPut, path, form, nil)
	if IsNotFound(err) {
		return DiskRescanResult{Supported: false}, nil
	}
	if err != nil {
		return DiskRescanResult{}, err
	}
	return DiskRescanResult{Supported: true}, nil
}

func (c *Client) OCIPull(ctx context.Context, node, storage string, fields map[string]any) (DiskRescanResult, error) {
	path := "/nodes/" + url.PathEscape(node) + "/storage/" + url.PathEscape(storage) + "/oci-registry-pull"
	var upid string
	err := c.do(ctx, http.MethodPost, path, mapForm(fields), &upid)
	if IsNotFound(err) {
		return DiskRescanResult{Supported: false}, nil
	}
	if err != nil {
		return DiskRescanResult{}, err
	}
	return DiskRescanResult{Supported: true, UPID: upid}, nil
}

func (c *Client) StorageIdentity(ctx context.Context, node, storage string) (FeatureObject, error) {
	return c.optionalObject(ctx, "/nodes/"+url.PathEscape(node)+"/storage/"+url.PathEscape(storage)+"/identity")
}

func (c *Client) UpdateStorageIdentity(ctx context.Context, node, storage string, fields map[string]any) (DiskRescanResult, error) {
	err := c.do(ctx, http.MethodPut, "/nodes/"+url.PathEscape(node)+"/storage/"+url.PathEscape(storage)+"/identity", mapForm(fields), nil)
	if IsNotFound(err) {
		return DiskRescanResult{Supported: false}, nil
	}
	if err != nil {
		return DiskRescanResult{}, err
	}
	return DiskRescanResult{Supported: true}, nil
}

func (c *Client) ScanStorage(ctx context.Context, node, kind string, query map[string]any) (FeatureList, error) {
	switch kind {
	case "nfs", "cifs", "iscsi", "lvm", "lvmthin", "zfs", "pbs":
	default:
		return FeatureList{Supported: false}, nil
	}
	q := mapForm(query)
	path := "/nodes/" + url.PathEscape(node) + "/scan/" + url.PathEscape(kind)
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
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
