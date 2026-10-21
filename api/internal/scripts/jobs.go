package scripts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"hypervane/internal/pve"
)

// Job and step states.
const (
	JobRunning = "running"
	JobDone    = "done"
	JobPartial = "partial" // some guests or nodes in a batch failed
	JobFailed  = "failed"

	StepPending = "pending"
	StepRunning = "running"
	StepDone    = "done"
	StepFailed  = "failed"
	StepSkipped = "skipped"
)

// Job is one run of a recipe, as the Recipes page follows it.
type Job struct {
	ID       string    `json:"id"`
	Recipe   string    `json:"recipe"`
	Title    string    `json:"title"`
	Status   string    `json:"status"`
	Steps    []JobStep `json:"steps"`
	Started  int64     `json:"started"`
	Finished int64     `json:"finished,omitempty"`
	// Message explains what a failure left behind.
	Message string `json:"message,omitempty"`
	VMID    int    `json:"vmid,omitempty"`

	owner string
}

type JobStep struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Status string   `json:"status"`
	UPIDs  []string `json:"upids,omitempty"`
	Note   string   `json:"note,omitempty"`
	Error  string   `json:"error,omitempty"`
}

// Jobs keeps recent recipe runs in memory: enough to follow a run and see how
// it ended. Proxmox's task list remains the lasting record.
type Jobs struct {
	// Timeout caps a whole run (a slow image download, a large migration).
	Timeout time.Duration
	// Keep is how many finished runs are remembered.
	Keep int

	mu   sync.Mutex
	list []*Job
}

func NewJobs() *Jobs { return &Jobs{Timeout: 3 * time.Hour, Keep: 50} }

// Start runs the plan in the background with the caller's client and returns
// the job as it begins.
func (js *Jobs) Start(c *pve.Client, plan *Plan, owner string) Job {
	job := &Job{
		ID:      newJobID(),
		Recipe:  plan.Recipe.ID,
		Title:   plan.Recipe.Title,
		Status:  JobRunning,
		Started: time.Now().Unix(),
		VMID:    plan.VMID,
		owner:   owner,
	}
	for _, s := range plan.Steps {
		job.Steps = append(job.Steps, JobStep{ID: s.ID, Label: s.Label, Status: StepPending})
	}
	js.mu.Lock()
	js.list = append(js.list, job)
	js.trim()
	snap := job.copy()
	js.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), js.Timeout)
		defer cancel()
		js.run(ctx, c, job.ID, plan)
	}()
	return snap
}

// Get returns a job the owner started; all lets a local admin see any.
func (js *Jobs) Get(id, owner string, all bool) (Job, bool) {
	js.mu.Lock()
	defer js.mu.Unlock()
	for _, j := range js.list {
		if j.ID == id && (all || j.owner == owner) {
			return j.copy(), true
		}
	}
	return Job{}, false
}

// List returns recent jobs, newest first.
func (js *Jobs) List(owner string, all bool) []Job {
	js.mu.Lock()
	defer js.mu.Unlock()
	out := []Job{}
	for i := len(js.list) - 1; i >= 0; i-- {
		if j := js.list[i]; all || j.owner == owner {
			out = append(out, j.copy())
		}
	}
	return out
}

func (js *Jobs) update(id string, fn func(*Job)) {
	js.mu.Lock()
	defer js.mu.Unlock()
	for _, j := range js.list {
		if j.ID == id {
			fn(j)
			return
		}
	}
}

func (js *Jobs) setStep(id string, i int, status, errMsg string) {
	js.update(id, func(j *Job) {
		j.Steps[i].Status = status
		if errMsg != "" {
			j.Steps[i].Error = errMsg
		}
	})
}

// trim drops the oldest finished runs beyond Keep; running ones stay.
func (js *Jobs) trim() {
	keep := js.Keep
	if keep <= 0 {
		keep = 50
	}
	for len(js.list) > keep {
		dropped := false
		for i, j := range js.list {
			if j.Status != JobRunning {
				js.list = append(js.list[:i], js.list[i+1:]...)
				dropped = true
				break
			}
		}
		if !dropped {
			return
		}
	}
}

func (j *Job) copy() Job {
	out := *j
	out.Steps = make([]JobStep, len(j.Steps))
	for i, s := range j.Steps {
		s.UPIDs = append([]string(nil), s.UPIDs...)
		out.Steps[i] = s
	}
	return out
}

func newJobID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
