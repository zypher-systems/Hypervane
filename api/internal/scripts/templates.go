package scripts

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"hypervane/internal/pve"
)

// recipeMarker ties a template to the recipe that built it, so refreshing it
// knows which image to use.
const recipeMarker = "hypervane-recipe: "

var markerRe = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(recipeMarker) + `(\S+)\s*$`)

// cloudBuild is everything needed to build a template from a cloud image.
type cloudBuild struct {
	recipe      Recipe
	node        string
	importStore string
	diskStore   string
	url, file   string
	vmid        int
	name        string
	cores       int
	memory      int
	diskGB      int
	net0        string
	cpu         string
	agent       string
	user        string
	keys        string
	ipconfig    string
	disk        string // system disk slot
	diskOptions string
	ciSlot      string
	description string
	extra       map[string]any
}

func planCloud(ctx context.Context, c *pve.Client, im *Images, cl *cluster, r Recipe, p Params) (*Plan, error) {
	if r.Image == nil {
		return nil, inputErr("this recipe has no image")
	}
	if _, err := cl.node(p.Node); err != nil {
		return nil, err
	}
	disk, download, err := ResolveStores(cl.storages, p.Node, p.Storage)
	if err != nil {
		return nil, &InputError{Msg: err.Error()}
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = r.DefaultName
	}
	if err := checkName(name); err != nil {
		return nil, err
	}
	// No disk size keeps the template as small as the image; guests grow
	// their disk when they are deployed.
	cores, memory, diskGB := nonzero(p.Cores, 2), nonzero(p.Memory, 2048), p.DiskGB
	if err := between(cores, 1, 512, "cores"); err != nil {
		return nil, err
	}
	if err := between(memory, 256, 4<<20, "memory (MiB)"); err != nil {
		return nil, err
	}
	if diskGB != 0 {
		if err := between(diskGB, 1, 64<<10, "disk (GiB)"); err != nil {
			return nil, err
		}
	}
	bridge := strings.TrimSpace(p.Bridge)
	if bridge == "" {
		bridge = "vmbr0"
	}
	if err := checkBridge(ctx, c, p.Node, bridge); err != nil {
		return nil, err
	}
	if err := between(p.VLAN, 0, 4094, "VLAN tag"); err != nil {
		return nil, err
	}
	if err := checkUser(strings.TrimSpace(p.User)); err != nil {
		return nil, err
	}
	if err := checkSSHKeys(p.SSHKeys); err != nil {
		return nil, err
	}
	vmid, err := pickVMID(ctx, c, cl, p.VMID)
	if err != nil {
		return nil, err
	}
	img := im.Resolve(ctx, *r.Image)
	net0 := "virtio,bridge=" + bridge
	if p.VLAN > 0 {
		net0 += ",tag=" + strconv.Itoa(p.VLAN)
	}
	agent := "0"
	if on(p.Agent, true) {
		agent = "1"
	}
	b := cloudBuild{
		recipe: r, node: p.Node, importStore: download, diskStore: disk,
		url: img.URL, file: img.Filename, vmid: vmid, name: name,
		cores: cores, memory: memory, diskGB: diskGB, net0: net0, cpu: r.Image.CPU, agent: agent,
		user: strings.TrimSpace(p.User), keys: strings.TrimSpace(p.SSHKeys), ipconfig: "ip=dhcp",
		disk: "scsi0", diskOptions: "discard=on", ciSlot: "ide2",
		description: describe(r, img.Filename, ""),
	}
	plan := &Plan{VMID: vmid, Steps: b.steps()}
	plan.Notes = append(plan.Notes, fmt.Sprintf("Image: %s from %s.", img.Filename, host(img.URL)))
	if img.Pinned {
		plan.Notes = append(plan.Notes, "Hypervane couldn't look up the newest build from this host, so it uses the build it knows.")
	}
	if diskGB == 0 {
		plan.Notes = append(plan.Notes, "The disk stays the size of the image. Set a disk size when you deploy a guest from it.")
	}
	plan.Notes = append(plan.Notes, "Guests cloned from it use DHCP unless you set an address when you deploy them.")
	return plan, nil
}

func (b cloudBuild) steps() []Step {
	steps := []Step{
		b.downloadStep(),
		{
			ID:    "create",
			Label: fmt.Sprintf("Create VM %d (%s) on %s from the image", b.vmid, b.name, b.diskStore),
			run:   b.create,
		},
	}
	if b.diskGB > 0 {
		steps = append(steps, Step{
			ID:     "grow",
			Label:  fmt.Sprintf("Grow its disk to %d GiB", b.diskGB),
			OnFail: fmt.Sprintf("VM %d exists but isn't a template yet: delete it, or fix the problem and convert it.", b.vmid),
			run:    b.grow,
		})
	}
	steps = append(steps, []Step{
		{
			ID:     "template",
			Label:  fmt.Sprintf("Convert VM %d to a template", b.vmid),
			OnFail: fmt.Sprintf("VM %d exists but isn't a template yet: delete it, or fix the problem and convert it.", b.vmid),
			run: func(ctx context.Context, x *Exec) error {
				upid, err := x.C.ConvertTemplate(ctx, b.node, "qemu", b.vmid)
				if err != nil {
					return err
				}
				return x.Wait(ctx, b.node, upid)
			},
		},
		b.tidyStep(),
	}...)
	return steps
}

func (b cloudBuild) volid() string { return b.importStore + ":import/" + b.file }

func (b cloudBuild) downloadStep() Step {
	return Step{
		ID:    "download",
		Label: fmt.Sprintf("Download %s to %s on %s", b.file, b.importStore, b.node),
		run: func(ctx context.Context, x *Exec) error {
			// Proxmox won't overwrite an earlier download of the same file.
			if items, err := x.C.StorageContent(ctx, b.node, b.importStore, "import"); err == nil {
				for _, it := range items {
					if it.VolID == b.volid() {
						upid, err := x.C.DeleteVolume(ctx, b.node, b.importStore, b.volid())
						if err != nil {
							return fmt.Errorf("remove the earlier download: %w", err)
						}
						if err := x.Wait(ctx, b.node, upid); err != nil {
							return err
						}
						x.Note("Replaced an earlier download of the same file.")
					}
				}
			}
			upid, err := x.C.DownloadURL(ctx, b.node, b.importStore, pve.DownloadURLRequest{
				URL: b.url, Filename: b.file, Content: "import",
			})
			if err != nil {
				return err
			}
			return x.Wait(ctx, b.node, upid)
		},
	}
}

func (b cloudBuild) tidyStep() Step {
	return Step{
		ID:       "tidy",
		Label:    fmt.Sprintf("Remove the downloaded image from %s", b.importStore),
		Optional: true,
		run: func(ctx context.Context, x *Exec) error {
			upid, err := x.C.DeleteVolume(ctx, b.node, b.importStore, b.volid())
			if err != nil {
				return err
			}
			return x.Wait(ctx, b.node, upid)
		},
	}
}

func (b cloudBuild) create(ctx context.Context, x *Exec) error {
	args := map[string]any{
		b.ciSlot:      b.diskStore + ":cloudinit",
		"boot":        "order=" + b.disk,
		"serial0":     "socket",
		"agent":       b.agent,
		"description": b.description,
	}
	for k, v := range b.extra {
		args[k] = v
	}
	upid, err := x.C.CreateQEMU(ctx, pve.CreateGuestRequest{
		Node:        b.node,
		VMID:        b.vmid,
		Name:        b.name,
		Memory:      b.memory,
		Cores:       b.cores,
		Storage:     b.diskStore,
		OSType:      "l26",
		SCSIHW:      "virtio-scsi-single",
		CPU:         b.cpu,
		CIUser:      b.user,
		SSHKeys:     b.keys,
		IPConfig0:   b.ipconfig,
		CreateArgs:  args,
		ImportFrom:  map[string]string{b.disk: b.volid()},
		DiskOptions: b.diskOptions,
		Nets:        map[string]string{"net0": b.net0},
	})
	if err != nil {
		return err
	}
	return x.Wait(ctx, b.node, upid)
}

func (b cloudBuild) grow(ctx context.Context, x *Exec) error {
	cfg, err := x.C.GuestConfig(ctx, b.node, "qemu", b.vmid)
	if err != nil {
		return err
	}
	want := int64(b.diskGB) << 30
	if cur := diskSize(fmt.Sprint(cfg[b.disk])); cur >= want {
		x.Note(fmt.Sprintf("The image is already %s, so the disk stays that size.", gib(cur)))
		return nil
	}
	upid, err := x.C.ResizeDisk(ctx, b.node, "qemu", b.vmid, pve.DiskResizeRequest{Disk: b.disk, Size: strconv.Itoa(b.diskGB) + "G"})
	if err != nil {
		return err
	}
	return x.Wait(ctx, b.node, upid)
}

// ---- refresh ---------------------------------------------------------------

// refreshKeep are template settings carried over to the rebuilt template.
var refreshKeep = []string{"sockets", "numa", "balloon", "vga", "machine", "ostype", "scsihw", "tags",
	"nameserver", "searchdomain", "cicustom", "ciupgrade", "citype", "onboot", "startup"}

var diskKeyRe = regexp.MustCompile(`^(scsi|virtio|sata|ide|efidisk|tpmstate)[0-9]+$`)

func planRefresh(ctx context.Context, c *pve.Client, im *Images, cl *cluster, p Params) (*Plan, error) {
	if p.VMID <= 0 {
		return nil, inputErr("choose a template")
	}
	g, err := cl.guest(p.VMID)
	if err != nil {
		return nil, err
	}
	if g.Template == 0 {
		return nil, inputErr("%s isn't a template", label(g))
	}
	if g.Type != "qemu" {
		return nil, inputErr("only VM templates can be refreshed")
	}
	cfg, err := c.GuestConfig(ctx, g.Node, "qemu", g.VMID)
	if err != nil {
		return nil, err
	}
	if str(cfg["protection"]) == "1" {
		return nil, inputErr("template %s is protected; turn protection off first", label(g))
	}
	// Only a cloud-image template: one system disk and a cloud-init drive.
	var disk, diskVal, ciSlot, ciStore string
	for k, v := range cfg {
		val := str(v)
		switch {
		case !diskKeyRe.MatchString(k):
			continue
		case strings.Contains(val, "cloudinit"):
			ciSlot, ciStore = k, strings.SplitN(val, ":", 2)[0]
		case strings.Contains(val, "media=cdrom"):
			if !strings.HasPrefix(val, "none") {
				return nil, inputErr("template %s has a CD/DVD image attached (%s); remove it first", label(g), k)
			}
		case disk == "" && (k == "scsi0" || k == "virtio0" || k == "sata0"):
			disk, diskVal = k, val
		default:
			return nil, inputErr("template %s has more than one disk (%s), so it isn't a plain cloud-image template", label(g), k)
		}
	}
	if disk == "" || ciSlot == "" {
		return nil, inputErr("template %s needs one system disk and a cloud-init drive to be rebuilt from a cloud image", label(g))
	}
	src := strings.TrimSpace(p.Source)
	if src == "" {
		if m := markerRe.FindStringSubmatch(str(cfg["description"])); m != nil {
			src = m[1]
		}
	}
	if src == "" {
		return nil, inputErr("choose the image to rebuild it from")
	}
	sr, ok := Lookup(src)
	if !ok || sr.Kind != KindCloudTemplate {
		return nil, inputErr("%q isn't a cloud image", src)
	}
	diskStore := strings.SplitN(diskVal, ":", 2)[0]
	_, download, err := ResolveStores(cl.storages, g.Node, diskStore)
	if err != nil {
		return nil, &InputError{Msg: err.Error()}
	}
	img := im.Resolve(ctx, *sr.Image)
	extra := map[string]any{}
	for _, k := range refreshKeep {
		if v, ok := cfg[k]; ok && str(v) != "" {
			extra[k] = v
		}
	}
	keys := str(cfg["sshkeys"])
	if dec, err := url.PathUnescape(keys); err == nil {
		keys = dec
	}
	sizeGB := int((diskSize(diskVal) + (1<<30 - 1)) >> 30)
	b := cloudBuild{
		recipe: sr, node: g.Node, importStore: download, diskStore: diskStore,
		url: img.URL, file: img.Filename, vmid: g.VMID, name: fallback(str(cfg["name"]), g.Name),
		cores: atoi(cfg["cores"], 1), memory: atoi(cfg["memory"], 512), diskGB: max(sizeGB, 4),
		net0: str(cfg["net0"]), cpu: str(cfg["cpu"]), agent: fallback(str(cfg["agent"]), "0"),
		user: str(cfg["ciuser"]), keys: keys, ipconfig: str(cfg["ipconfig0"]),
		disk: disk, diskOptions: diskOptions(diskVal), ciSlot: ciSlot,
		description: describe(sr, img.Filename, str(cfg["description"])), extra: extra,
	}
	if b.cpu == "" {
		b.cpu = sr.Image.CPU
	}
	b.diskStore, b.ciSlot = diskStore, ciSlot
	if ciStore != "" && ciStore != diskStore {
		b.extra[ciSlot] = ciStore + ":cloudinit"
	}
	if b.net0 == "" {
		return nil, inputErr("template %s has no network device to keep", label(g))
	}
	steps := b.steps()
	del := Step{
		ID:     "delete",
		Label:  fmt.Sprintf("Delete template %s and its disks", label(g)),
		OnFail: "Check the template in Proxmox before trying again.",
		run: func(ctx context.Context, x *Exec) error {
			upid, err := x.C.DeleteGuest(ctx, b.node, "qemu", b.vmid, pve.DeleteGuestOptions{Purge: true, DestroyUnreferencedDisks: true})
			if err != nil {
				return err
			}
			return x.Wait(ctx, b.node, upid)
		},
	}
	// Download first, so a failed download leaves the old template alone.
	steps = append([]Step{steps[0], del}, steps[1:]...)
	for i := range steps {
		if steps[i].ID == "create" {
			steps[i].OnFail = fmt.Sprintf("Template %d is gone and wasn't rebuilt: run this again, or build it with the %s recipe.", b.vmid, sr.Title)
		}
	}
	plan := &Plan{VMID: g.VMID, Steps: steps}
	plan.Confirm = fmt.Sprintf("Template %s is deleted and rebuilt from the newest %s image with the same VMID, name, hardware and cloud-init settings. Guests already cloned from it keep running as they are.", label(g), sr.Title)
	plan.Notes = append(plan.Notes, fmt.Sprintf("Image: %s from %s.", img.Filename, host(img.URL)))
	if str(cfg["cipassword"]) != "" {
		plan.Notes = append(plan.Notes, "The cloud-init password can't be copied; set it again on the new template if you use one.")
	}
	return plan, nil
}

// describe writes the template's notes: the user's own lines, then which
// recipe built it and when.
func describe(r Recipe, file, previous string) string {
	var keep []string
	for _, line := range strings.Split(previous, "\n") {
		if strings.HasPrefix(line, recipeMarker) || strings.HasPrefix(line, "Built by Hypervane") {
			continue
		}
		keep = append(keep, line)
	}
	own := strings.TrimSpace(strings.Join(keep, "\n"))
	built := fmt.Sprintf("Built by Hypervane from %s (%s) on %s.\n%s%s", r.Title, file, time.Now().Format("2006-01-02"), recipeMarker, r.ID)
	if own == "" {
		return built
	}
	return own + "\n\n" + built
}

// ---- deploy ----------------------------------------------------------------

func planDeploy(ctx context.Context, c *pve.Client, cl *cluster, p Params, many bool) (*Plan, error) {
	if p.VMID <= 0 {
		return nil, inputErr("choose a template")
	}
	tpl, err := cl.guest(p.VMID)
	if err != nil {
		return nil, err
	}
	if tpl.Template == 0 {
		return nil, inputErr("%s isn't a template", label(tpl))
	}
	if _, err := cl.node(tpl.Node); err != nil {
		return nil, err
	}
	count := 1
	if many {
		count = p.Count
		if err := between(count, 2, 20, "how many"); err != nil {
			return nil, err
		}
	}
	base := strings.TrimSpace(p.Name)
	if base == "" {
		if many {
			return nil, inputErr("enter a name prefix")
		}
		return nil, inputErr("enter a name")
	}
	names := []string{base}
	if many {
		names = names[:0]
		for i := 1; i <= count; i++ {
			names = append(names, fmt.Sprintf("%s-%d", base, i))
		}
	}
	for _, n := range names {
		if err := checkName(n); err != nil {
			return nil, err
		}
	}
	var ids []int
	if !many && p.NewID > 0 {
		if cl.vmidUsed(p.NewID) {
			return nil, inputErr("VMID %d is already in use", p.NewID)
		}
		ids = []int{p.NewID}
	} else if ids, err = cl.freeIDs(ctx, c, count); err != nil {
		return nil, err
	}
	full := on(p.Full, true)
	storage := strings.TrimSpace(p.Storage)
	if storage != "" {
		if !full {
			return nil, inputErr("a linked clone stays on the template's storage; turn on full clone to pick one")
		}
		if err := checkImageStore(cl.storages, tpl.Node, storage); err != nil {
			return nil, err
		}
	}
	if p.Cores != 0 {
		if err := between(p.Cores, 1, 512, "cores"); err != nil {
			return nil, err
		}
	}
	if p.Memory != 0 {
		if err := between(p.Memory, 256, 4<<20, "memory (MiB)"); err != nil {
			return nil, err
		}
	}
	user := strings.TrimSpace(p.User)
	if err := checkUser(user); err != nil {
		return nil, err
	}
	if err := checkSSHKeys(p.SSHKeys); err != nil {
		return nil, err
	}
	ipcfg := "ip=dhcp"
	if !many {
		if ipcfg, err = ipConfig(p.IP, p.Gateway); err != nil {
			return nil, err
		}
	}
	plan := &Plan{}
	cfg, err := c.GuestConfig(ctx, tpl.Node, tpl.Type, tpl.VMID)
	if err != nil {
		return nil, err
	}
	// Templates are kept small; each guest grows its own disk.
	var disk string
	if p.DiskGB != 0 {
		if err := between(p.DiskGB, 1, 64<<10, "disk size (GiB)"); err != nil {
			return nil, err
		}
		var val string
		disk, val = bootDisk(tpl.Type, cfg)
		if disk == "" {
			return nil, inputErr("couldn't find %s's system disk to grow", label(tpl))
		}
		if cur := diskSize(val); cur >= int64(p.DiskGB)<<30 {
			return nil, inputErr("the template's disk is already %s; enter a larger size or leave it empty", gib(cur))
		}
	}
	// Cloud-init settings only reach a VM with a cloud-init drive.
	fields := map[string]any{}
	if tpl.Type == "qemu" {
		hasCI := false
		for k, v := range cfg {
			if diskKeyRe.MatchString(k) && strings.Contains(str(v), "cloudinit") {
				hasCI = true
			}
		}
		if hasCI {
			if user != "" {
				fields["ciuser"] = user
			}
			if keys := strings.TrimSpace(p.SSHKeys); keys != "" {
				fields["sshkeys"] = pve.EncodeSSHKeys(keys)
			}
			if ipcfg != str(cfg["ipconfig0"]) {
				fields["ipconfig0"] = ipcfg
			}
		} else if user != "" || p.SSHKeys != "" || ipcfg != "ip=dhcp" {
			plan.Notes = append(plan.Notes, "This template has no cloud-init drive, so the user, keys and address settings can't reach the guest.")
		}
	} else if user != "" || p.SSHKeys != "" || ipcfg != "ip=dhcp" {
		plan.Notes = append(plan.Notes, "User, keys and address settings apply to VMs; containers keep the template's.")
	}
	if p.Cores > 0 {
		fields["cores"] = p.Cores
	}
	if p.Memory > 0 {
		fields["memory"] = p.Memory
	}
	start := on(p.Start, true)
	for i, id := range ids {
		id, name, group := id, names[i], "guest-"+strconv.Itoa(ids[i])
		kind := "full clone"
		if !full {
			kind = "linked clone"
		}
		plan.Steps = append(plan.Steps, Step{
			ID:    "clone-" + strconv.Itoa(id),
			Label: fmt.Sprintf("Clone %s into %d (%s) as a %s", label(tpl), id, name, kind),
			Group: group,
			run: func(ctx context.Context, x *Exec) error {
				upid, err := x.C.CloneGuest(ctx, tpl.Node, tpl.Type, tpl.VMID, pve.CloneRequest{NewID: id, Name: name, Full: full, Storage: storage})
				if err != nil {
					return err
				}
				return x.Wait(ctx, tpl.Node, upid)
			},
		})
		if len(fields) > 0 {
			plan.Steps = append(plan.Steps, Step{
				ID:     "configure-" + strconv.Itoa(id),
				Label:  fmt.Sprintf("Set %s on %d", describeFields(fields), id),
				Group:  group,
				OnFail: fmt.Sprintf("Guest %d was created but not configured.", id),
				run: func(ctx context.Context, x *Exec) error {
					return x.C.UpdateGuestConfig(ctx, tpl.Node, tpl.Type, id, fields, nil, "")
				},
			})
		}
		if disk != "" {
			size := strconv.Itoa(p.DiskGB) + "G"
			plan.Steps = append(plan.Steps, Step{
				ID:     "grow-" + strconv.Itoa(id),
				Label:  fmt.Sprintf("Grow %d's disk (%s) to %d GiB", id, disk, p.DiskGB),
				Group:  group,
				OnFail: fmt.Sprintf("Guest %d was created but its disk wasn't grown.", id),
				run: func(ctx context.Context, x *Exec) error {
					upid, err := x.C.ResizeDisk(ctx, tpl.Node, tpl.Type, id, pve.DiskResizeRequest{Disk: disk, Size: size})
					if err != nil {
						return err
					}
					return x.Wait(ctx, tpl.Node, upid)
				},
			})
		}
		if start {
			plan.Steps = append(plan.Steps, Step{
				ID:     "start-" + strconv.Itoa(id),
				Label:  fmt.Sprintf("Start %d", id),
				Group:  group,
				OnFail: fmt.Sprintf("Guest %d was created but didn't start.", id),
				run: func(ctx context.Context, x *Exec) error {
					upid, err := x.C.GuestPower(ctx, tpl.Node, tpl.Type, "start", id)
					if err != nil {
						return err
					}
					return x.Wait(ctx, tpl.Node, upid)
				},
			})
		}
	}
	if len(ids) == 1 {
		plan.VMID = ids[0]
	}
	if !full {
		plan.Notes = append(plan.Notes, "Linked clones share the template's disk, so the template can't be deleted while they exist.")
	}
	return plan, nil
}

func describeFields(fields map[string]any) string {
	var parts []string
	for _, k := range []string{"ciuser", "sshkeys", "ipconfig0", "cores", "memory"} {
		v, ok := fields[k]
		if !ok {
			continue
		}
		switch k {
		case "ciuser":
			parts = append(parts, "user "+str(v))
		case "sshkeys":
			parts = append(parts, "SSH keys")
		case "ipconfig0":
			parts = append(parts, "address "+strings.TrimPrefix(str(v), "ip="))
		case "cores":
			parts = append(parts, str(v)+" cores")
		case "memory":
			parts = append(parts, str(v)+" MiB memory")
		}
	}
	return strings.Join(parts, ", ")
}

// ---- helpers ---------------------------------------------------------------

func pickVMID(ctx context.Context, c *pve.Client, cl *cluster, want int) (int, error) {
	if want > 0 {
		if want < 100 {
			return 0, inputErr("VMIDs start at 100")
		}
		if cl.vmidUsed(want) {
			return 0, inputErr("VMID %d is already in use", want)
		}
		return want, nil
	}
	ids, err := cl.freeIDs(ctx, c, 1)
	if err != nil {
		return 0, err
	}
	return ids[0], nil
}

// checkBridge makes sure the bridge exists on the node. Without it the
// template is created fine and only its clones fail to start. A user who may
// not list the node's network skips the check.
func checkBridge(ctx context.Context, c *pve.Client, node, bridge string) error {
	if !bridgeRe.MatchString(bridge) {
		return inputErr("bridge %q isn't a valid name", bridge)
	}
	ifaces, err := c.NodeNetwork(ctx, node)
	if err != nil {
		return nil
	}
	var names []string
	for _, i := range ifaces {
		if strings.Contains(strings.ToLower(i.Type), "bridge") {
			if i.Iface == bridge {
				return nil
			}
			names = append(names, i.Iface)
		}
	}
	if len(names) == 0 {
		return nil
	}
	return inputErr("%s has no bridge %s (it has %s)", node, bridge, strings.Join(names, ", "))
}

func checkImageStore(list []pve.Storage, node, name string) error {
	for _, st := range list {
		if st.Storage != name {
			continue
		}
		if st.Disable != 0 || !OnNode(st, node) {
			return inputErr("storage %s isn't available on %s", name, node)
		}
		if !HasContent(st, "images") {
			return inputErr("storage %s can't hold VM disks", name)
		}
		return nil
	}
	return inputErr("unknown storage %s", name)
}

var sizeRe = regexp.MustCompile(`(?:^|,)size=([0-9.]+)([KMGT]?)`)

// diskSize reads size= from a Proxmox disk value, in bytes.
func diskSize(val string) int64 {
	m := sizeRe.FindStringSubmatch(val)
	if m == nil {
		return 0
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	mult := map[string]float64{"": 1, "K": 1 << 10, "M": 1 << 20, "G": 1 << 30, "T": 1 << 40}[m[2]]
	return int64(n * mult)
}

// diskOptions keeps a disk's options (discard, iothread, ...) without its
// volume and size, for rebuilding the same disk.
func diskOptions(val string) string {
	parts := strings.Split(val, ",")
	var out []string
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "size=") || strings.HasPrefix(p, "import-from=") {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, ",")
}

func gib(b int64) string {
	return strconv.FormatFloat(float64(b)/(1<<30), 'f', -1, 64) + " GiB"
}

func host(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func atoi(v any, def int) int {
	if n, err := strconv.Atoi(str(v)); err == nil && n > 0 {
		return n
	}
	return def
}

func nonzero(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

func fallback(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// bootDisk finds the disk a guest boots from: a container's rootfs, or the
// first disk in a VM's boot order (or its first disk when it has no order).
func bootDisk(kind string, cfg map[string]any) (string, string) {
	if kind == "lxc" {
		return "rootfs", str(cfg["rootfs"])
	}
	isDisk := func(k string) bool {
		v := str(cfg[k])
		return v != "" && diskKeyRe.MatchString(k) && !strings.Contains(v, "media=cdrom") && !strings.Contains(v, "cloudinit") &&
			!strings.HasPrefix(k, "efidisk") && !strings.HasPrefix(k, "tpmstate")
	}
	if order, ok := strings.CutPrefix(str(cfg["boot"]), "order="); ok {
		for _, k := range strings.Split(order, ";") {
			if isDisk(k) {
				return k, str(cfg[k])
			}
		}
	}
	if k := str(cfg["bootdisk"]); isDisk(k) {
		return k, str(cfg[k])
	}
	for _, k := range []string{"scsi0", "virtio0", "sata0", "ide0"} {
		if isDisk(k) {
			return k, str(cfg[k])
		}
	}
	return "", ""
}
