package controller

import (
	"net/http"
	"slices"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/bacchus-snu/sgs/model"
	"github.com/bacchus-snu/sgs/pkg/auth"
	"github.com/bacchus-snu/sgs/view"
	"github.com/bacchus-snu/sgs/worker"
)

func handleListWorkspaces(
	wsSvc model.WorkspaceService,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		user := c.Get("user").(*auth.User)

		var wss []*model.Workspace
		var invitations []*model.Workspace
		var err error
		if user.IsAdmin() {
			wss, err = wsSvc.ListAllWorkspaces(c.Request().Context())
		} else {
			wss, err = wsSvc.ListUserWorkspaces(c.Request().Context(), user.Username)
		}
		if err != nil {
			return err
		}

		// Fetch pending invitations for non-admin users
		if !user.IsAdmin() {
			invitations, err = wsSvc.ListUserInvitations(c.Request().Context(), user.Username)
			if err != nil {
				return err
			}
		}

		return c.Render(http.StatusOK, "", view.PageWorkspaceList(wss, invitations))
	}
}

func handleWorkspaceDetails(
	wsSvc model.WorkspaceService,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := model.ParseID(c.Param("id"))
		if err != nil {
			return echo.ErrNotFound
		}
		user := c.Get("user").(*auth.User)

		var ws *model.Workspace
		if user.IsAdmin() {
			ws, err = wsSvc.GetWorkspace(c.Request().Context(), id)
		} else {
			ws, err = wsSvc.GetUserWorkspace(c.Request().Context(), id, user.Username)
		}
		if err != nil {
			return err
		}

		return c.Render(http.StatusOK, "", view.PageWorkspaceDetails(ws))
	}
}

func handleRequestWorkspaceForm() echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.Render(http.StatusOK, "", view.PageRequestForm())
	}
}

// Check whether the user is allowed to request a workspace in the given
// nodegroup.
func checkNodegroups(user *auth.User, nodegroup string) error {
	if !model.Nodegroup(nodegroup).Valid() {
		return echo.ErrBadRequest
	}
	if slices.Contains(user.Groups, nodegroup) {
		return nil
	}
	return echo.ErrForbidden
}

func handleRequestWorkspace(
	wsSvc model.WorkspaceService,
) echo.HandlerFunc {
	type formData struct {
		Nodegroup          string `form:"nodegroup"`
		Userdata           string `form:"userdata"`
		QuotaGPU           uint64 `form:"quota-gpu"`
		QuotaGPUMemory     uint64 `form:"quota-gpu-memory"`
		QuotaStorage       uint64 `form:"quota-storage"`
		QuotaMemoryRequest uint64 `form:"quota-memory-requests"`
		QuotaMemoryLimit   uint64 `form:"quota-memory-limits"`
		QuotaCPURequest    uint64 `form:"quota-cpu-requests"`
		QuotaCPULimit      uint64 `form:"quota-cpu-limits"`
	}

	return func(c echo.Context) error {
		var req formData
		if err := c.Bind(&req); err != nil {
			return err
		}
		user := c.Get("user").(*auth.User)

		ws := model.Workspace{
			Nodegroup: model.Nodegroup(req.Nodegroup),
			Userdata:  req.Userdata,
			Quotas: map[model.Resource]uint64{
				model.ResGPURequest:       req.QuotaGPU,
				model.ResGPUMemoryRequest: req.QuotaGPUMemory,
				model.ResStorageRequest:   req.QuotaStorage,
				model.ResCPURequest:       req.QuotaCPURequest,
				model.ResCPULimit:         req.QuotaCPULimit,
				model.ResMemoryRequest:    req.QuotaMemoryRequest,
				model.ResMemoryLimit:      req.QuotaMemoryLimit,
			},
			Users: []model.WorkspaceUser{{Username: user.Username, Email: user.Email}},
		}

		if !ws.Valid() {
			return echo.ErrBadRequest
		}

		if err := checkNodegroups(user, req.Nodegroup); err != nil {
			return err
		}

		newWS, err := wsSvc.CreateWorkspace(c.Request().Context(), &ws, user.Email)
		if err != nil {
			return err
		}
		return c.Redirect(http.StatusSeeOther, c.Echo().Reverse("workspace-details", newWS.ID.Hash()))
	}
}

func handleUpdateWorkspace(
	queue worker.Queue,
	wsSvc model.WorkspaceService,
) echo.HandlerFunc {
	type formData struct {
		Enabled            string `form:"enabled"`
		Nodegroup          string `form:"nodegroup"`
		Userdata           string `form:"userdata"`
		QuotaGPU           uint64 `form:"quota-gpu"`
		QuotaGPUMemory     uint64 `form:"quota-gpu-memory"`
		QuotaStorage       uint64 `form:"quota-storage"`
		QuotaMemoryRequest uint64 `form:"quota-memory-requests"`
		QuotaMemoryLimit   uint64 `form:"quota-memory-limits"`
		QuotaCPURequest    uint64 `form:"quota-cpu-requests"`
		QuotaCPULimit      uint64 `form:"quota-cpu-limits"`
		Action             string `form:"action"`
	}

	return func(c echo.Context) error {
		var req formData
		if err := c.Bind(&req); err != nil {
			return err
		}
		id, err := model.ParseID(c.Param("id"))
		if err != nil {
			return echo.ErrNotFound
		}
		user := c.Get("user").(*auth.User)

		if req.Action == "delete" {
			if !user.IsAdmin() {
				return echo.ErrForbidden
			}
			err := wsSvc.DeleteWorkspace(c.Request().Context(), id)
			if err != nil {
				return err
			}

			queue.Enqueue()
			return c.Redirect(http.StatusSeeOther, c.Echo().Reverse("workspace-list"))
		}

		upd := model.WorkspaceUpdate{
			WorkspaceID: id,
			ByUser:      user.Username,
			Enabled:     req.Enabled == "on",
			Nodegroup:   model.Nodegroup(req.Nodegroup),
			Userdata:    req.Userdata,
			Quotas: map[model.Resource]uint64{
				model.ResGPURequest:       req.QuotaGPU,
				model.ResGPUMemoryRequest: req.QuotaGPUMemory,
				model.ResStorageRequest:   req.QuotaStorage,
				model.ResCPURequest:       req.QuotaCPURequest,
				model.ResCPULimit:         req.QuotaCPULimit,
				model.ResMemoryRequest:    req.QuotaMemoryRequest,
				model.ResMemoryLimit:      req.QuotaMemoryLimit,
			},
		}
		form, _ := c.FormParams()
		for k, v := range form {
			if !strings.HasPrefix(k, "user-") {
				continue
			}
			username := strings.TrimSpace(v[0])
			if username != "" {
				upd.Users = append(upd.Users, username)
			}
		}

		if !upd.Valid() {
			return echo.ErrBadRequest
		}

		var ws *model.Workspace
		switch req.Action {
		case "request":
			if err := checkNodegroups(user, req.Nodegroup); err != nil {
				return err
			}
			ws, err = wsSvc.RequestUpdateWorkspace(c.Request().Context(), &upd)
		case "update":
			if !user.IsAdmin() {
				return echo.ErrForbidden
			}
			ws, err = wsSvc.UpdateWorkspace(c.Request().Context(), &upd)
		default:
			return echo.ErrBadRequest
		}
		if err != nil {
			return err
		}

		// If not a request, we should re-render
		if req.Action != "request" {
			queue.Enqueue()
		}

		// We could render HTML based on the returned ws, but that would make
		// refreshing the browser potentially dangerous. Instead, redirect with See
		// Other.
		return c.Redirect(http.StatusSeeOther, c.Echo().Reverse("workspace-details", ws.ID.Hash()))
	}
}

func handleAcceptInvitation(
	wsSvc model.WorkspaceService,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := model.ParseID(c.Param("id"))
		if err != nil {
			return echo.ErrNotFound
		}
		user := c.Get("user").(*auth.User)

		err = wsSvc.AcceptInvitation(c.Request().Context(), id, user.Username, user.Email)
		if err != nil {
			return err
		}

		return c.Redirect(http.StatusSeeOther, c.Echo().Reverse("workspace-list"))
	}
}

func handleDeclineInvitation(
	wsSvc model.WorkspaceService,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := model.ParseID(c.Param("id"))
		if err != nil {
			return echo.ErrNotFound
		}
		user := c.Get("user").(*auth.User)

		err = wsSvc.DeclineInvitation(c.Request().Context(), id, user.Username)
		if err != nil {
			return err
		}

		return c.Redirect(http.StatusSeeOther, c.Echo().Reverse("workspace-list"))
	}
}
