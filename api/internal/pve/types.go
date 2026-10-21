package pve

type Version struct {
	Version string `json:"version"`
	Release string `json:"release"`
	Repoid  string `json:"repoid"`
}

type Resource struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Node       string  `json:"node,omitempty"`
	Name       string  `json:"name,omitempty"`
	Storage    string  `json:"storage,omitempty"`
	VMID       int     `json:"vmid,omitempty"`
	Status     string  `json:"status,omitempty"`
	CPU        float64 `json:"cpu"`
	MaxCPU     float64 `json:"maxcpu"`
	Mem        int64   `json:"mem"`
	MaxMem     int64   `json:"maxmem"`
	Disk       int64   `json:"disk"`
	MaxDisk    int64   `json:"maxdisk"`
	Uptime     int64   `json:"uptime"`
	Template   int     `json:"template"`
	Pool       string  `json:"pool,omitempty"`
	Tags       string  `json:"tags,omitempty"`
	HAState    string  `json:"hastate,omitempty"`
	NetIn      int64   `json:"netin"`
	NetOut     int64   `json:"netout"`
	DiskRead   int64   `json:"diskread"`
	DiskWrite  int64   `json:"diskwrite"`
	Level      string  `json:"level,omitempty"`
	Content    string  `json:"content,omitempty"`
	Plugintype string  `json:"plugintype,omitempty"`
	Pending    bool    `json:"pending,omitempty"`
	Lock       string  `json:"lock,omitempty"`
}

type ClusterStatus struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Type    string `json:"type"`
	IP      string `json:"ip,omitempty"`
	Online  int    `json:"online"`
	Local   int    `json:"local"`
	NodeID  int    `json:"nodeid"`
	Quorate int    `json:"quorate"`
	Nodes   int    `json:"nodes"`
	Version int    `json:"version"`
	Level   string `json:"level,omitempty"`
}

type NodeStatus struct {
	CPU        float64        `json:"cpu"`
	Idle       float64        `json:"idle"`
	Memory     MemoryStatus   `json:"memory"`
	RootFS     SpaceStatus    `json:"rootfs"`
	Swap       SpaceStatus    `json:"swap"`
	Uptime     int64          `json:"uptime"`
	LoadAvg    []string       `json:"loadavg"`
	KVersion   string         `json:"kversion"`
	PVEVersion string         `json:"pveversion"`
	CPUInfo    map[string]any `json:"cpuinfo"`
	Wait       float64        `json:"wait"`
	KSMstate   map[string]any `json:"ksm"`
}

type MemoryStatus struct {
	Used  int64 `json:"used"`
	Free  int64 `json:"free"`
	Total int64 `json:"total"`
}

type SpaceStatus struct {
	Used  int64 `json:"used"`
	Free  int64 `json:"free"`
	Total int64 `json:"total"`
	Avail int64 `json:"avail"`
}

type GuestStatus struct {
	Status    string                      `json:"status"`
	VMID      int                         `json:"vmid"`
	Name      string                      `json:"name,omitempty"`
	CPU       float64                     `json:"cpu"`
	Cpus      float64                     `json:"cpus"`
	Mem       int64                       `json:"mem"`
	MaxMem    int64                       `json:"maxmem"`
	Disk      int64                       `json:"disk"`
	MaxDisk   int64                       `json:"maxdisk"`
	Uptime    int64                       `json:"uptime"`
	PID       int                         `json:"pid,omitempty"`
	QMPStatus string                      `json:"qmpstatus,omitempty"`
	NetIn     int64                       `json:"netin"`
	NetOut    int64                       `json:"netout"`
	DiskRead  int64                       `json:"diskread"`
	DiskWrite int64                       `json:"diskwrite"`
	BlockStat map[string]BlockDeviceStats `json:"blockstat,omitempty"`
	HA        map[string]any              `json:"ha,omitempty"`
	Agent     int                         `json:"agent,omitempty"`
	Spice     int                         `json:"spice,omitempty"`
}

type BlockDeviceStats struct {
	RdOperations     float64 `json:"rd_operations"`
	WrOperations     float64 `json:"wr_operations"`
	RdTotalTimeNS    float64 `json:"rd_total_time_ns"`
	WrTotalTimeNS    float64 `json:"wr_total_time_ns"`
	FlushTotalTimeNS float64 `json:"flush_total_time_ns"`
}

type PendingChange struct {
	Key     string `json:"key"`
	Value   any    `json:"value,omitempty"`
	Pending any    `json:"pending,omitempty"`
	Delete  int    `json:"delete,omitempty"`
}

type Snapshot struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	SnapTime    int64  `json:"snaptime,omitempty"`
	VMState     int    `json:"vmstate,omitempty"`
	Parent      string `json:"parent,omitempty"`
}

type Task struct {
	UPID      string `json:"upid"`
	Node      string `json:"node"`
	PID       int    `json:"pid"`
	PStart    int64  `json:"pstart"`
	StartTime int64  `json:"starttime"`
	Type      string `json:"type"`
	ID        string `json:"id"`
	User      string `json:"user"`
	Status    string `json:"status,omitempty"`
	EndTime   int64  `json:"endtime,omitempty"`
}

type TaskStatus struct {
	Status     string `json:"status"`
	ExitStatus string `json:"exitstatus,omitempty"`
	UPID       string `json:"upid,omitempty"`
	Type       string `json:"type,omitempty"`
	ID         string `json:"id,omitempty"`
	User       string `json:"user,omitempty"`
	Node       string `json:"node,omitempty"`
	PID        int    `json:"pid,omitempty"`
	StartTime  int64  `json:"starttime,omitempty"`
}

type TaskLogLine struct {
	N int    `json:"n"`
	T string `json:"t"`
}

type Storage struct {
	Storage string `json:"storage"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Shared  int    `json:"shared,omitempty"`
	Disable int    `json:"disable,omitempty"`
	Path    string `json:"path,omitempty"`
	Nodes   string `json:"nodes,omitempty"`
}

type StorageContent struct {
	VolID   string `json:"volid"`
	Content string `json:"content"`
	Format  string `json:"format,omitempty"`
	Size    int64  `json:"size"`
	CTime   int64  `json:"ctime,omitempty"`
	VMID    int    `json:"vmid,omitempty"`
	Notes   string `json:"notes,omitempty"`
}

type RRDPoint map[string]any

type VNCProxy struct {
	Port   string `json:"port"`
	Ticket string `json:"ticket"`
	UPID   string `json:"upid,omitempty"`
	User   string `json:"user,omitempty"`
	Cert   string `json:"cert,omitempty"`
}

type TermProxy struct {
	Port   string `json:"port"`
	Ticket string `json:"ticket"`
	UPID   string `json:"upid,omitempty"`
	User   string `json:"user,omitempty"`
}

type SpiceProxy struct {
	Port     string `json:"port"`
	Ticket   string `json:"ticket"`
	Password string `json:"password,omitempty"`
	TLSPort  int    `json:"tls-port,omitempty"`
	Host     string `json:"host,omitempty"`
	Type     string `json:"type,omitempty"`
	UPID     string `json:"upid,omitempty"`
	User     string `json:"user,omitempty"`
}

type DeleteGuestOptions struct {
	Purge                    bool
	Force                    bool
	DestroyUnreferencedDisks bool
}

type CreateGuestRequest struct {
	Node                 string            `json:"node"`
	Type                 string            `json:"type"`
	VMID                 int               `json:"vmid"`
	Name                 string            `json:"name"`
	Memory               int               `json:"memory"`
	Cores                int               `json:"cores"`
	DiskGB               int               `json:"diskGb"`
	Storage              string            `json:"storage"`
	Bridge               string            `json:"bridge"`
	ISO                  string            `json:"iso,omitempty"`
	OSType               string            `json:"ostype,omitempty"`
	Template             string            `json:"template,omitempty"`
	Password             string            `json:"password,omitempty"`
	Bios                 string            `json:"bios,omitempty"`
	Machine              string            `json:"machine,omitempty"`
	SCSIHW               string            `json:"scsihw,omitempty"`
	CPU                  string            `json:"cpu,omitempty"`
	Sockets              int               `json:"sockets,omitempty"`
	VGA                  string            `json:"vga,omitempty"`
	Tablet               *bool             `json:"tablet,omitempty"`
	Args                 string            `json:"args,omitempty"`
	Agent                bool              `json:"agent,omitempty"`
	CIUser               string            `json:"ciuser,omitempty"`
	SSHKeys              string            `json:"sshkeys,omitempty"`
	IPConfig0            string            `json:"ipconfig0,omitempty"`
	Unprivileged         *bool             `json:"unprivileged,omitempty"`
	CreateArgs           map[string]any    `json:"createArgs,omitempty"`
	ImportFrom           map[string]string `json:"importFrom,omitempty"`
	ImportDest           map[string]string `json:"importDest,omitempty"`
	ImportWorkingStorage string            `json:"importWorkingStorage,omitempty"`
	// DiskOptions are appended to each imported disk, e.g. "discard=on".
	DiskOptions string            `json:"diskOptions,omitempty"`
	Nets        map[string]string `json:"nets,omitempty"`
}

type CloneRequest struct {
	NewID    int    `json:"newid"`
	Name     string `json:"name"`
	Target   string `json:"target,omitempty"`
	Full     bool   `json:"full"`
	Storage  string `json:"storage,omitempty"`
	SnapName string `json:"snapname,omitempty"`
}

type MigrateRequest struct {
	Target           string `json:"target"`
	Online           bool   `json:"online"`
	WithLocalDisks   bool   `json:"withLocalDisks"`
	TargetStorage    string `json:"targetStorage,omitempty"`
	MigrationNetwork string `json:"migrationNetwork,omitempty"`
}

type RemoteMigrateRequest struct {
	Remote         string `json:"remote"`
	TargetEndpoint string `json:"target-endpoint,omitempty"`
	Target         string `json:"target,omitempty"`
	TargetVMID     int    `json:"target-vmid,omitempty"`
	TargetStorage  string `json:"target-storage,omitempty"`
	TargetBridge   string `json:"target-bridge,omitempty"`
	Online         *bool  `json:"online,omitempty"`
	Delete         *bool  `json:"delete,omitempty"`
}

type DiskResizeRequest struct {
	Disk string `json:"disk"`
	Size string `json:"size"`
}

type DiskMoveRequest struct {
	Disk    string `json:"disk"`
	Storage string `json:"storage"`
	Format  string `json:"format,omitempty"`
	Delete  bool   `json:"delete,omitempty"`
	Digest  string `json:"digest,omitempty"`
}

type DiskUnlinkRequest struct {
	IDList string `json:"idlist"`
}

type DiskRescanResult struct {
	Supported bool   `json:"supported"`
	UPID      string `json:"upid,omitempty"`
}

type CloudInitConfig struct {
	CIUser       string `json:"ciuser,omitempty"`
	CIPassword   string `json:"cipassword,omitempty"`
	SSHKeys      string `json:"sshkeys,omitempty"`
	IPConfig0    string `json:"ipconfig0,omitempty"`
	Nameserver   string `json:"nameserver,omitempty"`
	SearchDomain string `json:"searchdomain,omitempty"`
	CICustom     string `json:"cicustom,omitempty"`
	Digest       string `json:"digest,omitempty"`
}

type GuestFeatureResult struct {
	Supported  bool     `json:"supported"`
	HasFeature bool     `json:"hasFeature"`
	Nodes      []string `json:"nodes,omitempty"`
}

type CloudInitDumpResult struct {
	Supported bool   `json:"supported"`
	Dump      string `json:"dump,omitempty"`
}

type ImportMetadataResult struct {
	Supported bool           `json:"supported"`
	Data      map[string]any `json:"data,omitempty"`
}

type SnapshotUpdateRequest struct {
	Description string `json:"description"`
}

type DownloadURLRequest struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Content  string `json:"content"`
	Checksum string `json:"checksum,omitempty"`
}

type BulkGuest struct {
	VMID int    `json:"vmid"`
	Node string `json:"node"`
	Type string `json:"type"`
}

type BulkRequest struct {
	Guests           []BulkGuest `json:"guests"`
	Target           string      `json:"target,omitempty"`
	TargetStorage    string      `json:"targetStorage,omitempty"`
	MigrationNetwork string      `json:"migrationNetwork,omitempty"`
	Online           *bool       `json:"online,omitempty"`
	WithLocalDisks   *bool       `json:"withLocalDisks,omitempty"`
}

type AgentPasswordRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type NodeCapabilities struct {
	Machines []map[string]any `json:"machines"`
	CPUs     []map[string]any `json:"cpus"`
}

type MediaItem struct {
	VolID   string `json:"volid"`
	Content string `json:"content"`
	Storage string `json:"storage"`
	Node    string `json:"node"`
	Size    int64  `json:"size"`
}

type UpdateGuestRequest struct {
	Node   string         `json:"node"`
	Type   string         `json:"type"`
	Digest string         `json:"digest,omitempty"`
	Fields map[string]any `json:"fields"`
	Delete []string       `json:"delete,omitempty"`
}

type SnapshotRequest struct {
	Node        string `json:"node"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	VMState     bool   `json:"vmstate,omitempty"`
}

type BackupJob struct {
	ID                 string `json:"id"`
	Enabled            int    `json:"enabled"`
	Schedule           string `json:"schedule,omitempty"`
	Storage            string `json:"storage,omitempty"`
	Mode               string `json:"mode,omitempty"`
	Compress           string `json:"compress,omitempty"`
	VMID               string `json:"vmid,omitempty"`
	All                int    `json:"all,omitempty"`
	Exclude            string `json:"exclude,omitempty"`
	Node               string `json:"node,omitempty"`
	NotesTemplate      string `json:"notes-template,omitempty"`
	Mailto             string `json:"mailto,omitempty"`
	MailNotification   string `json:"mailnotification,omitempty"`
	NotificationMode   string `json:"notification-mode,omitempty"`
	PruneBackups       string `json:"prune-backups,omitempty"`
	Comment            string `json:"comment,omitempty"`
	Fleecing           string `json:"fleecing,omitempty"`
	Bwlimit            any    `json:"bwlimit,omitempty"`
	PBSChangeDetection string `json:"pbs-change-detection-mode,omitempty"`
	Type               string `json:"type,omitempty"`
}

type BackupJobRequest struct {
	ID                 string            `json:"id,omitempty"`
	Enabled            *bool             `json:"enabled,omitempty"`
	Schedule           string            `json:"schedule,omitempty"`
	Storage            string            `json:"storage,omitempty"`
	Mode               string            `json:"mode,omitempty"`
	Compress           string            `json:"compress,omitempty"`
	VMID               string            `json:"vmid,omitempty"`
	All                bool              `json:"all,omitempty"`
	Exclude            string            `json:"exclude,omitempty"`
	Node               string            `json:"node,omitempty"`
	Notes              string            `json:"notes,omitempty"`
	Mailto             string            `json:"mailto,omitempty"`
	MailNotification   string            `json:"mailnotification,omitempty"`
	NotificationMode   string            `json:"notification-mode,omitempty"`
	PruneBackups       string            `json:"pruneBackups,omitempty"`
	Comment            string            `json:"comment,omitempty"`
	Fleecing           string            `json:"fleecing,omitempty"`
	Bwlimit            string            `json:"bwlimit,omitempty"`
	PBSChangeDetection string            `json:"pbsChangeDetection,omitempty"`
	Extra              map[string]string `json:"extra,omitempty"`
}

type IncludedVolume struct {
	ID       any              `json:"id"`
	Type     string           `json:"type,omitempty"`
	Name     string           `json:"name,omitempty"`
	Included int              `json:"included,omitempty"`
	Leaf     int              `json:"leaf,omitempty"`
	Children []IncludedVolume `json:"children,omitempty"`
}

type CoverageGuest struct {
	VMID int    `json:"vmid"`
	Name string `json:"name,omitempty"`
	Type string `json:"type"`
}

type GuestBackupInfo struct {
	VMID       int    `json:"vmid"`
	Name       string `json:"name,omitempty"`
	Type       string `json:"type"`
	Node       string `json:"node,omitempty"`
	LastBackup int64  `json:"lastBackup,omitempty"`
	LastVolID  string `json:"lastVolid,omitempty"`
	InJob      bool   `json:"inJob"`
}

type CoverageReport struct {
	NotBackedUp []CoverageGuest   `json:"notBackedUp"`
	Guests      []GuestBackupInfo `json:"guests"`
}

type VZDumpRequest struct {
	VMID int `json:"vmid"`
	// VMIDs backs up several guests of one node in a single task instead of VMID.
	VMIDs              []int             `json:"vmids,omitempty"`
	Storage            string            `json:"storage,omitempty"`
	Mode               string            `json:"mode,omitempty"`
	Compress           string            `json:"compress,omitempty"`
	Notes              string            `json:"notes,omitempty"`
	Bwlimit            string            `json:"bwlimit,omitempty"`
	Fleecing           string            `json:"fleecing,omitempty"`
	Mailto             string            `json:"mailto,omitempty"`
	MailNotification   string            `json:"mailnotification,omitempty"`
	NotificationMode   string            `json:"notification-mode,omitempty"`
	PBSChangeDetection string            `json:"pbsChangeDetection,omitempty"`
	Remove             bool              `json:"remove,omitempty"`
	Protected          bool              `json:"protected,omitempty"`
	Extra              map[string]string `json:"extra,omitempty"`
}

type RestoreRequest struct {
	Node    string `json:"node"`
	Archive string `json:"archive"`
	Type    string `json:"type,omitempty"`
	VMID    int    `json:"vmid"`
	Storage string `json:"storage,omitempty"`
	Unique  bool   `json:"unique,omitempty"`
	Force   bool   `json:"force,omitempty"`
	Start   bool   `json:"start,omitempty"`
}

// pveBool accepts PVE JSON booleans encoded as true/false or 1/0.
type pveBool bool

type FileRestoreEntry struct {
	Filepath string   `json:"filepath"`
	Text     string   `json:"text,omitempty"`
	Type     string   `json:"type,omitempty"`
	Leaf     *pveBool `json:"leaf,omitempty"`
	Size     int64    `json:"size,omitempty"`
	MTime    int64    `json:"mtime,omitempty"`
}

type PrunePreview struct {
	VolID string  `json:"volid"`
	CTime int64   `json:"ctime,omitempty"`
	Mark  string  `json:"mark,omitempty"`
	Type  string  `json:"type,omitempty"`
	VMID  FlexInt `json:"vmid,omitempty"`
	Size  int64   `json:"size,omitempty"`
}

type ReplicationJob struct {
	ID       string  `json:"id"`
	Guest    int     `json:"guest"`
	Target   string  `json:"target"`
	Schedule string  `json:"schedule,omitempty"`
	Type     string  `json:"type,omitempty"`
	Disable  int     `json:"disable,omitempty"`
	Comment  string  `json:"comment,omitempty"`
	Rate     float64 `json:"rate,omitempty"`
	Source   string  `json:"source,omitempty"`
	JobNum   int     `json:"jobnum,omitempty"`
}

type ReplicationRequest struct {
	ID        string `json:"id,omitempty"`
	Guest     int    `json:"guest"`
	Target    string `json:"target"`
	Schedule  string `json:"schedule,omitempty"`
	Type      string `json:"type,omitempty"`
	Comment   string `json:"comment,omitempty"`
	Rate      string `json:"rate,omitempty"`
	Disable   *bool  `json:"disable,omitempty"`
	RemoveJob string `json:"remove_job,omitempty"`
}

type DeleteReplicationOptions struct {
	Force bool
	Keep  bool
}

type ReplicationStatus struct {
	ID        string  `json:"id,omitempty"`
	Guest     int     `json:"guest,omitempty"`
	LastSync  int64   `json:"last_sync,omitempty"`
	Duration  float64 `json:"duration,omitempty"`
	FailCount int     `json:"fail_count,omitempty"`
	Error     string  `json:"error,omitempty"`
	PID       int     `json:"pid,omitempty"`
}

type HACurrent struct {
	Type     string `json:"type"`
	SID      string `json:"sid,omitempty"`
	State    string `json:"state,omitempty"`
	Status   string `json:"status,omitempty"`
	Node     string `json:"node,omitempty"`
	Request  string `json:"request,omitempty"`
	Quorate  int    `json:"quorate,omitempty"`
	CRMState string `json:"crm_state,omitempty"`
	ID       string `json:"id,omitempty"`
}

type HAStatus struct {
	Current []HACurrent `json:"current"`
	Manager any         `json:"manager"`
}

type HAResource struct {
	SID         string `json:"sid"`
	Type        string `json:"type,omitempty"`
	State       string `json:"state,omitempty"`
	Group       string `json:"group,omitempty"`
	MaxRestart  int    `json:"max_restart,omitempty"`
	MaxRelocate int    `json:"max_relocate,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

type HAResourceRequest struct {
	SID         string `json:"sid,omitempty"`
	Type        string `json:"type,omitempty"`
	VMID        int    `json:"vmid,omitempty"`
	State       string `json:"state,omitempty"`
	Group       string `json:"group,omitempty"`
	MaxRestart  *int   `json:"maxRestart,omitempty"`
	MaxRelocate *int   `json:"maxRelocate,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

type HAMoveRequest struct {
	Node string `json:"node"`
}

type HAGroup struct {
	Group      string `json:"group"`
	Nodes      string `json:"nodes,omitempty"`
	Restricted int    `json:"restricted,omitempty"`
	NoFailback int    `json:"nofailback,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

type HAGroupRequest struct {
	Group      string `json:"group,omitempty"`
	Nodes      string `json:"nodes,omitempty"`
	Restricted bool   `json:"restricted,omitempty"`
	NoFailback bool   `json:"nofailback,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

type HARule struct {
	Rule      string `json:"rule"`
	Type      string `json:"type,omitempty"`
	Resources string `json:"resources,omitempty"`
	Nodes     string `json:"nodes,omitempty"`
	Affinity  string `json:"affinity,omitempty"`
	Strict    int    `json:"strict,omitempty"`
	Disable   int    `json:"disable,omitempty"`
	Comment   string `json:"comment,omitempty"`
}

type HARuleRequest struct {
	Rule      string `json:"rule,omitempty"`
	Type      string `json:"type,omitempty"`
	Resources string `json:"resources,omitempty"`
	Nodes     string `json:"nodes,omitempty"`
	Affinity  string `json:"affinity,omitempty"`
	Strict    bool   `json:"strict,omitempty"`
	Disable   bool   `json:"disable,omitempty"`
	Comment   string `json:"comment,omitempty"`
}

type HARulesResponse struct {
	Supported bool     `json:"supported"`
	Rules     []HARule `json:"rules"`
}

type AccessUser struct {
	UserID    string  `json:"userid"`
	Comment   string  `json:"comment,omitempty"`
	Email     string  `json:"email,omitempty"`
	Enable    int     `json:"enable"`
	Expire    int64   `json:"expire,omitempty"`
	Firstname string  `json:"firstname,omitempty"`
	Lastname  string  `json:"lastname,omitempty"`
	Groups    FlexCSV `json:"groups,omitempty"`
	Keys      string  `json:"keys,omitempty"`
	RealmType string  `json:"realm-type,omitempty"`
}

type AccessUserRequest struct {
	UserID    string `json:"userid,omitempty"`
	Password  string `json:"password,omitempty"`
	Comment   string `json:"comment,omitempty"`
	Email     string `json:"email,omitempty"`
	Enable    *int   `json:"enable,omitempty"`
	Expire    *int64 `json:"expire,omitempty"`
	Firstname string `json:"firstname,omitempty"`
	Lastname  string `json:"lastname,omitempty"`
	Groups    string `json:"groups,omitempty"`
}

type AccessToken struct {
	TokenID string `json:"tokenid"`
	Comment string `json:"comment,omitempty"`
	Expire  int64  `json:"expire,omitempty"`
	Privsep int    `json:"privsep,omitempty"`
}

type AccessTokenCreate struct {
	TokenID string `json:"tokenid"`
	Comment string `json:"comment,omitempty"`
	Expire  int64  `json:"expire,omitempty"`
	Privsep *int   `json:"privsep,omitempty"`
	Value   string `json:"value,omitempty"`
}

type TFAEntry struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Enable      int    `json:"enable,omitempty"`
	Created     int64  `json:"created,omitempty"`
	Description string `json:"description,omitempty"`
}

type TFAEnrollRequest struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Password    string `json:"password,omitempty"`
}

type TFAEnrollResponse struct {
	ID       string   `json:"id,omitempty"`
	Type     string   `json:"type,omitempty"`
	URI      string   `json:"uri,omitempty"`
	QRCode   string   `json:"qrcode,omitempty"`
	Recovery []string `json:"recovery,omitempty"`
}

type AccessGroup struct {
	Group   string `json:"group"`
	Comment string `json:"comment,omitempty"`
	Users   string `json:"users,omitempty"`
}

type AccessRole struct {
	RoleID string `json:"roleid"`
	Privs  string `json:"privs,omitempty"`
}

type ACLEntry struct {
	Path      string `json:"path"`
	UGID      string `json:"ugid"`
	Type      string `json:"type"`
	RoleID    string `json:"roleid"`
	Propagate int    `json:"propagate,omitempty"`
}

type ACLRequest struct {
	Path      string `json:"path"`
	Roles     string `json:"roles"`
	Users     string `json:"users,omitempty"`
	Groups    string `json:"groups,omitempty"`
	Propagate *int   `json:"propagate,omitempty"`
	Delete    bool   `json:"delete,omitempty"`
}

type Realm struct {
	Realm   string `json:"realm"`
	Type    string `json:"type"`
	Comment string `json:"comment,omitempty"`
	Default int    `json:"default,omitempty"`
	TFA     string `json:"tfa,omitempty"`
}

type FWRule struct {
	Pos     int    `json:"pos"`
	Action  string `json:"action,omitempty"`
	Type    string `json:"type,omitempty"`
	Enable  int    `json:"enable"`
	Source  string `json:"source,omitempty"`
	Dest    string `json:"dest,omitempty"`
	Proto   string `json:"proto,omitempty"`
	DPort   string `json:"dport,omitempty"`
	SPort   string `json:"sport,omitempty"`
	Macro   string `json:"macro,omitempty"`
	Iface   string `json:"iface,omitempty"`
	Comment string `json:"comment,omitempty"`
	Log     string `json:"log,omitempty"`
	Digest  string `json:"digest,omitempty"`
}

type FWRuleRequest struct {
	Action  string `json:"action,omitempty"`
	Type    string `json:"type,omitempty"`
	Enable  *int   `json:"enable,omitempty"`
	Source  string `json:"source,omitempty"`
	Dest    string `json:"dest,omitempty"`
	Proto   string `json:"proto,omitempty"`
	DPort   string `json:"dport,omitempty"`
	SPort   string `json:"sport,omitempty"`
	Macro   string `json:"macro,omitempty"`
	Iface   string `json:"iface,omitempty"`
	Comment string `json:"comment,omitempty"`
	Log     string `json:"log,omitempty"`
	Pos     *int   `json:"pos,omitempty"`
	Digest  string `json:"digest,omitempty"`
}

type FWOptions struct {
	Enable      *int   `json:"enable,omitempty"`
	PolicyIn    string `json:"policy_in,omitempty"`
	PolicyOut   string `json:"policy_out,omitempty"`
	LogLevelIn  string `json:"log_level_in,omitempty"`
	LogLevelOut string `json:"log_level_out,omitempty"`
	Ebtables    *int   `json:"ebtables,omitempty"`
	NDP         *int   `json:"ndp,omitempty"`
	DHCP        *int   `json:"dhcp,omitempty"`
	RADV        *int   `json:"radv,omitempty"`
	MacFilter   *int   `json:"macfilter,omitempty"`
	IPFilter    *int   `json:"ipfilter,omitempty"`
	Digest      string `json:"digest,omitempty"`
}

type FWAlias struct {
	Name    string `json:"name"`
	CIDR    string `json:"cidr,omitempty"`
	Comment string `json:"comment,omitempty"`
	Digest  string `json:"digest,omitempty"`
}

type FWIPSet struct {
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`
	Digest  string `json:"digest,omitempty"`
}

type FWIPSetCIDR struct {
	CIDR    string `json:"cidr"`
	Nomatch int    `json:"nomatch,omitempty"`
	Comment string `json:"comment,omitempty"`
	Digest  string `json:"digest,omitempty"`
}

type FWGroup struct {
	Group   string `json:"group"`
	Comment string `json:"comment,omitempty"`
	Digest  string `json:"digest,omitempty"`
}

type FWMacro struct {
	Macro string `json:"macro"`
	Descr string `json:"descr,omitempty"`
}

type FWRef struct {
	Ref     string `json:"ref"`
	Type    string `json:"type,omitempty"`
	Name    string `json:"name,omitempty"`
	Comment string `json:"comment,omitempty"`
}

type FWRefsResponse struct {
	Supported bool    `json:"supported"`
	Refs      []FWRef `json:"refs"`
}

type FWLogLine struct {
	N int    `json:"n"`
	T string `json:"t"`
}

type FeatureList struct {
	Supported bool             `json:"supported"`
	Items     []map[string]any `json:"items"`
}

type FeatureObject struct {
	Supported bool           `json:"supported"`
	Item      map[string]any `json:"item,omitempty"`
}

type FeatureValue struct {
	Supported bool   `json:"supported"`
	Value     string `json:"value,omitempty"`
}

type NetIface struct {
	Iface       string `json:"iface"`
	Type        string `json:"type,omitempty"`
	Active      int    `json:"active"`
	Address     string `json:"address,omitempty"`
	CIDR        string `json:"cidr,omitempty"`
	Gateway     string `json:"gateway,omitempty"`
	BridgePorts string `json:"bridge_ports,omitempty"`
	Comments    string `json:"comments,omitempty"`
	Method      string `json:"method,omitempty"`
	Autostart   int    `json:"autostart,omitempty"`
	Exists      int    `json:"exists,omitempty"`
	Priority    int    `json:"priority,omitempty"`
}

type DiskInfo struct {
	Devpath string `json:"devpath"`
	Size    int64  `json:"size"`
	Used    string `json:"used,omitempty"`
	GPT     int    `json:"gpt"`
	OsdID   int    `json:"osdid"`
	Model   string `json:"model,omitempty"`
	Serial  string `json:"serial,omitempty"`
	Vendor  string `json:"vendor,omitempty"`
	Type    string `json:"type,omitempty"`
	Health  string `json:"health,omitempty"`
	Wearout any    `json:"wearout,omitempty"`
}

type NodeReport struct {
	Supported bool   `json:"supported"`
	Report    string `json:"report"`
}

type WakeOnLANResult struct {
	Supported bool   `json:"supported"`
	MAC       string `json:"mac,omitempty"`
}

type ZFSPoolStatus struct {
	Supported bool           `json:"supported"`
	Pool      map[string]any `json:"pool,omitempty"`
}

type APTRepoFile struct {
	Path         string           `json:"path,omitempty"`
	Digest       any              `json:"digest,omitempty"`
	Repositories []map[string]any `json:"repositories,omitempty"`
}

type APTRepositories struct {
	Digest string           `json:"digest,omitempty"`
	Files  []APTRepoFile    `json:"files,omitempty"`
	Infos  []map[string]any `json:"infos,omitempty"`
	Errors []map[string]any `json:"errors,omitempty"`
}

type NodeService struct {
	Service string `json:"service"`
	Name    string `json:"name,omitempty"`
	State   string `json:"state,omitempty"`
	Desc    string `json:"desc,omitempty"`
	Unit    string `json:"unit-state,omitempty"`
}

type NodeDNS struct {
	Search string `json:"search,omitempty"`
	DNS1   string `json:"dns1,omitempty"`
	DNS2   string `json:"dns2,omitempty"`
	DNS3   string `json:"dns3,omitempty"`
}

type NodeTime struct {
	Timezone  string `json:"timezone,omitempty"`
	Time      int64  `json:"time,omitempty"`
	Localtime int64  `json:"localtime,omitempty"`
}

type NodeHosts struct {
	Data   string `json:"data,omitempty"`
	Digest string `json:"digest,omitempty"`
}

type JournalLine struct {
	N int    `json:"n"`
	T string `json:"t"`
}

type NodeSubscription struct {
	Status      string `json:"status,omitempty"`
	Level       string `json:"level,omitempty"`
	Key         string `json:"key,omitempty"`
	ServerID    string `json:"serverid,omitempty"`
	Sockets     int    `json:"sockets,omitempty"`
	Checktime   int64  `json:"checktime,omitempty"`
	Message     string `json:"message,omitempty"`
	NextDue     string `json:"nextduedate,omitempty"`
	Productname string `json:"productname,omitempty"`
	Regdate     string `json:"regdate,omitempty"`
}

type CertInfo struct {
	Filename    string `json:"filename,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Issuer      string `json:"issuer,omitempty"`
	Subject     string `json:"subject,omitempty"`
	PublicKey   string `json:"public-key-type,omitempty"`
	San         any    `json:"san,omitempty"`
	NotBefore   int64  `json:"notbefore,omitempty"`
	NotAfter    int64  `json:"notafter,omitempty"`
	Pem         string `json:"pem,omitempty"`
}

type Pool struct {
	PoolID  string           `json:"poolid"`
	Comment string           `json:"comment,omitempty"`
	Members []map[string]any `json:"members,omitempty"`
}

type SDNStatus struct {
	Supported bool           `json:"supported"`
	Data      map[string]any `json:"data,omitempty"`
}

type SDNZone struct {
	Zone    string `json:"zone"`
	Type    string `json:"type,omitempty"`
	Bridge  string `json:"bridge,omitempty"`
	MTU     int    `json:"mtu,omitempty"`
	Nodes   string `json:"nodes,omitempty"`
	IPAM    string `json:"ipam,omitempty"`
	DNS     string `json:"dns,omitempty"`
	Pending any    `json:"pending,omitempty"`
	Digest  string `json:"digest,omitempty"`
}

type SDNVNet struct {
	VNet      string `json:"vnet"`
	Zone      string `json:"zone,omitempty"`
	Tag       int    `json:"tag,omitempty"`
	Alias     string `json:"alias,omitempty"`
	VLANAware any    `json:"vlanaware,omitempty"`
	Pending   any    `json:"pending,omitempty"`
	Digest    string `json:"digest,omitempty"`
}

type SDNSubnet struct {
	Subnet    string `json:"subnet"`
	Gateway   string `json:"gateway,omitempty"`
	SNAT      int    `json:"snat,omitempty"`
	VNet      string `json:"vnet,omitempty"`
	DhcpRange string `json:"dhcp-range,omitempty"`
	DhcpDNS   string `json:"dhcp-dns-server,omitempty"`
	Pending   any    `json:"pending,omitempty"`
	Digest    string `json:"digest,omitempty"`
}

type NodeSDNRuntime struct {
	Supported bool             `json:"supported"`
	Zones     []map[string]any `json:"zones,omitempty"`
	Neighbors []map[string]any `json:"neighbors,omitempty"`
	Routes    []map[string]any `json:"routes,omitempty"`
}

type CephStatus struct {
	Supported bool           `json:"supported"`
	Data      map[string]any `json:"data,omitempty"`
}

type CephFlags struct {
	Supported bool           `json:"supported"`
	Flags     map[string]any `json:"flags,omitempty"`
}

type CephMon struct {
	Name        string `json:"name"`
	Host        string `json:"host,omitempty"`
	Addr        string `json:"addr,omitempty"`
	State       string `json:"state,omitempty"`
	FSID        string `json:"fsid,omitempty"`
	CephVersion string `json:"ceph_version,omitempty"`
}

type CephMgr struct {
	Name  string `json:"name"`
	Host  string `json:"host,omitempty"`
	State string `json:"state,omitempty"`
	Addr  string `json:"addr,omitempty"`
}

type CephMDS struct {
	Name  string `json:"name"`
	Host  string `json:"host,omitempty"`
	State string `json:"state,omitempty"`
	Addr  string `json:"addr,omitempty"`
}

type CephOSD struct {
	OSDID  int     `json:"osdid"`
	ID     int     `json:"id"`
	Name   string  `json:"name,omitempty"`
	Host   string  `json:"host,omitempty"`
	Status string  `json:"status,omitempty"`
	In     int     `json:"in"`
	Weight float64 `json:"weight,omitempty"`
	Class  string  `json:"crush_device_class,omitempty"`
}

type CephPool struct {
	Pool          int    `json:"pool"`
	PoolName      string `json:"pool_name"`
	Size          int    `json:"size,omitempty"`
	MinSize       int    `json:"min_size,omitempty"`
	PGNum         int    `json:"pg_num,omitempty"`
	CrushRule     any    `json:"crush_rule,omitempty"`
	CrushRuleName string `json:"crush_rule_name,omitempty"`
	Application   string `json:"application,omitempty"`
}

type CephCmdSafety struct {
	Supported bool   `json:"supported"`
	Safe      bool   `json:"safe"`
	Status    string `json:"status,omitempty"`
}

type CephFS struct {
	Name string `json:"name"`
	Data string `json:"data,omitempty"`
	Meta string `json:"metadata,omitempty"`
}
