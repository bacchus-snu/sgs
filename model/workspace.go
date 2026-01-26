package model

import (
	"context"
)

// WorkspaceUser represents a user in a workspace with their acceptance status.
// Email being empty indicates a pending invitation.
type WorkspaceUser struct {
	Username string
	Email    string // empty = pending invitation
}

// IsAccepted returns true if the user has accepted the workspace invitation.
func (u WorkspaceUser) IsAccepted() bool {
	return u.Email != ""
}

// Usernames returns a slice of just the usernames from a WorkspaceUser slice.
func Usernames(users []WorkspaceUser) []string {
	result := make([]string, len(users))
	for i, u := range users {
		result[i] = u.Username
	}
	return result
}

type Workspace struct {
	ID

	Created   bool
	Enabled   bool
	Nodegroup Nodegroup
	Userdata  string

	Quotas map[Resource]uint64
	Users  []WorkspaceUser

	Request *WorkspaceUpdate
}

func (ws Workspace) Valid() bool {
	if !ws.Nodegroup.Valid() {
		return false
	}
	for quota := range ws.Quotas {
		if !quota.Valid() {
			return false
		}
	}

	uniqueUsers := make(map[string]struct{})
	for _, user := range ws.Users {
		uniqueUsers[user.Username] = struct{}{}
	}
	if len(uniqueUsers) != len(ws.Users) || len(uniqueUsers) == 0 {
		return false
	}

	if ws.Request != nil && !ws.Request.Valid() {
		return false
	}

	return true
}

// InitialRequest returns the initial request for a new workspace. It has the
// same attributes as the workspace itself, but enabled.
func (ws *Workspace) InitialRequest() *WorkspaceUpdate {
	usernames := make([]string, len(ws.Users))
	for i, u := range ws.Users {
		usernames[i] = u.Username
	}
	return &WorkspaceUpdate{
		WorkspaceID: ws.ID,
		ByUser:      ws.Users[0].Username,
		Enabled:     true,
		Nodegroup:   ws.Nodegroup,
		Userdata:    ws.Userdata,
		Quotas:      ws.Quotas,
		Users:       usernames,
	}
}

type WorkspaceUpdate struct {
	WorkspaceID ID
	ByUser      string

	Enabled   bool
	Nodegroup Nodegroup
	Userdata  string

	Quotas map[Resource]uint64
	Users  []string
}

func (ws WorkspaceUpdate) Valid() bool {
	// don't check ByUser, may be empty if admin
	if !ws.Nodegroup.Valid() {
		return false
	}
	for quota := range ws.Quotas {
		if !quota.Valid() {
			return false
		}
	}

	uniqueUsers := make(map[string]struct{})
	for _, user := range ws.Users {
		uniqueUsers[user] = struct{}{}
	}
	if len(uniqueUsers) != len(ws.Users) || len(uniqueUsers) == 0 {
		return false
	}

	return true
}

type Resource string

const (
	ResCPURequest          Resource = "requests.cpu"
	ResCPULimit            Resource = "limits.cpu"
	ResMemoryRequest       Resource = "requests.memory"
	ResMemoryLimit         Resource = "limits.memory"
	ResStorageRequest      Resource = "requests.storage"
	ResGPURequest          Resource = "requests.nvidia.com/gpu"
	ResGPUMemoryRequest    Resource = "requests.nvidia.com/gpumem"
)

var Resources = []Resource{
	ResCPURequest, ResCPULimit,
	ResMemoryRequest, ResMemoryLimit,
	ResStorageRequest, ResGPURequest, ResGPUMemoryRequest,
}

func (r Resource) Valid() bool {
	switch r {
	case ResCPULimit, ResCPURequest,
		ResMemoryLimit, ResMemoryRequest,
		ResStorageRequest, ResGPURequest, ResGPUMemoryRequest:
		return true
	}
	return false
}

type Nodegroup string

const (
	NodegroupUndergraduate Nodegroup = "undergraduate"
	NodegroupGraduate      Nodegroup = "graduate"
)

var Nodegroups = []Nodegroup{
	NodegroupUndergraduate,
	NodegroupGraduate,
}

func (n Nodegroup) Valid() bool {
	switch n {
	case NodegroupUndergraduate, NodegroupGraduate:
		return true
	}
	return false
}

type WorkspaceService interface {
	// Accept user-provided fields only.
	CreateWorkspace(ctx context.Context, ws *Workspace, creatorEmail string) (*Workspace, error)

	// Full scan every workspace.
	ListAllWorkspaces(ctx context.Context) ([]*Workspace, error)
	// Only list workspaces for a given user (accepted invitations only).
	ListUserWorkspaces(ctx context.Context, user string) ([]*Workspace, error)
	// List all created workspaces.
	ListCreatedWorkspaces(ctx context.Context) ([]*Workspace, error)
	// List workspaces where user has pending invitation (email is NULL).
	ListUserInvitations(ctx context.Context, user string) ([]*Workspace, error)

	GetWorkspace(ctx context.Context, id ID) (*Workspace, error)
	// Return ErrNotFound if not owned.
	GetUserWorkspace(ctx context.Context, id ID, user string) (*Workspace, error)

	// Immediately apply any changes, for admins.
	UpdateWorkspace(ctx context.Context, upd *WorkspaceUpdate) (*Workspace, error)
	// Requetst an update, for uesrs. Ignore admin-controlled fields.
	RequestUpdateWorkspace(ctx context.Context, upd *WorkspaceUpdate) (*Workspace, error)

	// Accept a workspace invitation (sets email for the user).
	AcceptInvitation(ctx context.Context, workspaceID ID, username, email string) error
	// Decline a workspace invitation (removes user from workspace).
	DeclineInvitation(ctx context.Context, workspaceID ID, username string) error

	DeleteWorkspace(ctx context.Context, id ID) error
}
