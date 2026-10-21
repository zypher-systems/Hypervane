# Hypervane roadmap

Path from the shipped daily-ops console to full Proxmox VE UI coverage, then Hypervane-native extras. One cluster until 1.8.

Native PVE exposes about 675 API operations. Version 0.1 covers the jobs operators run every day. Each later version is a shippable slice against a real cluster.

## Release train

| Version | Name | Theme | Status |
|---|---|---|---|
| 0.1 | Daily Ops | Dashboard, guests, storage, tasks, consoles | Shipped |
| 0.2 | Guest Studio | Clone, migrate, disks, cloud-init, uploads, bulk | Shipped |
| 0.3 | Data Protection | Backup jobs, restore, replication | Shipped |
| 0.4 | High Availability | HA resources, groups, rules, relocate | Shipped |
| 0.5 | Identity | Users, ACL, realms, per-user PVE sessions | Shipped |
| 0.6 | Firewall | Datacenter, node, guest, and VNet rules | Shipped |
| 0.7 | Node & Datacenter | Host net, disks, APT, certs, pools, options | Shipped |
| 0.8 | SDN | Zones, VNets, controllers, fabrics, apply | Shipped |
| 0.9 | Ceph | Mons, OSDs, pools, flags, crush | Shipped |
| 1.0 | Parity | Remaining PVE UI + polish on one cluster | Shipped |
| 1.1 | Guest Hardware | Add disk, NIC, CD/DVD, balloon, NUMA, options | Shipped |
| 1.2 | Guest Devices & LXC | PCI/USB, TPM/EFI, Hardware extras, LXC mounts | Shipped |
| 1.3 | Guest Lifecycle | Snapshot RAM, pending revert, agent extras, import | Shipped |
| 1.4 | Node leftovers | Inventory, report, WOL, ZFS, scans, startall | Shipped |
| 1.5 | Datacenter leftovers | Metrics, log, jobs, ACME helpers, OpenID | Shipped |
| 1.6 | Storage leftovers | RBD/CephFS wizards, volume notes, OCI, identity | Shipped |
| 1.7 | Ceph & SDN leftovers | RGW, CRUSH edit, SDN dry-run/DHCP | Shipped |
| 1.8 | Missing stock buttons | Delete, ACL tabs, task abort, HA arm, bulk suspend, SDN maps, Ceph releases, node execute/shells | Shipped |
| 1.9 | Typed wizards | Datacenter options, remaining storage, SDN/realm forms, pool picker, ACME, pending, agent, delete options | Shipped |
| 2.0 | ExtJS hatches | Header search, object History, raw config, tree console/bulk, Support, node Replication | Shipped |
| 2.1 | Disks and backups | move_disk format/delete, backup volume download, PBS namespace | Shipped |
| 2.2 | mdev and capture | PCI mdev picker, VNC canvas screenshot | Shipped |
| 2.3 | Remaining stock gaps | Remotes CRUD, tree migrate network, import remap | Shipped |
| 2.4 | Extra keys and viewers | Advanced extra keys, typed leftover fields, collapsed JSON | Shipped |
| 3.0 | First extras | Top-N dashboard, Community vs Enterprise license | Shipped |
| 3.1 | Pro extras | Pro license, Script Library, audit, long history, MCP read | Shipped |
| 3.2 | Recipes, Scripts, prod | Fedora/Alpine/refresh, Scripts pack, token fields, production images, host unlock | Shipped |
| 3.2.1 | UX parity | Guest tags, dialog a11y, Cmd+K jump palette | Shipped |
| 3.2.2 | Wrong target | Reset entity/live state; tree and console use the row connection | Shipped |
| 3.2.3 | Live hub | Websocket single-writer, watcher unwatch, live ACL filter; hardware refresh on UPID | Shipped |
| 3.2.4 | Task completion | Invalidate and toast when the UPID finishes | Shipped |
| 3.2.5 | Authz and session | MCP/history ACL, audit admin, remote-migrate gate, logout, XSS | Planned |
| 3.2.6 | Freshness and hub load | Summary poll, query keys, empty states, poller and TLS leftovers | Planned |
| 3.3.0 | Calm redesign | Tokens, routed tabs, datacenter overview, insights, token-scoped UI, PVE 9 decode fixes | In progress |

Build order is intentional: finish one cluster before multi-cluster. 3.2.2–3.2.6 is a correctness train (UI glitch + harden), not a feature train. Each sub-point is its own `N.N.N-patch` branch from `dev`.

## 0.1 Daily Ops — shipped

- First-run wizard, local Hypervane admin, encrypted API token
- Live resource tree, dashboard sparklines, RRD charts
- QEMU/LXC create, power, hardware edit, snapshots
- Storage browse/delete, task drawer, noVNC and xterm consoles

Omitted on purpose: clone, live migrate, ISO upload, backups, HA, firewall, SDN, Ceph, users/ACL, node disks, host networking.

## 0.2 Guest Studio — shipped

Close the remaining daily VM/CT gaps so Hypervane replaces the stock guest wizard.

- Clone and convert-to-template
- Live and offline migrate; cluster bulk start/shutdown/migrate
- Disk resize, move, unlink; cloud-init editor; boot order
- ISO and container-template upload; download-from-URL
- Richer create wizard: BIOS/UEFI, machine type, SCSI controller, OS type, cloud-init
- QEMU guest agent actions when the agent is up

## 0.3 Data Protection — shipped

- Scheduled backup jobs (`/cluster/backup`) with include/exclude trees
- Ad-hoc vzdump from a guest (UPID in the existing task drawer)
- Restore a guest from a backup volume; file-level restore browser
- Backup coverage report and not-backed-up guests
- Replication jobs: create, status, log, schedule-now
- Prune / retention UI on storage

## 0.4 High Availability — shipped

- HA resource registration and per-guest HA state on the tree
- HA groups (node affinity) and placement rules
- Relocate / migrate through the HA manager
- Manager status, arm, and disarm

## 0.5 Identity — shipped

- Users, groups, roles, and path-based ACL editor
- Realms: PAM, LDAP/AD, realm sync jobs
- PVE API token management and TFA enrollment
- Per-user Proxmox sessions (ticket pass-through); hide actions the current ACL cannot perform
- Multiple Hypervane operators, not only the first-run admin

## 0.6 Firewall — shipped

- Three-tier rules: datacenter, node, guest — plus SDN VNet on the 0.8 SDN page
- Options, aliases, security groups, IP sets, macros, refs
- Per-level firewall log viewer

## 0.7 Node and datacenter — shipped

- Host network editor: Linux bridge, VLAN, OVS, apply/revert
- Disks: list, LVM/LVM-thin, directory, ZFS create/destroy
- APT repositories, updates, and node services
- DNS, timezone, syslog/journal, subscription status
- Certificates and ACME accounts/plugins
- Resource pools and datacenter options
- Notifications: SMTP, Gotify, webhook, matchers
- PCI / USB / directory mapping for passthrough

## 0.8 SDN — shipped

- Zones, VNets, subnets, IPAM, DNS, controllers
- EVPN/VXLAN fabrics, route maps, prefix lists (tab hidden when PVE has no fabrics)
- Pending summary, apply (UPID), and rollback
- VNet firewall reuses the 0.6 rule editor
- Node runtime view: zone status, neighbors, and routes
- Header and `/sdn` hide when SDN is not installed

## 0.9 Ceph — shipped

- Cluster status, flags, and metadata when Ceph is present
- Install packages and init first mon from the node Ceph tab
- Mon / Mgr / MDS lifecycle
- OSD create from unused disks; in/out, scrub, destroy
- Pools (including `add_storages`), CRUSH viewer, rules, CephFS
- Config and Ceph log viewers
- Header stays visible so an operator can install; cluster tabs wait for a live FSID

## 1.0 Single-cluster parity — shipped

Close leftover ExtJS escape hatches on one cluster.

- Guest CPU models, display/SPICE, QEMU monitor, and sendkey
- Datacenter storage plugins: PBS, NFS/CIFS/iSCSI scan wizards (scan hides when the node returns 404)
- Cluster create/join, nodes, and QDevice (join/qdevice hide when unsupported)
- ACME and notification edit, targets, typed Gotify/webhook fields
- OpenAPI 1.0.0; ticket still never reaches the browser

## 1.1 Guest Hardware — shipped

Close the stock Hardware and Options tabs on one cluster.

- Add disk wizard (new volume, unused attach, cloud-init disk) and unused list
- Guest disk rescan (404-tolerant)
- NIC editor for QEMU and LXC (bridge, VLAN, rate, firewall, LXC IP)
- CD/DVD change, eject, extra ide/sata slots
- Balloon, memory shares, NUMA flag, CPU affinity/limit/units, hotplug
- Options: onboot, startup, protection, hookscript, keyboard, SMBIOS, freeze, kvm, acpi, localtime
- Config digest on guest updates; pending changes on the Hardware tab
- Unlink asks for confirm; ACL gates for Disk / Network / Memory / Options

## 1.2 Guest Devices and LXC — shipped

- PCI and USB passthrough (mapping or raw host device from node hardware inventory)
- TPM, EFI disk, pre-enrolled keys / secure boot
- Audio, watchdog, RNG, serial, virtiofs
- LXC features, nesting, mount points, swap, cmode, tty, timezone

## 1.3 Guest Lifecycle — shipped

- Snapshot RAM toggle and snapshot description edit
- Pending revert (DELETE pending + digest)
- Feature check to hide snapshot/clone when storage cannot
- Cloud-init dump, regenerate, and cicustom
- Guest agent: osinfo, nics, filesystems, users, exec, freeze/thaw
- Import disk from a storage volume or URL (import-from / metadata)

## 1.4 Node leftovers — shipped

Close the remaining stock node escape hatches on one cluster.

- Hardware inventory tab (PCI/USB)
- System report and Wake-on-LAN
- Node description / notes
- ZFS status, scrub, and import; init GPT
- Scan LVM, LVM-thin, ZFS, and PBS
- Node startall / stopall / suspendall / migrateall
- Appliance list (aplinfo) in the create wizard

## 1.5 Datacenter leftovers — shipped

Close the remaining stock datacenter, identity, and backup escape hatches on one cluster.

- Metric servers (Influx / Graphite)
- Cluster log
- Jobs: realm-sync schedule and schedule-analyze
- ACME directories, meta, TOS, challenge-schema
- Notify sendmail endpoint and target test
- Custom CPU models
- vzdump defaults
- OpenID realm (PVE SSO into the cluster)
- Unlock TFA
- Datacenter notes

## 1.6 Storage leftovers — shipped

Close the remaining stock storage escape hatches on one cluster. PBS scan already shipped in 1.0/1.4.

- RBD, CephFS, and ZFS-over-iSCSI wizards
- Volume notes
- Import metadata in the storage browser and OCI pull
- Storage identity (PBS / encrypted)

## 1.7 Ceph and SDN leftovers — shipped

Close the last stock Ceph and SDN escape hatches on one cluster.

- RGW probe (hidden when PVE has no radosgw path)
- CRUSH rule assign on pools; map edit via `PUT /ceph/crush`
- Start / stop / cmd-safety
- OSD metadata, LV info, pool status, cluster metadata
- SDN dry-run and lock
- DHCP DNS on subnets, IPAM status, fabric interfaces

## 1.8 Missing stock buttons — shipped

Click paths for the nine stock ExtJS jobs that had no Hypervane button.

- Guest remove with purge / force / destroy-unreferenced-disks
- Permissions tabs on VM, node, storage, and pool
- Abort a running task
- HA manager arm / disarm
- Bulk suspend
- Global SDN route-maps and prefix-lists plus ordered entries
- Ceph releases dropdown and cluster/node restart-bulk
- Node execute (JSON API batch)
- Node VNC / SPICE shell

## 1.9 Typed wizards — shipped

Replace leftover JSON / extra-keys with typed stock-parity editors so an operator who does not already know PVE field names can leave ExtJS.

- Datacenter options (keyboard, language, tags, next-id, U2F, WebAuthn)
- Remaining storage wizards: dir, lvm, lvmthin, zfspool, btrfs, esxi, plus typed StorageEdit
- Typed SDN zone, controller, IPAM, DNS, fabric, and fabric-node PUT
- LDAP / AD / OpenID realm create and edit
- Pool member picker, ACME challenge-schema form, schedule-analyze table
- Pending guest config inline edit and structured guest-agent tables
- Replication delete keep/force/remove_job and HA unregister purge
- Readable Ceph status, config, and CRUSH panels

## 2.0 ExtJS hatches — shipped

Leave ExtJS for the five stock hatches plus a node Replication tab.

- Header inventory search and `/` shortcut (client-side; opens the tree)
- Object History on guest, node, and storage (`GET /nodes/{node}/tasks` archive)
- Raw qemu/lxc config editor (digest-guarded PUT)
- Tree Console and Ctrl/Meta multi-select bulk (start/stop/suspend/migrate)
- Datacenter Support rollup (`nodeSubscription` per node)
- Node Replication tab (filter jobs where `source` is this node)
- OpenAPI 2.0.0

## 2.1 Disks and backups — shipped

- Disk Move dialog: storage, format (raw/qcow2/vmdk), delete-source
- Streamed backup volume download (`GET /storage/{storage}/download`); hidden on PBS
- PBS namespace on storage create
- OpenAPI 2.1.0

## 2.2 mdev and capture — shipped

- PCI dialog mdev types (`GET /nodes/{node}/hardware/pci/{id}/mdev`, 404-tolerant)
- Console Capture PNG from the noVNC canvas (running QEMU + VNC)
- OpenAPI 2.2.0

## 2.3 Remaining stock gaps — shipped

- PVE remotes create/edit/delete (`/cluster/sdn/remotes`, 404-tolerant) plus `target-endpoint` on remote migrate
- Tree migrate dialog: migration network
- Import wizard: per-disk slot and storage remap
- OpenAPI 2.3.0

## 2.4 Extra keys and viewers — shipped

Last stock polish. Daily ExtJS is not required.

- Extra keys on datacenter options, storage create/edit, and storage identity sit behind Advanced
- Typed leftover fields: description, fencing, notify, storage nodes, encryption-key, crypt-mode
- Cluster config, join info, SDN status, Ceph metadata/CRUSH/OSD, SMART/ZFS, and import metadata keep typed tables; raw JSON is Advanced only

## 3.0 First extras — shipped

Hypervane-native extras on one cluster. Stock PVE ops stay Community.

- Top 10 (`/top`): CPU, Memory, Storage (Usage / Read-Write / IOPS), Network with timeframe in/out totals; click through to the guest
- Signed Ed25519 license (`hvlic1.…`). Community is the default; Pro and Enterprise are verified on the API
- Settings shows edition, org, and expiry. `GET /auth/me` includes `license`
- `requireEntitlement` returns 402. Stock routes and Top-N stay ungated
- OpenAPI 3.0.0

## 3.1 Pro extras — shipped

Pro sits between Community and Enterprise. Fleet stays tabled.

- `pro` edition and remapped entitlements. Enterprise remains a superset.
- Script Library (`/scripts`): Debian 13 / Ubuntu 24.04 / Ubuntu 26.04 cloud templates (Community). AlmaLinux 9 (Pro). Rocky 9 (Enterprise). Forms preview REST steps; the API runs download → create → resize → template via UPID.
- Audit log (`/audit`) of mutating BFF routes
- Long history (`/history`) from Hypervane metric samples plus year-window IOPS
- Read-only MCP (`/mcp`) over Hypervane JSON — no PVE, no writes
- OpenAPI 3.1.0

## 3.2 Recipes, Scripts, and production — shipped

- Community Fedora Cloud 42 and Alpine 3.22 templates. Pro refresh-from-upstream.
- Scripts category: snapshot a node, backup unprotected, enable QEMU agent, convert to template. Review params, then REST + UPID.
- Connection token is user + API key (full `user@realm!id=secret` paste still works)
- Production `api` and `web` images (from 3.3.0, production builds from the GitHub checkout; CI waits until Hypervane is sold)
- `HYPERVANE_LICENSE_UNLOCK=enterprise` lifted edition gates on a self-hosted host (removed in 3.3.0: paid features come only from a verified license).

## 3.2.1 UX parity — shipped

Close leftover stock UX gaps without touching cluster targeting.

- Guest tags in the tree and guest header, plus a typed tag editor
- Dialog keyboard, focus trap, and ARIA
- Cmd+K / Ctrl+K jump palette

## 3.2.2 Wrong target — shipped

Never act on the wrong guest, node, or cluster. Shipped via `3.2.2-patch`.

- Close manage panels and clear live socket state when the route id or connection changes; key panels by entity (HV-016, HV-003, HV-004, HV-018, HV-023–025)
- Move Remove purge / destroy-disks / force into the dialog (HV-019)
- Clear tree selection on connection change and after migrate (HV-021)
- Select the tree row connection before opening the guest context menu (`tree-cluster`)
- Pass `?connection=` on console websocket URLs (`console-conn`)

## 3.2.3 Live hub — shipped

The websocket hub must not panic, leak watchers, or leak the full task list. Hardware UPID actions wait for the task before refetching. Shipped via `3.2.3-patch`.

- Single-writer websocket loop (`ws-writer`)
- Unwatch every topic on disconnect (`watch-leak`)
- Filter the published `{connection, tasks}` shape and `task:` / `node:` / `rrd:` topics (`live-acl`)
- `attachTask(upid, keys)` invalidates guest hardware after resize/move/rescan finish so the next edit uses a fresh digest

## 3.2.4 Task completion — shipped

Invalidate and toast when the Proxmox UPID finishes on the remaining call sites. Shipped via `3.2.4-patch`. Hardware resize already waits; this cut spreads that to snapshots and the other task-backed actions.

- Pass query keys from the remaining `attachTask` sites so a finished `exitstatus` refreshes the panel that queued the work (HV-001, HV-002, HV-017)
- Toast start from `attachTask` when keys are registered; the existing watcher toasts the result (HV-006)

## 3.2.5 Authz and session — planned

Gates, logout, and XSS. Branch `3.2.5-patch`. No guest writes.

- ACL on MCP and history (`mcp-acl`)
- Admin-only audit (`audit-scope`)
- Paid gate on remote-migrate (`remote-migrate-gate`; done in 3.3.0 as Pro)
- Hide PVE URLs and TLS mode when unauthenticated (`auth-topology`)
- Login backoff and body cap (`login-abuse`)
- LiveSocket stop on logout; clear the query cache (`livesocket`, `logout-cache`)
- Safe MetricChart tooltips (`metric-xss`)
- Route error boundary and 401 bounce (`shell-errors`)

## 3.2.6 Freshness and hub load — planned

Remaining stale UI, then leftover realtime majors. Branch `3.2.6-patch`.

- Central `["summary"]` interval and query keys that include node/type (HV-005, HV-007)
- Shared EmptyState that takes query status (HV-008, HV-026)
- Notes draft reset, storage filter reset, formatting and copy (HV-009–015, HV-020, HV-022)
- Console write deadlines, task-log resume, poller workers, inventory change-suppression, system-CA TLS verify, audit quality
- Minors after this cut. Alerts stay the first Pro hole once the window closes.

## 3.3.0 Calm redesign — in progress

Branch `3.3.0-redesign`. Direction "A refined" from the design canvas: quiet by default, one accent for the primary action, and color only where something needs a decision.

- Foundation: design tokens, neutral buttons and badges (stopped is gray, not red), shared Tabs, Segmented, and Meter, one chart palette
- Navigation: Datacenter root in the tree, one header and tab strip for every datacenter page, routed `?tab=` panels instead of stacked modals, ⌘K finds pages and settings
- Overview at `/`: health line, insights, "if a node fails" capacity, nodes, notes, cluster charts
- Insights (`GET /api/insights`, `internal/insights`): quorum, offline nodes, HA errors, last night's backup failures from vzdump logs, stale locks, storage and certificate thresholds, pending restarts, guests outside backup jobs, old snapshots, long-stopped guests, failover memory; viewer-scoped and cached
- Local admins see only what the connection token may do, with a read-only notice
- Licensing and edition gates are removed for the open-source release. Audit, long history, MCP, every recipe, and remote migrate are included. There is no license key and no licensing server. `fleet` `sso` `rbac` `mcp_write` are still unbuilt and gate nothing.
- Recipes reworked: every recipe plans its exact steps before running (the preview is the plan), validates inputs against the cluster, asks before destructive runs, and reports each run step by step (`/scripts/jobs`). Fixed: Ubuntu images saved as `.img` (Proxmox refuses them for import), Fedora 42 gone from the mirror (now Fedora 44, newest build looked up), re-running a template recipe failing on the earlier download, refresh deleting the template before the download succeeded, SSH keys rejected by Proxmox (form-encoded `+`), containers sent `sshkeys` instead of `ssh-public-keys`. New: Debian 12, Ubuntu 22.04, openSUSE Leap 16.0, Arch, AlmaLinux 10, Rocky 10, CentOS Stream 10, deploy from template, deploy several, check updates, shut down / start a node's guests, move all guests off a node, schedule backups for unprotected guests, clean up old snapshots
- Fixes found against a live PVE 9 cluster: double-encoded path IDs (task stop, HA, users), 50-line task logs, and response types for cluster config, certificates, APT repositories, LVM, replication status, prune preview, users, OpenID, ACME schema, IPAM status, vzdump defaults; PVE 9 HA groups and datacenter firewall log

## Tabled after 3.1

- Fleet inventory, fleet-level `remote_migrate`
- Hypervane SSO and native RBAC
- MCP writes

## Constraints that do not change

- The browser never holds a PVE ticket or token
- Mutations stay REST and UPID-backed
- Live inventory and task logs stay on the WebSocket hub
- Consoles terminate on the API
- `PVEClient` stays injectable so 0.5 can swap the service token for a per-user ticket
