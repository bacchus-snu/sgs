package test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/bacchus-snu/sgs/model"
)

func TestWorkspace(t *testing.T, wsf func() model.WorkspaceService) {
	type testScenario func(t *testing.T, wsSvc model.WorkspaceService)
	tests := map[string]testScenario{
		"happy": func(t *testing.T, wsSvc model.WorkspaceService) {
			want := model.Workspace{
				Nodegroup: model.NodegroupUndergraduate,
				Userdata:  "userdata",
				Quotas:    map[model.Resource]uint64{model.ResGPURequest: 8},
				Users:     []model.WorkspaceUser{{Username: "user1"}},
			}
			want.ID = testWorkspaceCreate(t, wsSvc, &want, nil)
			want.Request = want.InitialRequest()

			testWorkspaceListAll(t, wsSvc, []*model.Workspace{&want})
			testWorkspaceListUser(t, wsSvc, "user1", []*model.Workspace{&want})
			testWorkspaceListCreated(t, wsSvc, nil)
			testWorkspaceGet(t, wsSvc, want.ID, &want)
			testWorkspaceGetUser(t, wsSvc, want.ID, "user1", &want)

			changes := model.WorkspaceUpdate{
				WorkspaceID: want.ID,
				ByUser:      "user1",
				Enabled:     true,
				Nodegroup:   model.NodegroupGraduate,
				Userdata:    "changed",
				Quotas:      map[model.Resource]uint64{model.ResGPURequest: 4},
				Users:       []string{"user1", "user2"},
			}
			want.Request = &changes
			testWorkspaceRequestUpdate(t, wsSvc, &changes, &want, nil)

			testWorkspaceListAll(t, wsSvc, []*model.Workspace{&want})
			testWorkspaceListUser(t, wsSvc, "user1", []*model.Workspace{&want})
			testWorkspaceListCreated(t, wsSvc, nil)
			testWorkspaceGet(t, wsSvc, want.ID, &want)
			testWorkspaceGetUser(t, wsSvc, want.ID, "user1", &want)

			want.Created = true
			want.Enabled = true
			want.Nodegroup = changes.Nodegroup
			want.Userdata = changes.Userdata
			want.Quotas = changes.Quotas
			// Convert []string to []WorkspaceUser, preserving existing emails
			existingEmails := make(map[string]string)
			for _, u := range want.Users {
				if u.Email != "" {
					existingEmails[u.Username] = u.Email
				}
			}
			want.Users = make([]model.WorkspaceUser, len(changes.Users))
			for i, u := range changes.Users {
				want.Users[i] = model.WorkspaceUser{Username: u, Email: existingEmails[u]}
			}
			want.Request = nil
			testWorkspaceUpdate(t, wsSvc, &changes, &want, nil)

			testWorkspaceListAll(t, wsSvc, []*model.Workspace{&want})
			testWorkspaceListUser(t, wsSvc, "user1", []*model.Workspace{&want})
			testWorkspaceListCreated(t, wsSvc, []*model.Workspace{&want})
			testWorkspaceGet(t, wsSvc, want.ID, &want)
			testWorkspaceGetUser(t, wsSvc, want.ID, "user1", &want)

			testWorkspaceDelete(t, wsSvc, want.ID, nil)

			testWorkspaceListAll(t, wsSvc, nil)
			testWorkspaceListUser(t, wsSvc, "user1", nil)
			testWorkspaceListCreated(t, wsSvc, nil)
			testWorkspaceGet(t, wsSvc, want.ID, nil)
			testWorkspaceGetUser(t, wsSvc, want.ID, "user1", nil)
		},

		"ignore-fields": func(t *testing.T, wsSvc model.WorkspaceService) {
			testWorkspaceCreate(t, wsSvc, &model.Workspace{
				ID:        123,
				Enabled:   true,
				Nodegroup: model.NodegroupUndergraduate,
				Userdata:  "userdata",
				Quotas:    map[model.Resource]uint64{model.ResGPURequest: 8},
				Users:     []model.WorkspaceUser{{Username: "user1"}},
				Request: &model.WorkspaceUpdate{
					WorkspaceID: 123,
					ByUser:      "user1",
					Enabled:     true,
					Nodegroup:   model.NodegroupGraduate,
					Userdata:    "changed",
					Quotas:      map[model.Resource]uint64{model.ResGPURequest: 4},
					Users:       []string{"user1", "user2"},
				},
			}, nil)
		},

		"create-invalid": func(t *testing.T, wsSvc model.WorkspaceService) {
			// invalid nodegroup
			testWorkspaceCreate(t, wsSvc, &model.Workspace{
				Nodegroup: "invalid",
				Users:     []model.WorkspaceUser{{Username: "user1"}},
			}, model.ErrInvalid)
			// invalid quotas
			testWorkspaceCreate(t, wsSvc, &model.Workspace{
				Nodegroup: model.NodegroupUndergraduate,
				Quotas:    map[model.Resource]uint64{"invalid": 8},
				Users:     []model.WorkspaceUser{{Username: "user1"}},
			}, model.ErrInvalid)
			// invalid users
			testWorkspaceCreate(t, wsSvc, &model.Workspace{
				Nodegroup: model.NodegroupUndergraduate,
				Users:     nil,
			}, model.ErrInvalid)
			// duplicate users
			testWorkspaceCreate(t, wsSvc, &model.Workspace{
				Nodegroup: model.NodegroupUndergraduate,
				Users:     []model.WorkspaceUser{{Username: "user1"}, {Username: "user1"}},
			}, model.ErrInvalid)
		},

		"update-invalid": func(t *testing.T, wsSvc model.WorkspaceService) {
			id := testWorkspaceCreate(t, wsSvc, &model.Workspace{
				Nodegroup: model.NodegroupUndergraduate,
				Users:     []model.WorkspaceUser{{Username: "user1"}},
			}, nil)

			// invalid nodegroup
			testWorkspaceUpdate(t, wsSvc, &model.WorkspaceUpdate{
				WorkspaceID: id,
				Nodegroup:   "invalid",
				Users:       []string{"user1"},
			}, nil, model.ErrInvalid)
			testWorkspaceRequestUpdate(t, wsSvc, &model.WorkspaceUpdate{
				WorkspaceID: id,
				ByUser:      "user1",
				Nodegroup:   "invalid",
				Users:       []string{"user1"},
			}, nil, model.ErrInvalid)
			// invalid quotas
			testWorkspaceUpdate(t, wsSvc, &model.WorkspaceUpdate{
				WorkspaceID: id,
				Nodegroup:   model.NodegroupUndergraduate,
				Quotas:      map[model.Resource]uint64{"invalid": 8},
				Users:       []string{"user1"},
			}, nil, model.ErrInvalid)
			testWorkspaceRequestUpdate(t, wsSvc, &model.WorkspaceUpdate{
				WorkspaceID: id,
				ByUser:      "user1",
				Nodegroup:   model.NodegroupUndergraduate,
				Quotas:      map[model.Resource]uint64{"invalid": 8},
				Users:       []string{"user1"},
			}, nil, model.ErrInvalid)
			// invalid users
			testWorkspaceUpdate(t, wsSvc, &model.WorkspaceUpdate{
				WorkspaceID: id,
				Nodegroup:   model.NodegroupUndergraduate,
				Users:       nil,
			}, nil, model.ErrInvalid)
			testWorkspaceRequestUpdate(t, wsSvc, &model.WorkspaceUpdate{
				WorkspaceID: id,
				ByUser:      "user1",
				Nodegroup:   model.NodegroupUndergraduate,
				Users:       nil,
			}, nil, model.ErrInvalid)
			// duplicate users
			testWorkspaceUpdate(t, wsSvc, &model.WorkspaceUpdate{
				WorkspaceID: id,
				Nodegroup:   model.NodegroupUndergraduate,
				Users:       []string{"user1", "user1"},
			}, nil, model.ErrInvalid)
			testWorkspaceRequestUpdate(t, wsSvc, &model.WorkspaceUpdate{
				WorkspaceID: id,
				ByUser:      "user1",
				Nodegroup:   model.NodegroupUndergraduate,
				Users:       []string{"user1", "user1"},
			}, nil, model.ErrInvalid)
			// invalid self
			testWorkspaceRequestUpdate(t, wsSvc, &model.WorkspaceUpdate{
				WorkspaceID: id,
				ByUser:      "user1",
				Nodegroup:   model.NodegroupUndergraduate,
				Users:       []string{"user2"},
			}, nil, model.ErrInvalid)
		},

		"empty": func(t *testing.T, wsSvc model.WorkspaceService) {
			testWorkspaceListAll(t, wsSvc, nil)
			testWorkspaceListUser(t, wsSvc, "user1", nil)
			testWorkspaceListCreated(t, wsSvc, nil)

			testWorkspaceGet(t, wsSvc, 123, nil)
			testWorkspaceGetUser(t, wsSvc, 123, "user1", nil)

			testWorkspaceUpdate(t, wsSvc, &model.WorkspaceUpdate{
				WorkspaceID: 123,
				Nodegroup:   model.NodegroupUndergraduate,
				Users:       []string{"user1"},
			}, nil, model.ErrNotFound)
			testWorkspaceRequestUpdate(t, wsSvc, &model.WorkspaceUpdate{
				WorkspaceID: 123,
				ByUser:      "user1",
				Nodegroup:   model.NodegroupUndergraduate,
				Users:       []string{"user1"},
			}, nil, model.ErrNotFound)

			testWorkspaceDelete(t, wsSvc, 123, model.ErrNotFound)
		},

		"userfilter": func(t *testing.T, wsSvc model.WorkspaceService) {
			ws1 := model.Workspace{
				Nodegroup: model.NodegroupUndergraduate,
				Users:     []model.WorkspaceUser{{Username: "user1"}},
			}
			ws1.ID = testWorkspaceCreate(t, wsSvc, &ws1, nil)
			ws1.Request = ws1.InitialRequest()

			ws2 := model.Workspace{
				Nodegroup: model.NodegroupGraduate,
				Users:     []model.WorkspaceUser{{Username: "user2"}},
			}
			ws2.ID = testWorkspaceCreate(t, wsSvc, &ws2, nil)
			ws2.Request = ws2.InitialRequest()

			wsAll := model.Workspace{
				Nodegroup: model.NodegroupUndergraduate,
				Users:     []model.WorkspaceUser{{Username: "user1"}, {Username: "user2"}},
			}
			wsAll.ID = testWorkspaceCreate(t, wsSvc, &wsAll, nil)
			wsAll.Request = wsAll.InitialRequest()

			testWorkspaceListAll(t, wsSvc, []*model.Workspace{&ws1, &ws2, &wsAll})
			testWorkspaceListUser(t, wsSvc, "user1", []*model.Workspace{&ws1, &wsAll})
			testWorkspaceListUser(t, wsSvc, "user2", []*model.Workspace{&ws2, &wsAll})
			testWorkspaceListUser(t, wsSvc, "user3", nil)

			testWorkspaceGet(t, wsSvc, ws1.ID, &ws1)
			testWorkspaceGet(t, wsSvc, ws2.ID, &ws2)
			testWorkspaceGet(t, wsSvc, wsAll.ID, &wsAll)

			testWorkspaceGetUser(t, wsSvc, ws1.ID, "user2", nil)
			testWorkspaceGetUser(t, wsSvc, ws2.ID, "user1", nil)
			testWorkspaceGetUser(t, wsSvc, wsAll.ID, "user1", &wsAll)
			testWorkspaceGetUser(t, wsSvc, wsAll.ID, "user2", &wsAll)
			testWorkspaceGetUser(t, wsSvc, wsAll.ID, "user3", nil)

			testWorkspaceRequestUpdate(t, wsSvc, &model.WorkspaceUpdate{
				WorkspaceID: ws2.ID,
				ByUser:      "user1",
				Nodegroup:   model.NodegroupGraduate,
				Users:       []string{"user1"},
			}, nil, model.ErrNotFound)
		},

		"enabled-created": func(t *testing.T, wsSvc model.WorkspaceService) {
			ws := model.Workspace{
				Nodegroup: model.NodegroupUndergraduate,
				Users:     []model.WorkspaceUser{{Username: "user1"}},
			}
			ws.ID = testWorkspaceCreate(t, wsSvc, &ws, nil)

			ws.Created = true
			ws.Enabled = true
			testWorkspaceUpdate(t, wsSvc, &model.WorkspaceUpdate{
				WorkspaceID: ws.ID,
				Enabled:     true,
				Nodegroup:   ws.Nodegroup,
				Users:       model.Usernames(ws.Users),
			}, &ws, nil)

			ws.Enabled = false
			testWorkspaceUpdate(t, wsSvc, &model.WorkspaceUpdate{
				WorkspaceID: ws.ID,
				Enabled:     false,
				Nodegroup:   ws.Nodegroup,
				Users:       model.Usernames(ws.Users),
			}, &ws, nil)
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test(t, wsf())
		})
	}
}

func testWorkspaceCreate(t *testing.T, wsSvc model.WorkspaceService, ws *model.Workspace, expErr error) model.ID {
	t.Helper()
	const creatorEmail = "test@example.com"
	got, err := wsSvc.CreateWorkspace(context.Background(), ws, creatorEmail)
	if !errors.Is(err, expErr) {
		t.Fatalf("CreateWorkspace(%#v) = %v; want %v", ws, err, expErr)
	}
	if err != nil {
		return 0
	}

	want := *ws
	want.ID = got.ID
	want.Enabled = false
	// Creator (first user) gets their email set
	if len(want.Users) > 0 {
		want.Users = make([]model.WorkspaceUser, len(ws.Users))
		copy(want.Users, ws.Users)
		want.Users[0].Email = creatorEmail
	}
	want.Request = want.InitialRequest()
	if diff := cmp.Diff(got, &want, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("CreateWorkspace(%#v) = mismatch\n%s", ws, diff)
	}

	// Update the original workspace for subsequent test assertions
	if len(ws.Users) > 0 {
		ws.Users[0].Email = creatorEmail
	}

	return got.ID
}

func testWorkspaceListAll(t *testing.T, wsSvc model.WorkspaceService, expect []*model.Workspace) {
	t.Helper()
	wss, err := wsSvc.ListAllWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListAllWorkspaces() = %v; want nil", err)
	}
	if diff := cmp.Diff(wss, expect, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("ListAllWorkspaces() = mismatch\n%s", diff)
	}
}

func testWorkspaceListUser(t *testing.T, wsSvc model.WorkspaceService, user string, expect []*model.Workspace) {
	t.Helper()
	wss, err := wsSvc.ListUserWorkspaces(context.Background(), user)
	if err != nil {
		t.Fatalf("ListUserWorkspaces(%q) = %v; want nil", user, err)
	}
	if diff := cmp.Diff(wss, expect, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("ListUserWorkspaces(%q) = mismatch\n%s", user, diff)
	}
}

func testWorkspaceListCreated(t *testing.T, wsSvc model.WorkspaceService, expect []*model.Workspace) {
	t.Helper()
	wss, err := wsSvc.ListCreatedWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListCreatedWorkspaces() = %v; want nil", err)
	}
	if diff := cmp.Diff(wss, expect, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("ListCreatedWorkspaces() = mismatch\n%s", diff)
	}
}

func testWorkspaceGet(t *testing.T, wsSvc model.WorkspaceService, id model.ID, expect *model.Workspace) {
	t.Helper()
	ws, err := wsSvc.GetWorkspace(context.Background(), id)
	if expect != nil && err != nil {
		t.Fatalf("GetWorkspace(%d) = %v; want nil", id, err)
	}
	if expect == nil && !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("GetWorkspace(%d) = %v; want %v", id, err, model.ErrNotFound)
	}
	if diff := cmp.Diff(ws, expect, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("GetWorkspace(%d) = mismatch\n%s", id, diff)
	}
}

func testWorkspaceGetUser(t *testing.T, wsSvc model.WorkspaceService, id model.ID, user string, expect *model.Workspace) {
	t.Helper()
	ws, err := wsSvc.GetUserWorkspace(context.Background(), id, user)
	if expect != nil && err != nil {
		t.Fatalf("GetUserWorkspace(%d, %q) = %v; want nil", id, user, err)
	}
	if expect == nil && !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("GetUserWorkspace(%d, %q) = %v; want %v", id, user, err, model.ErrNotFound)
	}
	if diff := cmp.Diff(ws, expect, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("GetUserWorkspace(%d, %q) = mismatch\n%s", id, user, diff)
	}
}

func testWorkspaceUpdate(t *testing.T, wsSvc model.WorkspaceService, upd *model.WorkspaceUpdate, expect *model.Workspace, expErr error) {
	t.Helper()
	ws, err := wsSvc.UpdateWorkspace(context.Background(), upd)
	if !errors.Is(err, expErr) {
		t.Fatalf("UpdateWorkspace(%#v) = %v; want %v", upd, err, expErr)
	}
	if diff := cmp.Diff(ws, expect, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("UpdateWorkspace(%#v) = mismatch\n%s", upd, diff)
	}
}

func testWorkspaceRequestUpdate(t *testing.T, wsSvc model.WorkspaceService, upd *model.WorkspaceUpdate, expect *model.Workspace, expErr error) {
	t.Helper()
	ws, err := wsSvc.RequestUpdateWorkspace(context.Background(), upd)
	if !errors.Is(err, expErr) {
		t.Fatalf("RequestUpdateWorkspace(%#v) = %v; want %v", upd, err, expErr)
	}
	if diff := cmp.Diff(ws, expect, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("RequestUpdateWorkspace(%#v) = mismatch\n%s", upd, diff)
	}
}

func testWorkspaceDelete(t *testing.T, wsSvc model.WorkspaceService, id model.ID, expErr error) {
	t.Helper()
	err := wsSvc.DeleteWorkspace(context.Background(), id)
	if !errors.Is(err, expErr) {
		t.Fatalf("DeleteWorkspace(%d) = %v; want %v", id, err, expErr)
	}
}
