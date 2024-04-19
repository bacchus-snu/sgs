package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/bacchus-snu/sgs/model"
	"github.com/bacchus-snu/sgs/model/mock"
)

func TestQueueCoalesce(t *testing.T) {
	t.Parallel()

	repo := mock.New()

	// slow worker
	calls := 0
	wf := func(ctx context.Context, vwss ValueWorkspaces) error {
		t := time.NewTimer(time.Second)
		defer t.Stop()
		select {
		case <-t.C:
			calls++
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	q := NewQueue(repo.Workspaces, WorkerFunc(wf), 5*time.Second, 5*time.Second)

	for range 10 {
		q.Enqueue()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	if err := q.Start(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("q.Start err = %v; want %v", err, context.DeadlineExceeded)
	}

	if calls != 1 {
		t.Fatalf("calls = %d; want 1", calls)
	}
}

func TestQueuePeriod(t *testing.T) {
	t.Parallel()

	repo := mock.New()

	calls := 0
	wf := func(ctx context.Context, vwss ValueWorkspaces) error {
		calls++
		return nil
	}

	q := NewQueue(repo.Workspaces, WorkerFunc(wf), time.Second, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second/2)
	t.Cleanup(cancel)

	if err := q.Start(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("q.Start err = %v; want %v", err, context.DeadlineExceeded)
	}

	if calls != 2 {
		t.Fatalf("calls = %d; want 2", calls)
	}
}

func TestQueue(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	repo := mock.New()

	// enabled workspace
	ws, err := repo.Workspaces.CreateWorkspace(ctx, &model.Workspace{
		Nodegroup: model.NodegroupUndergraduate,
		Userdata:  "enabled",
		Quotas: map[model.Resource]uint64{
			model.ResGPURequest: 1,
		},
		Users: []string{"user"},
	})
	if err != nil {
		t.Fatalf("CreateWorkspace err = %v; want nil", err)
	}

	ws, err = repo.Workspaces.UpdateWorkspace(ctx, &model.WorkspaceUpdate{
		WorkspaceID: ws.ID,
		Enabled:     true,
		Nodegroup:   ws.Nodegroup,
		Userdata:    ws.Userdata,
		Quotas:      ws.Quotas,
		Users:       ws.Users,
	})
	if err != nil {
		t.Fatalf("UpdateWorkspace err = %v; want nil", err)
	}
	wantVws := toVWorkspace(ws)

	// disabled workspace
	wsDisabled, err := repo.Workspaces.CreateWorkspace(ctx, &model.Workspace{
		Nodegroup: model.NodegroupUndergraduate,
		Userdata:  "enabled",
		Quotas: map[model.Resource]uint64{
			model.ResGPURequest: 1,
		},
		Users: []string{"user"},
	})
	if err != nil {
		t.Fatalf("CreateWorkspace err = %v; want nil", err)
	}

	wsDisabled, err = repo.Workspaces.UpdateWorkspace(ctx, &model.WorkspaceUpdate{
		WorkspaceID: wsDisabled.ID,
		Enabled:     true,
		Nodegroup:   wsDisabled.Nodegroup,
		Userdata:    wsDisabled.Userdata,
		Quotas:      wsDisabled.Quotas,
		Users:       wsDisabled.Users,
	})
	if err != nil {
		t.Fatalf("UpdateWorkspace err = %v; want nil", err)
	}
	wsDisabled, err = repo.Workspaces.UpdateWorkspace(ctx, &model.WorkspaceUpdate{
		WorkspaceID: wsDisabled.ID,
		Enabled:     false,
		Nodegroup:   wsDisabled.Nodegroup,
		Userdata:    wsDisabled.Userdata,
		Quotas:      wsDisabled.Quotas,
		Users:       wsDisabled.Users,
	})
	if err != nil {
		t.Fatalf("UpdateWorkspace err = %v; want nil", err)
	}
	wantDisabledVws := toVWorkspace(wsDisabled)

	// non-created workspace
	_, err = repo.Workspaces.CreateWorkspace(ctx, &model.Workspace{
		Nodegroup: model.NodegroupUndergraduate,
		Userdata:  "non-created",
		Quotas:    map[model.Resource]uint64{},
		Users:     []string{"user"},
	})
	if err != nil {
		t.Fatalf("CreateWorkspace err = %v; want nil", err)
	}

	calls := 0
	callVwss := ValueWorkspaces{}
	wf := func(ctx context.Context, vwss ValueWorkspaces) error {
		calls++
		callVwss = vwss
		return nil
	}

	q := NewQueue(repo.Workspaces, WorkerFunc(wf), 5*time.Second, 5*time.Second)
	q.Enqueue()

	if err := q.Start(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("q.Start err = %v; want %v", err, context.DeadlineExceeded)
	}

	if calls != 1 {
		t.Fatalf("calls = %d; want 1", calls)
	}

	if len(callVwss.Workspaces) != 2 {
		t.Fatalf("len(callVwss.Workspaces) = %d; want 2", len(callVwss.Workspaces))
	}
	if diff := cmp.Diff(callVwss.Workspaces[0], wantVws, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("callVwss.Workspaces[0] = mismatch\n%s", diff)
	}
	if diff := cmp.Diff(callVwss.Workspaces[1], wantDisabledVws, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("callVwss.Workspaces[0] = mismatch\n%s", diff)
	}
}
