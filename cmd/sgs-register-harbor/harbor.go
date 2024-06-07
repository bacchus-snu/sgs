package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-openapi/runtime"
	"github.com/goharbor/go-client/pkg/harbor"
	"github.com/goharbor/go-client/pkg/sdk/v2.0/client"
	"github.com/goharbor/go-client/pkg/sdk/v2.0/client/member"
	"github.com/goharbor/go-client/pkg/sdk/v2.0/client/project"
	"github.com/goharbor/go-client/pkg/sdk/v2.0/client/repository"
	"github.com/goharbor/go-client/pkg/sdk/v2.0/client/robot"
	"github.com/goharbor/go-client/pkg/sdk/v2.0/models"
)

type harborIface interface {
	listProjects(ctx context.Context) ([]string, error)
	createProject(ctx context.Context, name string) error
	deleteProject(ctx context.Context, name string) error

	listMembers(ctx context.Context, name string) ([]harborMember, error)
	createMember(ctx context.Context, name string, username string) error
	deleteMember(ctx context.Context, name string, memberID int64) error

	createRobot(ctx context.Context, name string) error
}

// We need the username for creation, ID for deletion.
type harborMember struct {
	id   int64
	name string
}

func pointer[T any](v T) *T {
	return &v
}

type harborImpl client.HarborAPI

func loadHarbor(url, username, password string) (*harborImpl, error) {
	clientSet, err := harbor.NewClientSet(&harbor.ClientSetConfig{
		URL:      url,
		Username: username,
		Password: password,
	})
	if err != nil {
		return nil, err
	}
	return (*harborImpl)(clientSet.V2()), nil
}

func (h *harborImpl) listProjects(ctx context.Context) ([]string, error) {
	out := []string(nil)

	for page := int64(1); ; page++ {
		res, err := h.Project.ListProjects(ctx, &project.ListProjectsParams{
			Page:     pointer(page),
			PageSize: pointer(int64(10)),
		})
		if err != nil {
			return nil, err
		}

		for _, p := range res.Payload {
			out = append(out, p.Name)
		}
		if int64(len(out)) >= res.XTotalCount {
			break
		}
	}

	return out, nil
}

func (h harborImpl) createProject(ctx context.Context, name string) error {
	_, err := h.Project.CreateProject(ctx, &project.CreateProjectParams{
		Project: &models.ProjectReq{ProjectName: name},
	})
	return err
}

func (h harborImpl) deleteProject(ctx context.Context, name string) error {
	repos := []string(nil)

	for page := int64(1); ; page++ {
		res, err := h.Repository.ListRepositories(ctx, &repository.ListRepositoriesParams{
			ProjectName: name,
			Page:        pointer(page),
			PageSize:    pointer(int64(10)),
		})
		if err != nil {
			return err
		}

		for _, r := range res.Payload {
			repos = append(repos, strings.TrimPrefix(r.Name, name+"/"))
		}

		if int64(len(repos)) >= res.XTotalCount {
			break
		}
	}

	for _, repo := range repos {
		fmt.Println(repo)
		_, err := h.Repository.DeleteRepository(ctx, &repository.DeleteRepositoryParams{
			ProjectName:    name,
			RepositoryName: repo,
		})
		if err != nil {
			return err
		}
	}

	_, err := h.Project.DeleteProject(ctx, &project.DeleteProjectParams{
		XIsResourceName: pointer(true),
		ProjectNameOrID: name,
	})
	return err
}

func (h harborImpl) listMembers(ctx context.Context, name string) ([]harborMember, error) {
	out := []harborMember(nil)

	for page := int64(1); ; page++ {
		res, err := h.Member.ListProjectMembers(ctx, &member.ListProjectMembersParams{
			XIsResourceName: pointer(true),
			ProjectNameOrID: name,
			Page:            pointer(page),
			PageSize:        pointer(int64(10)),
		})
		if err != nil {
			return nil, err
		}

		for _, m := range res.Payload {
			out = append(out, harborMember{m.EntityID, m.EntityName})
		}

		if int64(len(out)) >= res.XTotalCount {
			break
		}
	}

	return out, nil
}

func (h harborImpl) createMember(ctx context.Context, name string, username string) error {
	_, err := h.Member.CreateProjectMember(ctx, &member.CreateProjectMemberParams{
		XIsResourceName: pointer(true),
		ProjectNameOrID: name,
		ProjectMember: &models.ProjectMember{
			RoleID:     4, // maintainer
			MemberUser: &models.UserEntity{Username: username},
		},
	})

	// As a special case, member creation may fail if
	// - the user does not exist / has never logged in to harbor
	// - the user is already a member of the project (including the admin)
	// We return nil in these cases.
	if aerr := (&runtime.APIError{}); errors.As(err, &aerr) && aerr.Code == 404 {
		err = nil
	}
	if err1 := (&member.CreateProjectMemberConflict{}); errors.As(err, &err1) {
		err = nil
	}

	return err
}

func (h harborImpl) deleteMember(ctx context.Context, name string, memberID int64) error {
	_, err := h.Member.DeleteProjectMember(ctx, &member.DeleteProjectMemberParams{
		XIsResourceName: pointer(true),
		ProjectNameOrID: name,
		Mid:             memberID,
	})
	return err
}

func (h harborImpl) createRobot(ctx context.Context, name string) error {
	res, err := h.Robot.CreateRobot(ctx, &robot.CreateRobotParams{
		Robot: &models.RobotCreate{
			Level: "project",
			Name:  "bacchus-sgs",
			Permissions: []*models.RobotPermission{{
				Kind:      "project",
				Namespace: name,
				Access: []*models.Access{{
					Resource: "repository",
					Action:   "pull",
				}},
			}},
			Duration: -1,
		},
	})
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx,
		"kubectl", "create", "secret", "docker-registry",
		"-n", name, "sgs-registry",
		"--docker-server", os.Getenv("SGS_HARBOR_URL"),
		"--docker-username", res.Payload.Name,
		"--docker-password", res.Payload.Secret, // TODO: avoid passing secret as argument
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w; %s", err, string(out))
	}

	cmd = exec.CommandContext(ctx,
		"kubectl", "patch", "serviceaccount",
		"-n", name, "default",
		"-p", fmt.Sprintf(`{"imagePullSecrets": [{"name": "sgs-registry"}]}`),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w; %s", err, string(out))
	}

	return nil
}
