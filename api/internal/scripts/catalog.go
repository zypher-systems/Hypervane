// Package scripts is the Recipes library: curated, typed jobs that run as
// ordinary Proxmox API calls with the caller's own credentials. Each recipe
// validates its inputs and plans its exact steps before anything changes, so
// the preview a user reviews is the list of calls that will run.
package scripts

// Categories group recipes on the Recipes page.
const (
	CategoryTemplates = "templates"
	CategoryGuests    = "guests"
	CategoryNodes     = "nodes"
	CategoryProtect   = "protection"
)

// Kinds pick the planner that turns a recipe and its inputs into steps.
const (
	KindCloudTemplate   = "cloud-template"
	KindRefreshTemplate = "refresh-template"
	KindDeploy          = "deploy"
	KindDeployMany      = "deploy-many"
	KindSnapshotNode    = "snapshot-node"
	KindSnapshotCleanup = "snapshot-cleanup"
	KindBackupNow       = "backup-unprotected"
	KindBackupJob       = "backup-job"
	KindCheckUpdates    = "check-updates"
	KindShutdownNode    = "shutdown-node"
	KindStartNode       = "start-node"
	KindEvacuateNode    = "evacuate-node"
	KindEnableAgent     = "enable-agent"
	KindConvertTemplate = "convert-template"
)

// Field is one input on a recipe's form. Type tells the page which control to
// draw: node, target-node, storage, backup-storage, guest, qemu-guest,
// template, source, mode, number, text, keys, bool.
type Field struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Default  any    `json:"default,omitempty"`
	Help     string `json:"help,omitempty"`
	Optional bool   `json:"optional,omitempty"`
	// Advanced fields sit behind "More options" so the common case stays short.
	Advanced bool `json:"advanced,omitempty"`
}

// Recipe is one entry in the library.
type Recipe struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Summary  string  `json:"summary"`
	Category string  `json:"category"`
	Kind     string  `json:"kind"`
	Fields   []Field `json:"fields"`
	// Family groups image recipes on the page ("Debian", "Enterprise Linux").
	Family      string `json:"family,omitempty"`
	Image       *Image `json:"-"`
	DefaultName string `json:"-"`
}

// Image is where a template recipe's cloud image comes from.
type Image struct {
	// URL is a link that always serves the current build, or, with Index set,
	// the pinned build used when the newest can't be looked up.
	URL string
	// Filename is what the image is saved as on the import store. Proxmox only
	// imports .qcow2, .raw and .vmdk names, whatever the mirror calls the file.
	Filename string
	// Index and Pattern find the newest build for mirrors without a stable
	// link: Pattern matches file names in the Index listing, and the file is
	// downloaded from Index plus the newest match.
	Index   string
	Pattern string
	// CPU overrides Proxmox's default CPU type, for images that need newer
	// instructions (Enterprise Linux 10 needs x86-64-v3).
	CPU string
	// Agent is true when the image ships the QEMU guest agent.
	Agent bool
}

// Catalog is the whole library, in page order.
var Catalog = []Recipe{
	// ---- Templates -------------------------------------------------------
	cloud("debian-13-template", "Debian 13 (trixie)", "Debian", "debian-13-cloud", Image{
		URL:      "https://cloud.debian.org/images/cloud/trixie/latest/debian-13-generic-amd64.qcow2",
		Filename: "debian-13-generic-amd64.qcow2",
	}),
	cloud("debian-12-template", "Debian 12 (bookworm)", "Debian", "debian-12-cloud", Image{
		URL:      "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2",
		Filename: "debian-12-generic-amd64.qcow2",
	}),
	cloud("ubuntu-2604-template", "Ubuntu 26.04 LTS", "Ubuntu", "ubuntu-2604-cloud", Image{
		URL:      "https://cloud-images.ubuntu.com/resolute/current/resolute-server-cloudimg-amd64.img",
		Filename: "resolute-server-cloudimg-amd64.qcow2",
	}),
	cloud("ubuntu-2404-template", "Ubuntu 24.04 LTS", "Ubuntu", "ubuntu-2404-cloud", Image{
		URL:      "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
		Filename: "noble-server-cloudimg-amd64.qcow2",
	}),
	cloud("ubuntu-2204-template", "Ubuntu 22.04 LTS", "Ubuntu", "ubuntu-2204-cloud", Image{
		URL:      "https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img",
		Filename: "jammy-server-cloudimg-amd64.qcow2",
	}),
	cloud("fedora-44-template", "Fedora Cloud 44", "Fedora", "fedora-44-cloud", Image{
		URL:      "https://download.fedoraproject.org/pub/fedora/linux/releases/44/Cloud/x86_64/images/Fedora-Cloud-Base-Generic-44-1.7.x86_64.qcow2",
		Filename: "Fedora-Cloud-Base-Generic-44-1.7.x86_64.qcow2",
		Index:    "https://download.fedoraproject.org/pub/fedora/linux/releases/44/Cloud/x86_64/images/",
		Pattern:  `Fedora-Cloud-Base-Generic-44-[0-9.]+\.x86_64\.qcow2`,
		Agent:    true,
	}),
	cloud("alpine-324-template", "Alpine 3.24", "Alpine", "alpine-324-cloud", Image{
		URL:      "https://dl-cdn.alpinelinux.org/alpine/v3.24/releases/cloud/generic_alpine-3.24.1-x86_64-bios-cloudinit-r0.qcow2",
		Filename: "generic_alpine-3.24.1-x86_64-bios-cloudinit-r0.qcow2",
		Index:    "https://dl-cdn.alpinelinux.org/alpine/v3.24/releases/cloud/",
		Pattern:  `generic_alpine-3\.24\.[0-9]+-x86_64-bios-cloudinit-r[0-9]+\.qcow2`,
	}),
	cloud("opensuse-leap-16-template", "openSUSE Leap 16.0", "openSUSE", "leap-16-cloud", Image{
		URL:      "https://download.opensuse.org/distribution/leap/16.0/appliances/Leap-16.0-Minimal-VM.x86_64-Cloud.qcow2",
		Filename: "Leap-16.0-Minimal-VM.x86_64-Cloud.qcow2",
	}),
	cloud("arch-template", "Arch Linux", "Arch", "arch-cloud", Image{
		URL:      "https://geo.mirror.pkgbuild.com/images/latest/Arch-Linux-x86_64-cloudimg.qcow2",
		Filename: "Arch-Linux-x86_64-cloudimg.qcow2",
	}),

	// ---- Templates: Enterprise Linux -------------------------------------
	cloud("almalinux-10-template", "AlmaLinux 10", "Enterprise Linux", "almalinux-10-cloud", Image{
		URL:      "https://repo.almalinux.org/almalinux/10/cloud/x86_64/images/AlmaLinux-10-GenericCloud-latest.x86_64.qcow2",
		Filename: "AlmaLinux-10-GenericCloud-latest.x86_64.qcow2",
		CPU:      "x86-64-v3",
		Agent:    true,
	}),
	cloud("almalinux-9-template", "AlmaLinux 9", "Enterprise Linux", "almalinux-9-cloud", Image{
		URL:      "https://repo.almalinux.org/almalinux/9/cloud/x86_64/images/AlmaLinux-9-GenericCloud-latest.x86_64.qcow2",
		Filename: "AlmaLinux-9-GenericCloud-latest.x86_64.qcow2",
		Agent:    true,
	}),
	cloud("rocky-10-template", "Rocky Linux 10", "Enterprise Linux", "rocky-10-cloud", Image{
		URL:      "https://dl.rockylinux.org/pub/rocky/10/images/x86_64/Rocky-10-GenericCloud-Base.latest.x86_64.qcow2",
		Filename: "Rocky-10-GenericCloud-Base.latest.x86_64.qcow2",
		CPU:      "x86-64-v3",
		Agent:    true,
	}),
	cloud("rocky-9-template", "Rocky Linux 9", "Enterprise Linux", "rocky-9-cloud", Image{
		URL:      "https://dl.rockylinux.org/pub/rocky/9/images/x86_64/Rocky-9-GenericCloud-Base.latest.x86_64.qcow2",
		Filename: "Rocky-9-GenericCloud-Base.latest.x86_64.qcow2",
		Agent:    true,
	}),
	cloud("centos-stream-10-template", "CentOS Stream 10", "Enterprise Linux", "centos-stream-10-cloud", Image{
		URL:      "https://cloud.centos.org/centos/10-stream/x86_64/images/CentOS-Stream-GenericCloud-10-latest.x86_64.qcow2",
		Filename: "CentOS-Stream-GenericCloud-10-latest.x86_64.qcow2",
		CPU:      "x86-64-v3",
		Agent:    true,
	}),
	{
		ID:       "refresh-template",
		Title:    "Refresh a template",
		Summary:  "Rebuild a cloud-image template from the newest image, keeping its VMID, name, hardware and cloud-init settings.",
		Category: CategoryTemplates,
		Kind:     KindRefreshTemplate,
		Fields: []Field{
			{Name: "vmid", Label: "Template", Type: "template"},
			{Name: "source", Label: "Image", Type: "source", Help: "Picked for you when Hypervane built the template."},
		},
	},

	// ---- Guests ----------------------------------------------------------
	{
		ID:       "deploy-from-template",
		Title:    "Deploy a guest from a template",
		Summary:  "Clone a template into a new guest, set its login and network through cloud-init, and start it.",
		Category: CategoryGuests,
		Kind:     KindDeploy,
		Fields: append([]Field{
			{Name: "vmid", Label: "Template", Type: "template"},
			{Name: "name", Label: "Name", Type: "text", Help: "Letters, digits, dots and dashes."},
			{Name: "newId", Label: "New VMID", Type: "number", Optional: true, Help: "Leave empty for the next free ID."},
		}, deployFields()...),
	},
	{
		ID:       "deploy-many",
		Title:    "Deploy several guests from a template",
		Summary:  "Clone a template into a numbered set of guests (web-1, web-2, …) with the same login and network settings.",
		Category: CategoryGuests,
		Kind:     KindDeployMany,
		Fields: append([]Field{
			{Name: "vmid", Label: "Template", Type: "template"},
			{Name: "name", Label: "Name prefix", Type: "text", Help: "Guests are named prefix-1, prefix-2, …"},
			{Name: "count", Label: "How many", Type: "number", Default: 3, Help: "Up to 20. They take the next free VMIDs."},
		}, withoutStaticIP(deployFields())...),
	},
	{
		ID:       "enable-agent",
		Title:    "Turn on the QEMU guest agent",
		Summary:  "Let Proxmox talk to the guest agent for clean shutdowns, IP addresses and consistent backups.",
		Category: CategoryGuests,
		Kind:     KindEnableAgent,
		Fields: []Field{
			{Name: "vmid", Label: "VM", Type: "qemu-guest", Help: "The guest needs the qemu-guest-agent package; a running VM picks this up after its next full stop and start."},
		},
	},
	{
		ID:       "convert-to-template",
		Title:    "Turn a VM into a template",
		Summary:  "Convert a stopped guest into a template you can clone. This can't be undone.",
		Category: CategoryGuests,
		Kind:     KindConvertTemplate,
		Fields: []Field{
			{Name: "vmid", Label: "Guest", Type: "guest", Help: "Must be stopped."},
		},
	},

	// ---- Nodes -----------------------------------------------------------
	{
		ID:       "check-updates",
		Title:    "Check every node for updates",
		Summary:  "Refresh the package lists on each node and report how many updates are waiting. Installs nothing.",
		Category: CategoryNodes,
		Kind:     KindCheckUpdates,
	},
	{
		ID:       "shutdown-node-guests",
		Title:    "Shut down a node's guests",
		Summary:  "Cleanly shut down every running guest on a node, for maintenance or a reboot.",
		Category: CategoryNodes,
		Kind:     KindShutdownNode,
		Fields: []Field{
			{Name: "node", Label: "Node", Type: "node"},
			{Name: "force", Label: "Force off guests that haven't shut down after 3 minutes", Type: "bool", Default: true},
		},
	},
	{
		ID:       "start-node-guests",
		Title:    "Start a node's guests",
		Summary:  "Start the stopped guests on a node, after maintenance or a reboot.",
		Category: CategoryNodes,
		Kind:     KindStartNode,
		Fields: []Field{
			{Name: "node", Label: "Node", Type: "node"},
			{Name: "onbootOnly", Label: "Only guests set to start at boot", Type: "bool", Default: true},
		},
	},
	{
		ID:       "evacuate-node",
		Title:    "Move all guests off a node",
		Summary:  "Migrate every guest on a node to another node before maintenance. Running VMs move live.",
		Category: CategoryNodes,
		Kind:     KindEvacuateNode,
		Fields: []Field{
			{Name: "node", Label: "Node to empty", Type: "node"},
			{Name: "target", Label: "Move guests to", Type: "target-node"},
		},
	},

	// ---- Backups and snapshots ------------------------------------------
	{
		ID:       "backup-unprotected",
		Title:    "Back up unprotected guests now",
		Summary:  "Back up every guest that no backup job covers, one task per node.",
		Category: CategoryProtect,
		Kind:     KindBackupNow,
		Fields: []Field{
			{Name: "node", Label: "Node", Type: "node", Optional: true, Help: "Leave on All nodes to cover the whole cluster."},
			{Name: "storage", Label: "Backup storage", Type: "backup-storage"},
			{Name: "mode", Label: "Mode", Type: "mode", Default: "snapshot", Advanced: true, Help: "Snapshot keeps guests running; stop shuts them down for the backup."},
		},
	},
	{
		ID:       "snapshot-node",
		Title:    "Snapshot a node's guests",
		Summary:  "Take one snapshot of each guest on a node, with the same name, before a risky change.",
		Category: CategoryProtect,
		Kind:     KindSnapshotNode,
		Fields: []Field{
			{Name: "node", Label: "Node", Type: "node"},
			{Name: "snapName", Label: "Snapshot name", Type: "text", Optional: true, Help: "Leave empty for a dated name like hv-20260925-1400."},
			{Name: "runningOnly", Label: "Running guests only", Type: "bool", Default: false},
		},
	},
	{
		ID:       "backup-job-unprotected",
		Title:    "Schedule backups for unprotected guests",
		Summary:  "Create a nightly backup job for every guest no job covers yet, with sensible retention.",
		Category: CategoryProtect,
		Kind:     KindBackupJob,
		Fields: []Field{
			{Name: "storage", Label: "Backup storage", Type: "backup-storage"},
			{Name: "schedule", Label: "When", Type: "text", Default: "21:00", Help: "Proxmox calendar format: 21:00 is every night, sat 02:00 is weekly."},
			{Name: "keep", Label: "Keep daily backups for (days)", Type: "number", Default: 7, Advanced: true, Help: "Plus 4 weekly and 3 monthly."},
			{Name: "mode", Label: "Mode", Type: "mode", Default: "snapshot", Advanced: true},
		},
	},
	{
		ID:       "snapshot-cleanup",
		Title:    "Clean up old snapshots",
		Summary:  "Delete snapshots older than a number of days. Old snapshots keep growing and slow disks down.",
		Category: CategoryProtect,
		Kind:     KindSnapshotCleanup,
		Fields: []Field{
			{Name: "node", Label: "Node", Type: "node", Optional: true, Help: "Leave on All nodes to cover the whole cluster."},
			{Name: "days", Label: "Older than (days)", Type: "number", Default: 30},
			{Name: "onlyHypervane", Label: "Only snapshots Hypervane took (named hv-…)", Type: "bool", Default: true},
		},
	},
}

// cloud builds a template recipe for a cloud image.
func cloud(id, title, family, name string, img Image) Recipe {
	summary := "Download the " + title + " cloud image and turn it into a template with cloud-init, ready to clone."
	return Recipe{
		ID:          id,
		Title:       title,
		Summary:     summary,
		Category:    CategoryTemplates,
		Kind:        KindCloudTemplate,
		Family:      family,
		Image:       &img,
		DefaultName: name,
		Fields:      templateFields(name, img.Agent),
	}
}

func templateFields(name string, agent bool) []Field {
	agentHelp := "Install the qemu-guest-agent package in guests you clone from it."
	if agent {
		agentHelp = "This image ships the agent."
	}
	return []Field{
		{Name: "node", Label: "Node", Type: "node", Help: "Downloads the image and keeps the template."},
		{Name: "storage", Label: "Disk storage", Type: "storage"},
		{Name: "name", Label: "Name", Type: "text", Default: name},
		{Name: "vmid", Label: "VMID", Type: "number", Optional: true, Help: "Leave empty for the next free ID."},
		{Name: "user", Label: "Default user", Type: "text", Optional: true, Help: "Leave empty for the image's own (debian, ubuntu, fedora, …)."},
		{Name: "sshKeys", Label: "SSH public keys", Type: "keys", Optional: true, Help: "One per line. Clones inherit them."},
		{Name: "cores", Label: "Cores", Type: "number", Default: 2, Advanced: true},
		{Name: "memory", Label: "Memory (MiB)", Type: "number", Default: 2048, Advanced: true},
		{Name: "diskGb", Label: "Disk (GiB)", Type: "number", Optional: true, Advanced: true, Help: "Leave empty to keep the template as small as the image and grow the disk when you deploy."},
		{Name: "bridge", Label: "Bridge", Type: "text", Default: "vmbr0", Advanced: true},
		{Name: "vlan", Label: "VLAN tag", Type: "number", Optional: true, Advanced: true, Help: "Leave empty for no tag."},
		{Name: "agent", Label: "QEMU guest agent", Type: "bool", Default: true, Advanced: true, Help: agentHelp},
	}
}

func deployFields() []Field {
	return []Field{
		{Name: "user", Label: "User", Type: "text", Optional: true, Help: "Leave empty to keep the template's."},
		{Name: "sshKeys", Label: "SSH public keys", Type: "keys", Optional: true, Help: "Leave empty to keep the template's."},
		{Name: "diskGb", Label: "Disk size (GiB)", Type: "number", Optional: true, Help: "Grows the disk from the template's size. Leave empty to keep it."},
		{Name: "ip", Label: "IP address", Type: "text", Default: "dhcp", Help: "dhcp, or an address with prefix like 192.168.1.50/24."},
		{Name: "gateway", Label: "Gateway", Type: "text", Optional: true, Help: "Needed with a fixed address."},
		{Name: "start", Label: "Start when ready", Type: "bool", Default: true},
		{Name: "full", Label: "Full clone", Type: "bool", Default: true, Advanced: true, Help: "A full clone doesn't depend on the template. A linked clone is faster and smaller but keeps the template in use."},
		{Name: "storage", Label: "Disk storage", Type: "storage", Optional: true, Advanced: true, Help: "Leave empty to use the template's storage."},
		{Name: "cores", Label: "Cores", Type: "number", Optional: true, Advanced: true, Help: "Leave empty to keep the template's."},
		{Name: "memory", Label: "Memory (MiB)", Type: "number", Optional: true, Advanced: true, Help: "Leave empty to keep the template's."},
	}
}

// withoutStaticIP drops the address fields: guests deployed together share
// their settings, so they always use DHCP.
func withoutStaticIP(fields []Field) []Field {
	out := make([]Field, 0, len(fields))
	for _, f := range fields {
		if f.Name != "ip" && f.Name != "gateway" {
			out = append(out, f)
		}
	}
	return out
}

// Lookup finds a recipe by ID.
func Lookup(id string) (Recipe, bool) {
	for _, r := range Catalog {
		if r.ID == id {
			return r, true
		}
	}
	return Recipe{}, false
}

// Public is the catalog as the Recipes page reads it. A recipe with no inputs
// sends an empty field list, not null.
func Public() []Recipe {
	out := make([]Recipe, len(Catalog))
	for i, r := range Catalog {
		if r.Fields == nil {
			r.Fields = []Field{}
		}
		out[i] = r
	}
	return out
}
