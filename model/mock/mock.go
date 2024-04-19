package mock

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/bacchus-snu/sgs/model"
)

func cloneWorkspace(ws *model.Workspace) *model.Workspace {
	out := *ws
	out.Quotas = maps.Clone(out.Quotas)
	out.Users = slices.Clone(out.Users)
	out.Request = cloneWorkspaceRequest(out.Request)
	return &out
}

func cloneWorkspaceRequest(upd *model.WorkspaceUpdate) *model.WorkspaceUpdate {
	if upd == nil {
		return nil
	}

	out := *upd
	out.Quotas = maps.Clone(out.Quotas)
	out.Users = slices.Clone(out.Users)
	return &out
}

type Repository struct {
	Workspaces *mockWorkspaces
}

type mockWorkspaces struct {
	mu     sync.Mutex
	nextID model.ID
	data   map[model.ID]*model.Workspace
}

func New() Repository {
	return Repository{Workspaces: &mockWorkspaces{
		data: make(map[model.ID]*model.Workspace),
	}}
}

func (svc *mockWorkspaces) CreateWorkspace(ctx context.Context, ws *model.Workspace) (*model.Workspace, error) {
	if !ws.Valid() {
		return nil, model.ErrInvalid
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	newWS := cloneWorkspace(ws)
	newWS.ID = svc.nextID
	svc.nextID++
	newWS.Enabled = false
	newWS.Request = nil
	slices.Sort(newWS.Users)

	svc.data[newWS.ID] = newWS
	return cloneWorkspace(newWS), nil
}

func (svc *mockWorkspaces) ListAllWorkspaces(ctx context.Context) ([]*model.Workspace, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	var wss []*model.Workspace
	for _, ws := range svc.data {
		wss = append(wss, cloneWorkspace(ws))
	}

	slices.SortFunc(wss, func(i, j *model.Workspace) int {
		switch {
		case i.ID < j.ID:
			return -1
		case i.ID > j.ID:
			return 1
		default:
			return 0
		}
	})
	return wss, nil
}

func (svc *mockWorkspaces) ListUserWorkspaces(ctx context.Context, user string) ([]*model.Workspace, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	var wss []*model.Workspace
	for _, ws := range svc.data {
		if slices.Contains(ws.Users, user) {
			wss = append(wss, cloneWorkspace(ws))
		}
	}

	slices.SortFunc(wss, func(i, j *model.Workspace) int {
		switch {
		case i.ID < j.ID:
			return -1
		case i.ID > j.ID:
			return 1
		default:
			return 0
		}
	})
	return wss, nil
}

func (svc *mockWorkspaces) ListCreatedWorkspaces(ctx context.Context) ([]*model.Workspace, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	var wss []*model.Workspace
	for _, ws := range svc.data {
		if ws.Created {
			wss = append(wss, cloneWorkspace(ws))
		}
	}

	slices.SortFunc(wss, func(i, j *model.Workspace) int {
		switch {
		case i.ID < j.ID:
			return -1
		case i.ID > j.ID:
			return 1
		default:
			return 0
		}
	})
	return wss, nil
}

func (svc *mockWorkspaces) GetWorkspace(ctx context.Context, id model.ID) (*model.Workspace, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	ws, ok := svc.data[id]
	if !ok {
		return nil, model.ErrNotFound
	}
	return cloneWorkspace(ws), nil
}

func (svc *mockWorkspaces) GetUserWorkspace(ctx context.Context, id model.ID, user string) (*model.Workspace, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	ws, ok := svc.data[id]
	if !ok {
		return nil, model.ErrNotFound
	}
	if !slices.Contains(ws.Users, user) {
		return nil, model.ErrNotFound
	}

	return cloneWorkspace(ws), nil
}

func (svc *mockWorkspaces) UpdateWorkspace(ctx context.Context, upd *model.WorkspaceUpdate) (*model.Workspace, error) {
	if !upd.Valid() {
		return nil, model.ErrInvalid
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	ws, ok := svc.data[upd.WorkspaceID]
	if !ok {
		return nil, model.ErrNotFound
	}

	ws.Enabled = upd.Enabled
	ws.Created = ws.Created || ws.Enabled // latch on
	ws.Nodegroup = upd.Nodegroup
	ws.Userdata = upd.Userdata
	ws.Quotas = maps.Clone(upd.Quotas)
	ws.Users = slices.Clone(upd.Users)
	slices.Sort(ws.Users)
	ws.Request = nil

	return cloneWorkspace(ws), nil
}

func (svc *mockWorkspaces) RequestUpdateWorkspace(ctx context.Context, upd *model.WorkspaceUpdate) (*model.Workspace, error) {
	if !upd.Valid() {
		return nil, model.ErrInvalid
	}
	if upd.ByUser == "" || !slices.Contains(upd.Users, upd.ByUser) {
		return nil, model.ErrInvalid
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	ws, ok := svc.data[upd.WorkspaceID]
	if !ok {
		return nil, model.ErrNotFound
	}
	if !slices.Contains(ws.Users, upd.ByUser) {
		return nil, model.ErrNotFound
	}

	ws.Request = cloneWorkspaceRequest(upd)
	slices.Sort(ws.Request.Users)
	return cloneWorkspace(ws), nil
}

func (svc *mockWorkspaces) DeleteWorkspace(ctx context.Context, id model.ID) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	if _, ok := svc.data[id]; !ok {
		return model.ErrNotFound
	}
	delete(svc.data, id)
	return nil
}
