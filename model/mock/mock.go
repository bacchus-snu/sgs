package mock

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/bacchus-snu/sgs/model"
)

func containsUser(users []model.WorkspaceUser, username string) bool {
	for _, u := range users {
		if u.Username == username {
			return true
		}
	}
	return false
}

func sortUsers(users []model.WorkspaceUser) {
	slices.SortFunc(users, func(a, b model.WorkspaceUser) int {
		return cmp.Compare(a.Username, b.Username)
	})
}

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

func (svc *mockWorkspaces) CreateWorkspace(ctx context.Context, ws *model.Workspace, creatorEmail string) (*model.Workspace, error) {
	if !ws.Valid() {
		return nil, model.ErrInvalid
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	newWS := cloneWorkspace(ws)
	newWS.ID = svc.nextID
	svc.nextID++
	newWS.Enabled = false
	newWS.Request = &model.WorkspaceUpdate{
		WorkspaceID: newWS.ID,
		ByUser:      newWS.Users[0].Username,
		Enabled:     true,
		Nodegroup:   newWS.Nodegroup,
		Userdata:    newWS.Userdata,
		Quotas:      maps.Clone(newWS.Quotas),
		Users:       model.Usernames(newWS.Users),
	}
	sortUsers(newWS.Users)

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
		if containsUser(ws.Users, user) {
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
	if !containsUser(ws.Users, user) {
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
	// Convert []string to []WorkspaceUser
	newUsers := make([]model.WorkspaceUser, len(upd.Users))
	for i, u := range upd.Users {
		newUsers[i] = model.WorkspaceUser{Username: u}
	}
	ws.Users = newUsers
	sortUsers(ws.Users)
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
	if !containsUser(ws.Users, upd.ByUser) {
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

func (svc *mockWorkspaces) ListUserInvitations(ctx context.Context, user string) ([]*model.Workspace, error) {
	// For mock, return empty - no invitation tracking
	return []*model.Workspace{}, nil
}

func (svc *mockWorkspaces) AcceptInvitation(ctx context.Context, workspaceID model.ID, username, email string) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	ws, ok := svc.data[workspaceID]
	if !ok || !containsUser(ws.Users, username) {
		return model.ErrNotFound
	}
	return nil
}

func (svc *mockWorkspaces) DeclineInvitation(ctx context.Context, workspaceID model.ID, username string) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	ws, ok := svc.data[workspaceID]
	if !ok {
		return model.ErrNotFound
	}
	newUsers := make([]model.WorkspaceUser, 0, len(ws.Users))
	for _, u := range ws.Users {
		if u.Username != username {
			newUsers = append(newUsers, u)
		}
	}
	if len(newUsers) == len(ws.Users) {
		return model.ErrNotFound
	}
	ws.Users = newUsers
	return nil
}
