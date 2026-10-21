package scripts

import (
	"context"
	"strings"
	"time"

	"hypervane/internal/pve"
)

// Exec is what a running step can do: call Proxmox, wait for its tasks (which
// the job records, so the page links to each task), and leave a note.
type Exec struct {
	C    *pve.Client
	job  *Jobs
	id   string
	step int
}

// Wait records a Proxmox task on the step and waits for it to finish. An empty
// UPID (a call that finished at once) returns straight away.
func (x *Exec) Wait(ctx context.Context, node, upid string) error {
	if upid == "" {
		return nil
	}
	if n, _, _ := pve.ParseUPID(upid); n != "" {
		node = n
	}
	x.job.update(x.id, func(j *Job) { j.Steps[x.step].UPIDs = append(j.Steps[x.step].UPIDs, upid) })
	_, err := x.C.WaitTask(ctx, node, upid)
	return err
}

// Note adds a line to the step's result.
func (x *Exec) Note(s string) {
	x.job.update(x.id, func(j *Job) {
		st := &j.Steps[x.step]
		st.Note = strings.TrimSpace(st.Note + " " + s)
	})
}

// run executes a plan's steps in order and keeps the job up to date.
func (js *Jobs) run(ctx context.Context, c *pve.Client, id string, plan *Plan) {
	failedGroups := map[string]bool{}
	var failed, done int
	stop := false
	for i, step := range plan.Steps {
		if stop || (step.Group != "" && failedGroups[step.Group]) {
			js.setStep(id, i, StepSkipped, "")
			continue
		}
		js.setStep(id, i, StepRunning, "")
		err := step.run(ctx, &Exec{C: c, job: js, id: id, step: i})
		if err == nil {
			js.setStep(id, i, StepDone, "")
			done++
			continue
		}
		js.setStep(id, i, StepFailed, err.Error())
		switch {
		case step.Optional:
		case step.Group != "":
			failed++
			failedGroups[step.Group] = true
		default:
			failed++
			stop = true
		}
		if step.OnFail != "" {
			js.update(id, func(j *Job) { j.Message = step.OnFail })
		}
		if ctx.Err() != nil {
			stop = true
		}
	}
	js.update(id, func(j *Job) {
		j.Finished = time.Now().Unix()
		switch {
		case failed == 0:
			j.Status = JobDone
		case done > 0 && !stop:
			j.Status = JobPartial
		default:
			j.Status = JobFailed
		}
	})
}
