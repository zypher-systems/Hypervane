package poller

import (
	"context"
	"strconv"
	"sync"

	"hypervane/internal/pve"
)

func (p *Poller) pollPending(ctx context.Context) {
	next := make(map[string]bool)
	sem := make(chan struct{}, 6)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, cc := range p.clients() {
		if cc.Client == nil {
			continue
		}
		_, resources, err := p.CachedResources(ctx, cc.ID)
		if err != nil || len(resources) == 0 {
			continue
		}
		for _, r := range resources {
			if (r.Type != "qemu" && r.Type != "lxc") || r.Template != 0 || r.Node == "" || r.VMID == 0 {
				continue
			}
			wg.Add(1)
			go func(cc ConnectionClient, r pve.Resource) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				items, err := cc.Client.GuestPending(ctx, r.Node, r.Type, r.VMID)
				if err != nil || !pve.HasPendingChanges(items) {
					return
				}
				mu.Lock()
				next[cc.ID+":"+strconv.Itoa(r.VMID)] = true
				mu.Unlock()
			}(cc, r)
		}
	}
	wg.Wait()
	p.pendingMu.Lock()
	p.pendingVMs = next
	p.pendingMu.Unlock()
}

func (p *Poller) stampPending(connectionID string, resources []pve.Resource) {
	p.pendingMu.Lock()
	defer p.pendingMu.Unlock()
	for i := range resources {
		if resources[i].Type != "qemu" && resources[i].Type != "lxc" {
			continue
		}
		resources[i].Pending = p.pendingVMs[connectionID+":"+strconv.Itoa(resources[i].VMID)]
	}
}
