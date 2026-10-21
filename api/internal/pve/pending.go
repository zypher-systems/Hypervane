package pve

// HasPendingChanges reports whether any guest config key still needs a
// reboot or hotplug to take effect.
func HasPendingChanges(items []PendingChange) bool {
	for _, p := range items {
		if p.Delete != 0 || p.Pending != nil {
			return true
		}
	}
	return false
}
