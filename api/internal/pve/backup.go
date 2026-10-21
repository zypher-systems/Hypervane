package pve

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

func (c *Client) BackupJobs(ctx context.Context) ([]BackupJob, error) {
	var out []BackupJob
	err := c.do(ctx, http.MethodGet, "/cluster/backup", nil, &out)
	return out, err
}

func (j *BackupJob) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	j.ID = flexString(raw["id"])
	j.Enabled = flexBoolInt(raw["enabled"], 1)
	j.Schedule = flexString(raw["schedule"])
	j.Storage = flexString(raw["storage"])
	j.Mode = flexString(raw["mode"])
	j.Compress = flexString(raw["compress"])
	j.VMID = flexString(raw["vmid"])
	j.All = flexBoolInt(raw["all"], 0)
	j.Exclude = flexString(raw["exclude"])
	j.Node = flexString(raw["node"])
	j.NotesTemplate = flexString(firstRaw(raw, "notes-template", "notes_template"))
	j.Mailto = flexString(raw["mailto"])
	j.MailNotification = flexString(raw["mailnotification"])
	j.NotificationMode = flexString(firstRaw(raw, "notification-mode", "notification_mode"))
	j.PruneBackups = flexString(firstRaw(raw, "prune-backups", "prune_backups"))
	j.Comment = flexString(raw["comment"])
	j.Fleecing = flexString(raw["fleecing"])
	j.Bwlimit = raw["bwlimit"]
	j.PBSChangeDetection = flexString(firstRaw(raw, "pbs-change-detection-mode", "pbs_change_detection_mode"))
	j.Type = flexString(raw["type"])
	return nil
}

func firstRaw(raw map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := raw[key]; ok && v != nil {
			return v
		}
	}
	return nil
}

func flexString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "1"
		}
		return "0"
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s := flexString(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ",")
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+"="+flexString(t[k]))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(t)
	}
}

func flexBoolInt(v any, empty int) int {
	switch t := v.(type) {
	case nil:
		return empty
	case bool:
		if t {
			return 1
		}
		return 0
	case float64:
		if t != 0 {
			return 1
		}
		return 0
	case json.Number:
		n, err := t.Int64()
		if err != nil || n == 0 {
			return 0
		}
		return 1
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		if s == "" {
			return empty
		}
		if s == "0" || s == "false" || s == "no" || s == "off" {
			return 0
		}
		return 1
	default:
		return empty
	}
}

func (c *Client) BackupJob(ctx context.Context, id string) (BackupJob, error) {
	var out BackupJob
	err := c.do(ctx, http.MethodGet, "/cluster/backup/"+url.PathEscape(id), nil, &out)
	return out, err
}

func (c *Client) CreateBackupJob(ctx context.Context, req BackupJobRequest) (BackupJob, error) {
	if req.ID == "" {
		req.ID = newBackupJobID()
	}
	form := backupJobForm(req)
	form.Set("id", req.ID)
	if err := c.do(ctx, http.MethodPost, "/cluster/backup", form, nil); err != nil {
		return BackupJob{}, err
	}
	return c.BackupJob(ctx, req.ID)
}

func (c *Client) UpdateBackupJob(ctx context.Context, id string, req BackupJobRequest) error {
	form := backupJobForm(req)
	return c.do(ctx, http.MethodPut, "/cluster/backup/"+url.PathEscape(id), form, nil)
}

func (c *Client) DeleteBackupJob(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/cluster/backup/"+url.PathEscape(id), nil, nil)
}

func (c *Client) IncludedVolumes(ctx context.Context, id string) (IncludedVolume, error) {
	var out IncludedVolume
	err := c.do(ctx, http.MethodGet, "/cluster/backup/"+url.PathEscape(id)+"/included_volumes", nil, &out)
	return out, err
}

func (c *Client) NotBackedUp(ctx context.Context) ([]CoverageGuest, error) {
	var out []CoverageGuest
	err := c.do(ctx, http.MethodGet, "/cluster/backup-info/not-backed-up", nil, &out)
	return out, err
}

func (c *Client) VZDump(ctx context.Context, node string, req VZDumpRequest) (string, error) {
	ids := []string{strconv.Itoa(req.VMID)}
	if len(req.VMIDs) > 0 {
		ids = ids[:0]
		for _, id := range req.VMIDs {
			ids = append(ids, strconv.Itoa(id))
		}
	}
	form := url.Values{"vmid": {strings.Join(ids, ",")}}
	if req.Storage != "" {
		form.Set("storage", req.Storage)
	}
	if req.Mode != "" {
		form.Set("mode", req.Mode)
	}
	if req.Compress != "" {
		form.Set("compress", req.Compress)
	}
	if req.Notes != "" {
		form.Set("notes", req.Notes)
	}
	if req.Bwlimit != "" {
		form.Set("bwlimit", req.Bwlimit)
	}
	if req.Fleecing != "" {
		form.Set("fleecing", req.Fleecing)
	}
	if req.Mailto != "" {
		form.Set("mailto", req.Mailto)
	}
	if req.MailNotification != "" {
		form.Set("mailnotification", req.MailNotification)
	}
	if req.NotificationMode != "" {
		form.Set("notification-mode", req.NotificationMode)
	}
	if req.PBSChangeDetection != "" {
		form.Set("pbs-change-detection-mode", req.PBSChangeDetection)
	}
	if req.Remove {
		form.Set("remove", "1")
	}
	if req.Protected {
		form.Set("protected", "1")
	}
	for k, v := range req.Extra {
		if v != "" {
			form.Set(k, v)
		}
	}
	var upid string
	err := c.do(ctx, http.MethodPost, "/nodes/"+url.PathEscape(node)+"/vzdump", form, &upid)
	return upid, err
}

func (c *Client) VZDumpDefaults(ctx context.Context, node, storage string) (FeatureObject, error) {
	q := url.Values{}
	if storage != "" {
		q.Set("storage", storage)
	}
	path := "/nodes/" + url.PathEscape(node) + "/vzdump/defaults"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	return c.optionalObject(ctx, path)
}

func (c *Client) UpdateVZDumpDefaults(ctx context.Context, node string, fields map[string]any) (DiskRescanResult, error) {
	err := c.do(ctx, http.MethodPut, "/nodes/"+url.PathEscape(node)+"/vzdump/defaults", mapForm(fields), nil)
	if IsNotFound(err) {
		return DiskRescanResult{Supported: false}, nil
	}
	if err != nil {
		return DiskRescanResult{}, err
	}
	return DiskRescanResult{Supported: true}, nil
}

func (c *Client) ExtractBackupConfig(ctx context.Context, node, volume string) (string, error) {
	q := url.Values{"volume": {volume}}
	var raw any
	err := c.do(ctx, http.MethodGet, "/nodes/"+url.PathEscape(node)+"/vzdump/extractconfig?"+q.Encode(), nil, &raw)
	if err != nil {
		return "", err
	}
	switch v := raw.(type) {
	case string:
		return v, nil
	default:
		return fmt.Sprint(v), nil
	}
}

func (c *Client) RestoreGuest(ctx context.Context, req RestoreRequest) (string, error) {
	form := url.Values{"vmid": {strconv.Itoa(req.VMID)}}
	if req.Storage != "" {
		form.Set("storage", req.Storage)
	}
	if req.Unique {
		form.Set("unique", "1")
	}
	if req.Force {
		form.Set("force", "1")
	}
	if req.Start {
		form.Set("start", "1")
	}
	kind := kindOrQEMU(req.Type)
	if kind == "lxc" {
		form.Set("ostemplate", req.Archive)
		form.Set("restore", "1")
	} else {
		form.Set("archive", req.Archive)
	}
	var upid string
	err := c.do(ctx, http.MethodPost, "/nodes/"+url.PathEscape(req.Node)+"/"+kind, form, &upid)
	return upid, err
}

func (c *Client) FileRestoreList(ctx context.Context, node, storage, volume, filepath string) ([]FileRestoreEntry, error) {
	q := url.Values{"volume": {backupVolume(storage, volume)}, "filepath": {fileRestorePath(filepath)}}
	path := "/nodes/" + url.PathEscape(node) + "/storage/" + url.PathEscape(storage) + "/file-restore/list?" + q.Encode()
	var out []FileRestoreEntry
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

func (c *Client) FileRestoreDownload(ctx context.Context, node, storage, volume, filepath string) (*http.Response, error) {
	q := url.Values{"volume": {backupVolume(storage, volume)}, "filepath": {fileRestorePath(filepath)}}
	full, err := url.Parse(c.base.String() + "/nodes/" + url.PathEscape(node) + "/storage/" + url.PathEscape(storage) + "/file-restore/download?" + q.Encode())
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full.String(), nil)
	if err != nil {
		return nil, err
	}
	c.Authorize(req)
	res, err := c.longClient().Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		raw, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		return nil, parseAPIError(res.StatusCode, raw)
	}
	return res, nil
}

func (c *Client) VolumeDownload(ctx context.Context, node, storage, volume string) (*http.Response, error) {
	path := "/nodes/" + url.PathEscape(node) + "/storage/" + url.PathEscape(storage) + "/content/" + url.PathEscape(volume) + "?download=1"
	full, err := url.Parse(c.base.String() + path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full.String(), nil)
	if err != nil {
		return nil, err
	}
	c.Authorize(req)
	res, err := c.longClient().Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusNotFound {
		_ = res.Body.Close()
		return nil, &apiError{Status: http.StatusNotFound, Message: "volume download is not available"}
	}
	if res.StatusCode >= 400 {
		raw, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		return nil, parseAPIError(res.StatusCode, raw)
	}
	ct := res.Header.Get("Content-Type")
	if strings.Contains(ct, "json") {
		_ = res.Body.Close()
		return nil, &apiError{Status: http.StatusNotFound, Message: "volume download is not available"}
	}
	return res, nil
}

func (c *Client) PruneBackupsPreview(ctx context.Context, node, storage, prune string, vmid int, kind string) ([]PrunePreview, error) {
	path := prunePath(node, storage, prune, vmid, kind)
	var out []PrunePreview
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

func (c *Client) PruneBackups(ctx context.Context, node, storage, prune string, vmid int, kind string) (string, error) {
	path := prunePath(node, storage, prune, vmid, kind)
	var upid string
	err := c.do(ctx, http.MethodDelete, path, nil, &upid)
	return upid, err
}

func (c *Client) ReplicationJobs(ctx context.Context) ([]ReplicationJob, error) {
	var out []ReplicationJob
	err := c.do(ctx, http.MethodGet, "/cluster/replication", nil, &out)
	return out, err
}

func (c *Client) ReplicationJob(ctx context.Context, id string) (ReplicationJob, error) {
	var out ReplicationJob
	err := c.do(ctx, http.MethodGet, "/cluster/replication/"+url.PathEscape(id), nil, &out)
	return out, err
}

func (c *Client) CreateReplication(ctx context.Context, req ReplicationRequest) error {
	if req.ID == "" {
		jobs, err := c.ReplicationJobs(ctx)
		if err != nil {
			return err
		}
		req.ID = nextReplicationID(jobs, req.Guest)
	}
	if req.Type == "" {
		req.Type = "local"
	}
	form := replicationForm(req)
	form.Set("id", req.ID)
	return c.do(ctx, http.MethodPost, "/cluster/replication", form, nil)
}

func nextReplicationID(jobs []ReplicationJob, guest int) string {
	prefix := strconv.Itoa(guest) + "-"
	used := map[int]struct{}{}
	for _, job := range jobs {
		n, ok := replicationJobNum(job, guest, prefix)
		if !ok {
			continue
		}
		used[n] = struct{}{}
	}
	for n := 0; ; n++ {
		if _, taken := used[n]; !taken {
			return prefix + strconv.Itoa(n)
		}
	}
}

func replicationJobNum(job ReplicationJob, guest int, prefix string) (int, bool) {
	if rest, ok := strings.CutPrefix(job.ID, prefix); ok {
		if n, err := strconv.Atoi(rest); err == nil {
			return n, true
		}
	}
	if job.Guest == guest {
		return job.JobNum, true
	}
	return 0, false
}

func (c *Client) UpdateReplication(ctx context.Context, id string, req ReplicationRequest) error {
	return c.do(ctx, http.MethodPut, "/cluster/replication/"+url.PathEscape(id), replicationForm(req), nil)
}

func (c *Client) DeleteReplication(ctx context.Context, id string, opts DeleteReplicationOptions) error {
	q := url.Values{}
	if opts.Force {
		q.Set("force", "1")
	}
	if opts.Keep {
		q.Set("keep", "1")
	}
	path := "/cluster/replication/" + url.PathEscape(id)
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) ReplicationStatus(ctx context.Context, node, id string) (ReplicationStatus, error) {
	var out ReplicationStatus
	err := c.do(ctx, http.MethodGet, "/nodes/"+url.PathEscape(node)+"/replication/"+url.PathEscape(id)+"/status", nil, &out)
	return out, err
}

func (c *Client) ReplicationLog(ctx context.Context, node, id string) ([]TaskLogLine, error) {
	var out []TaskLogLine
	err := c.do(ctx, http.MethodGet, "/nodes/"+url.PathEscape(node)+"/replication/"+url.PathEscape(id)+"/log", nil, &out)
	return out, err
}

func (c *Client) ReplicationScheduleNow(ctx context.Context, node, id string) (string, error) {
	var upid string
	err := c.do(ctx, http.MethodPost, "/nodes/"+url.PathEscape(node)+"/replication/"+url.PathEscape(id)+"/schedule_now", url.Values{}, &upid)
	return upid, err
}

func backupVolume(storage, volume string) string {
	volume = strings.TrimSpace(volume)
	if storage != "" && strings.HasPrefix(volume, storage+":") {
		return strings.TrimPrefix(volume, storage+":")
	}
	if i := strings.Index(volume, ":"); i > 0 && strings.HasPrefix(volume[i+1:], "backup/") {
		return volume[i+1:]
	}
	return volume
}

// fileRestorePath is the PVE file-restore filepath. The schema requires it;
// "/" lists the snapshot root. Any other value is the base64 path PVE returned
// on the parent listing.
func fileRestorePath(filepath string) string {
	if strings.TrimSpace(filepath) == "" {
		return "/"
	}
	return filepath
}

func (b *pveBool) UnmarshalJSON(data []byte) error {
	switch strings.TrimSpace(string(data)) {
	case "true", "1", `"true"`, `"1"`:
		*b = true
	case "false", "0", `"false"`, `"0"`, "null":
		*b = false
	default:
		return fmt.Errorf("invalid bool %s", data)
	}
	return nil
}

func InferBackupType(volid string) string {
	s := strings.ToLower(volid)
	if strings.Contains(s, "vzdump-lxc") || strings.Contains(s, "/ct/") || strings.HasPrefix(s, "ct/") {
		return "lxc"
	}
	return "qemu"
}

func backupJobForm(req BackupJobRequest) url.Values {
	form := url.Values{}
	if req.Schedule != "" {
		form.Set("schedule", req.Schedule)
	}
	if req.Storage != "" {
		form.Set("storage", req.Storage)
	}
	if req.Mode != "" {
		form.Set("mode", req.Mode)
	}
	if req.Compress != "" {
		form.Set("compress", req.Compress)
	}
	if req.All {
		form.Set("all", "1")
	} else if req.VMID != "" {
		form.Set("vmid", req.VMID)
		form.Set("all", "0")
	}
	if req.Exclude != "" {
		form.Set("exclude", req.Exclude)
	}
	if req.Node != "" {
		form.Set("node", req.Node)
	}
	if req.Notes != "" {
		form.Set("notes-template", req.Notes)
	}
	if req.Mailto != "" {
		form.Set("mailto", req.Mailto)
	}
	if req.MailNotification != "" {
		form.Set("mailnotification", req.MailNotification)
	}
	if req.NotificationMode != "" {
		form.Set("notification-mode", req.NotificationMode)
	}
	if req.PruneBackups != "" {
		form.Set("prune-backups", req.PruneBackups)
	}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	if req.Fleecing != "" {
		form.Set("fleecing", req.Fleecing)
	}
	if req.Bwlimit != "" {
		form.Set("bwlimit", req.Bwlimit)
	}
	if req.PBSChangeDetection != "" {
		form.Set("pbs-change-detection-mode", req.PBSChangeDetection)
	}
	if req.Enabled != nil {
		if *req.Enabled {
			form.Set("enabled", "1")
		} else {
			form.Set("enabled", "0")
		}
	}
	for k, v := range req.Extra {
		if v != "" {
			form.Set(k, v)
		}
	}
	return form
}

func replicationForm(req ReplicationRequest) url.Values {
	form := url.Values{}
	if req.Target != "" {
		form.Set("target", req.Target)
	}
	if req.Type != "" {
		form.Set("type", req.Type)
	}
	if req.Schedule != "" {
		form.Set("schedule", req.Schedule)
	}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	if req.Rate != "" {
		form.Set("rate", req.Rate)
	}
	if req.Disable != nil {
		if *req.Disable {
			form.Set("disable", "1")
		} else {
			form.Set("disable", "0")
		}
	}
	if req.RemoveJob != "" {
		form.Set("remove_job", req.RemoveJob)
	}
	return form
}

func prunePath(node, storage, prune string, vmid int, kind string) string {
	q := url.Values{}
	if prune != "" {
		q.Set("prune-backups", prune)
	}
	if vmid > 0 {
		q.Set("vmid", strconv.Itoa(vmid))
	}
	if kind != "" {
		q.Set("type", kind)
	}
	path := "/nodes/" + url.PathEscape(node) + "/storage/" + url.PathEscape(storage) + "/prunebackups"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	return path
}

func newBackupJobID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "backup-" + hex.EncodeToString(b)
}
