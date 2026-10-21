package pve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
)

type NodeStorage struct {
	Storage string `json:"storage"`
	Disable int    `json:"disable,omitempty"`
	Enabled *int   `json:"enabled,omitempty"`
}

func (s NodeStorage) usable() bool {
	if s.Storage == "" || s.Disable != 0 {
		return false
	}
	if s.Enabled != nil && *s.Enabled == 0 {
		return false
	}
	return true
}

type NodeIORRD struct {
	Disk []RRDPoint `json:"disk"`
	IOPS []RRDPoint `json:"iops"`
}

func (c *Client) NodeStorages(ctx context.Context, node string) ([]NodeStorage, error) {
	var out []NodeStorage
	err := c.do(ctx, http.MethodGet, "/nodes/"+url.PathEscape(node)+"/storage", nil, &out)
	return out, err
}

func (c *Client) NodeIORRD(ctx context.Context, node, timeframe string) (NodeIORRD, error) {
	out := NodeIORRD{Disk: []RRDPoint{}, IOPS: []RRDPoint{}}
	stores, err := c.NodeStorages(ctx, node)
	if err == nil {
		var sets [][]RRDPoint
		for _, st := range stores {
			if !st.usable() {
				continue
			}
			pts, err := c.StorageRRD(ctx, node, st.Storage, timeframe)
			if err != nil {
				continue
			}
			sets = append(sets, pts)
		}
		out.Disk, out.IOPS = MergeStorageIO(sets)
	}
	needDisk, needIOPS := len(out.Disk) == 0, len(out.IOPS) == 0
	if needDisk || needIOPS {
		if guests, err := c.ClusterResources(ctx); err == nil {
			var sets [][]RRDPoint
			for _, r := range guests {
				if r.Node != node || (r.Type != "qemu" && r.Type != "lxc") || r.Template != 0 {
					continue
				}
				pts, err := c.GuestRRD(ctx, node, r.Type, r.VMID, timeframe)
				if err != nil {
					continue
				}
				sets = append(sets, pts)
			}
			disk, iops := MergeStorageIO(sets)
			if needDisk && len(disk) > 0 {
				out.Disk = disk
			}
			if needIOPS && len(iops) > 0 {
				out.IOPS = iops
			}
		}
	}
	return out, nil
}

func rrdFloat(p RRDPoint, keys ...string) (float64, bool) {
	for _, k := range keys {
		v, ok := p[k]
		if !ok || v == nil {
			continue
		}
		switch n := v.(type) {
		case float64:
			return n, true
		case json.Number:
			f, err := n.Float64()
			if err == nil {
				return f, true
			}
		case int:
			return float64(n), true
		case int64:
			return float64(n), true
		case string:
			f, err := strconv.ParseFloat(n, 64)
			if err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

func MergeStorageIO(sets [][]RRDPoint) (disk, iops []RRDPoint) {
	type acc struct {
		rd, wr, rreq, wreq float64
		hasDisk, hasIOPS   bool
	}
	byTime := map[int64]*acc{}
	for _, set := range sets {
		for _, p := range set {
			t, ok := rrdFloat(p, "time")
			if !ok {
				continue
			}
			key := int64(t)
			a := byTime[key]
			if a == nil {
				a = &acc{}
				byTime[key] = a
			}
			if v, ok := rrdFloat(p, "rd_bytes", "diskread", "read"); ok {
				a.rd += v
				a.hasDisk = true
			}
			if v, ok := rrdFloat(p, "wr_bytes", "diskwrite", "write"); ok {
				a.wr += v
				a.hasDisk = true
			}
			if v, ok := rrdFloat(p, "rd_requests", "ioread", "rd_operations"); ok {
				a.rreq += v
				a.hasIOPS = true
			}
			if v, ok := rrdFloat(p, "wr_requests", "iowrite", "wr_operations"); ok {
				a.wreq += v
				a.hasIOPS = true
			}
		}
	}
	times := make([]int64, 0, len(byTime))
	for t := range byTime {
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	for _, t := range times {
		a := byTime[t]
		if a.hasDisk {
			disk = append(disk, RRDPoint{"time": float64(t), "diskread": a.rd, "diskwrite": a.wr})
		}
		if a.hasIOPS {
			iops = append(iops, RRDPoint{"time": float64(t), "rd_requests": a.rreq, "wr_requests": a.wreq})
		}
	}
	if disk == nil {
		disk = []RRDPoint{}
	}
	if iops == nil {
		iops = []RRDPoint{}
	}
	return disk, iops
}
