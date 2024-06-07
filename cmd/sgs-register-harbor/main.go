package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	defer context.AfterFunc(ctx, stop)()

	if err := run(ctx); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

type workspace struct {
	IDHash string   `yaml:"idHash"`
	Users  []string `yaml:"users"`
}

func run(ctx context.Context) error {
	var want struct {
		Workspaces []workspace `yaml:"workspaces"`
	}
	err := yaml.NewDecoder(os.Stdin).Decode(&want)
	if err != nil {
		return fmt.Errorf("failed parsing workspace specification: %w", err)
	}

	harborURL := os.Getenv("SGS_HARBOR_URL")
	if harborURL == "" {
		return errors.New("SGS_HARBOR_URL is not set")
	}
	harborUsername := os.Getenv("SGS_HARBOR_USERNAME")
	if harborUsername == "" {
		return errors.New("SGS_HARBOR_USERNAME is not set")
	}
	harborPassword := os.Getenv("SGS_HARBOR_PASSWORD")
	if harborPassword == "" {
		return errors.New("SGS_HARBOR_PASSWORD is not set")
	}

	hapi, err := loadHarbor(harborURL, harborUsername, harborPassword)
	if err != nil {
		return fmt.Errorf("failed loading harbor client: %w", err)
	}

	return runSync(ctx, hapi, want.Workspaces)
}

func runSync(ctx context.Context, hapi harborIface, want []workspace) error {
	var outErr error

	projects, err := hapi.listProjects(ctx)
	if err != nil {
		return fmt.Errorf("failed listing projects: %w", err)
	}

	for _, p := range projects {
		if !strings.HasPrefix(p, "ws-") {
			// ignore non-workspace projects
			continue
		}

		ind := slices.IndexFunc(want, func(w workspace) bool {
			return fmt.Sprintf("ws-%s", w.IDHash) == p
		})

		if ind == -1 {
			// not found, delete project
			slog.InfoContext(ctx, fmt.Sprintf("deleting project %q", p))
			if err := hapi.deleteProject(ctx, p); err != nil {
				err = fmt.Errorf("failed deleting project %q: %w", p, err)
				slog.WarnContext(ctx, err.Error())
				outErr = errors.Join(outErr, err)
			}
			continue
		}

		slog.InfoContext(ctx, fmt.Sprintf("syncing workspace %q", p))
		if err := syncWorkspace(ctx, hapi, want[ind]); err != nil {
			err = fmt.Errorf("failed syncing workspace %q: %w", p, err)
			slog.WarnContext(ctx, err.Error())
			outErr = errors.Join(outErr, err)
		}

		// remove from reference slice
		want = slices.Delete(want, ind, ind+1)
	}

	// create remaining workspaces
	for _, w := range want {
		slog.InfoContext(ctx, fmt.Sprintf("creating workspace %q", w.IDHash))
		if err := createWorkspace(ctx, hapi, w); err != nil {
			err = fmt.Errorf("failed creating workspace %q: %w", w.IDHash, err)
			slog.WarnContext(ctx, err.Error())
			outErr = errors.Join(outErr, err)
		}
	}

	return outErr
}

func syncWorkspace(ctx context.Context, hapi harborIface, w workspace) error {
	project := fmt.Sprintf("ws-%s", w.IDHash)

	members, err := hapi.listMembers(ctx, project)
	if err != nil {
		return fmt.Errorf("failed listing members: %w", err)
	}

	for _, m := range members {
		if m.name == "admin" {
			continue
		}

		ind := slices.Index(w.Users, m.name)
		if ind == -1 {
			// not found, delete member
			slog.InfoContext(ctx, fmt.Sprintf("deleting user %q from project %q", m.name, project))
			if err := hapi.deleteMember(ctx, project, m.id); err != nil {
				return err
			}
			continue
		}

		w.Users = slices.Delete(w.Users, ind, ind+1)
	}

	for _, u := range w.Users {
		slog.InfoContext(ctx, fmt.Sprintf("adding user %q to project %q", u, project))
		if err := hapi.createMember(ctx, project, u); err != nil {
			return err
		}
	}

	return nil
}

func createWorkspace(ctx context.Context, hapi harborIface, w workspace) error {
	project := fmt.Sprintf("ws-%s", w.IDHash)

	if err := hapi.createProject(ctx, project); err != nil {
		return err
	}
	for _, u := range w.Users {
		slog.InfoContext(ctx, fmt.Sprintf("adding user %q to project %q", u, project))
		if err := hapi.createMember(ctx, project, u); err != nil {
			return err
		}
	}

	slog.InfoContext(ctx, fmt.Sprintf("creating robot for project %q", project))
	// TODO: It would be nice to actually sync the robot if not exists on cluster
	// but that's hard.
	if err := hapi.createRobot(ctx, project); err != nil {
		return err
	}

	return nil
}
