package scripts

import (
	"context"
	"errors"
	"testing"
	"time"
)

func stepFn(err error) func(context.Context, *Exec) error {
	return func(context.Context, *Exec) error { return err }
}

func waitDone(t *testing.T, js *Jobs, id string) Job {
	t.Helper()
	for i := 0; i < 500; i++ {
		if j, _ := js.Get(id, "u1", false); j.Status != JobRunning {
			return j
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return Job{}
}

func TestAFailedStepStopsTheJob(t *testing.T) {
	js := NewJobs()
	pl := &Plan{Recipe: Recipe{ID: "r", Title: "R"}, Steps: []Step{
		{ID: "a", run: stepFn(nil)},
		{ID: "b", OnFail: "b left a half-built VM", run: stepFn(errors.New("boom"))},
		{ID: "c", run: stepFn(nil)},
	}}
	j := waitDone(t, js, js.Start(nil, pl, "u1").ID)
	if j.Status != JobFailed || j.Steps[0].Status != StepDone || j.Steps[1].Status != StepFailed ||
		j.Steps[1].Error != "boom" || j.Steps[2].Status != StepSkipped || j.Message != "b left a half-built VM" {
		t.Fatalf("%+v", j)
	}
}

func TestAFailedGroupSkipsOnlyItsOwnSteps(t *testing.T) {
	js := NewJobs()
	pl := &Plan{Recipe: Recipe{ID: "r"}, Steps: []Step{
		{ID: "clone-1", Group: "1", run: stepFn(nil)},
		{ID: "start-1", Group: "1", run: stepFn(nil)},
		{ID: "clone-2", Group: "2", run: stepFn(errors.New("vmid taken"))},
		{ID: "start-2", Group: "2", run: stepFn(nil)},
		{ID: "clone-3", Group: "3", run: stepFn(nil)},
		{ID: "tidy", Optional: true, run: stepFn(errors.New("could not tidy"))},
	}}
	j := waitDone(t, js, js.Start(nil, pl, "u1").ID)
	want := []string{StepDone, StepDone, StepFailed, StepSkipped, StepDone, StepFailed}
	for i, s := range j.Steps {
		if s.Status != want[i] {
			t.Fatalf("step %s = %s, want %s", s.ID, s.Status, want[i])
		}
	}
	if j.Status != JobPartial {
		t.Fatalf("status %s", j.Status)
	}
	// Only an optional step failing still counts as done.
	pl = &Plan{Recipe: Recipe{ID: "r"}, Steps: []Step{{ID: "a", run: stepFn(nil)}, {ID: "tidy", Optional: true, run: stepFn(errors.New("x"))}}}
	if j := waitDone(t, js, js.Start(nil, pl, "u1").ID); j.Status != JobDone {
		t.Fatalf("optional failure: %s", j.Status)
	}
}

func TestJobsAreVisibleToTheirOwner(t *testing.T) {
	js := NewJobs()
	pl := &Plan{Recipe: Recipe{ID: "r"}, Steps: []Step{{ID: "a", run: stepFn(nil)}}}
	id := js.Start(nil, pl, "u1").ID
	waitDone(t, js, id)
	if _, ok := js.Get(id, "u2", false); ok {
		t.Fatal("another user must not see the job")
	}
	if _, ok := js.Get(id, "u2", true); !ok {
		t.Fatal("an admin sees every job")
	}
	if len(js.List("u2", false)) != 0 || len(js.List("u1", false)) != 1 {
		t.Fatal("list must be scoped to the owner")
	}
}

func TestOldFinishedJobsAreDropped(t *testing.T) {
	js := NewJobs()
	js.Keep = 3
	pl := &Plan{Recipe: Recipe{ID: "r"}, Steps: []Step{{ID: "a", run: stepFn(nil)}}}
	var last string
	for i := 0; i < 6; i++ {
		last = js.Start(nil, pl, "u1").ID
		waitDone(t, js, last)
	}
	if n := len(js.List("u1", false)); n > 3 {
		t.Fatalf("kept %d jobs", n)
	}
	if _, ok := js.Get(last, "u1", false); !ok {
		t.Fatal("the newest job must be kept")
	}
}
