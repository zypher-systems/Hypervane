package poller

import (
	"context"
	"log"
	"sync"
	"time"

	"hypervane/internal/pve"
	"hypervane/internal/store"
)

type iopsSnap struct {
	rd int64
	wr int64
	io int64
	ts time.Time
}

func (p *Poller) pollIOPS(ctx context.Context) {
	if p.store == nil {
		return
	}
	for _, cc := range p.clients() {
		if cc.Client == nil {
			continue
		}
		p.pollConnectionIOPS(ctx, cc)
	}
}

func (p *Poller) pollConnectionIOPS(ctx context.Context, cc ConnectionClient) {
	c := cc.Client
	_, resources, err := p.CachedResources(ctx, cc.ID)
	if err != nil || len(resources) == 0 {
		return
	}
	type sample struct {
		key  string
		node string
		vmid int
		rd   int64
		wr   int64
		io   int64
	}
	var (
		mu  sync.Mutex
		got []sample
		sem = make(chan struct{}, 8)
		wg  sync.WaitGroup
	)
	for _, r := range resources {
		if (r.Type != "qemu" && r.Type != "lxc") || r.Template != 0 || r.Node == "" || r.VMID == 0 {
			continue
		}
		if r.Status != "" && r.Status != "running" && r.Uptime == 0 {
			continue
		}
		wg.Add(1)
		go func(r pve.Resource) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			st, err := c.GuestStatus(reqCtx, r.Node, r.Type, r.VMID)
			if err != nil {
				return
			}
			rd, wr, ok := pve.SumBlockOps(st.BlockStat)
			if !ok {
				return
			}
			io, _ := pve.SumBlockTimeNS(st.BlockStat)
			mu.Lock()
			got = append(got, sample{key: cc.ID + ":" + r.ID, node: r.Node, vmid: r.VMID, rd: rd, wr: wr, io: io})
			mu.Unlock()
		}(r)
	}
	wg.Wait()
	if len(got) == 0 {
		return
	}

	now := time.Now()
	type acc struct{ rd, wr float64 }
	type guestRate struct {
		vmid int
		rd   float64
		wr   float64
		wait float64
	}
	byNode := map[string]acc{}
	var guests []guestRate
	p.iopsMu.Lock()
	for _, s := range got {
		prev, ok := p.iopsPrev[s.key]
		p.iopsPrev[s.key] = iopsSnap{rd: s.rd, wr: s.wr, io: s.io, ts: now}
		if !ok {
			continue
		}
		dt := now.Sub(prev.ts)
		rdRate, rdOK := pve.IOPSRate(prev.rd, s.rd, dt)
		wrRate, wrOK := pve.IOPSRate(prev.wr, s.wr, dt)
		wait, waitOK := pve.IOWaitFrac(prev.io, s.io, dt)
		if !rdOK && !wrOK && !waitOK {
			continue
		}
		if !rdOK {
			rdRate = 0
		}
		if !wrOK {
			wrRate = 0
		}
		if !waitOK {
			wait = 0
		}
		if s.vmid > 0 {
			guests = append(guests, guestRate{vmid: s.vmid, rd: rdRate, wr: wrRate, wait: wait})
		}
		n := byNode[s.node]
		n.rd += rdRate
		n.wr += wrRate
		byNode[s.node] = n
	}
	p.iopsMu.Unlock()

	for _, g := range guests {
		if err := p.store.WriteGuestIOPS(ctx, cc.ID, g.vmid, now, g.rd, g.wr, g.wait); err != nil {
			log.Printf("iops write guest %s/%d: %v", cc.ID, g.vmid, err)
		}
	}
	for node, n := range byNode {
		if err := p.store.WriteNodeIOPS(ctx, cc.ID, node, now, n.rd, n.wr); err != nil {
			log.Printf("iops write %s/%s: %v", cc.ID, node, err)
		}
	}
	if now.Sub(p.iopsPrune) > time.Hour {
		p.iopsPrune = now
		cutoff := now.Add(-400 * 24 * time.Hour)
		if err := p.store.PruneNodeIOPS(ctx, cutoff); err != nil {
			log.Printf("iops prune: %v", err)
		}
		if err := p.store.PruneGuestIOPS(ctx, cutoff); err != nil {
			log.Printf("guest iops prune: %v", err)
		}
		if err := p.store.PruneMetricHistory(ctx, cutoff); err != nil {
			log.Printf("metric history prune: %v", err)
		}
	}
}

func (p *Poller) NodeIOPS(ctx context.Context, connectionID, node, timeframe string) ([]pve.RRDPoint, error) {
	if p == nil || p.store == nil {
		return []pve.RRDPoint{}, nil
	}
	to := time.Now()
	from := to.Add(-store.IOPSWindow(timeframe))
	pts, err := p.store.QueryNodeIOPS(ctx, connectionID, node, from, to, store.IOPSStep(timeframe))
	if err != nil {
		return nil, err
	}
	out := make([]pve.RRDPoint, 0, len(pts))
	for _, s := range pts {
		out = append(out, pve.RRDPoint{
			"time":        float64(s.Time.Unix()),
			"rd_requests": s.Rd,
			"wr_requests": s.Wr,
		})
	}
	return out, nil
}

func (p *Poller) GuestIOPS(ctx context.Context, connectionID string, vmid int, timeframe string) ([]pve.RRDPoint, error) {
	if p == nil || p.store == nil {
		return []pve.RRDPoint{}, nil
	}
	to := time.Now()
	from := to.Add(-store.IOPSWindow(timeframe))
	pts, err := p.store.QueryGuestIOPS(ctx, connectionID, vmid, from, to, store.IOPSStep(timeframe))
	if err != nil {
		return nil, err
	}
	out := make([]pve.RRDPoint, 0, len(pts))
	for _, s := range pts {
		out = append(out, pve.RRDPoint{
			"time":        float64(s.Time.Unix()),
			"rd_requests": s.Rd,
			"wr_requests": s.Wr,
			"iowait":      s.Wait,
		})
	}
	return out, nil
}
