package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strconv"

	"github.com/spf13/viper"

	"github.com/bacchus-snu/sgs/model"
)

type Worker interface {
	Work(ctx context.Context, vwss ValueWorkspaces) error
}

type WorkerFunc func(ctx context.Context, vwss ValueWorkspaces) error

func (wf WorkerFunc) Work(ctx context.Context, vwss ValueWorkspaces) error {
	return wf(ctx, vwss)
}

type ValueWorkspace struct {
	ID        int64             `json:"id"`
	IDHash    string            `json:"idHash"`
	Enabled   bool              `json:"enabled"`
	Nodegroup string            `json:"nodegroup"`
	Quotas    map[string]string `json:"quotas"`
	Users     []string          `json:"users"`
}

type ValueWorkspaces struct {
	Workspaces []ValueWorkspace `json:"workspaces"`
}

func toVWorkspace(ws *model.Workspace) ValueWorkspace {
	vws := ValueWorkspace{
		ID:        int64(ws.ID),
		IDHash:    ws.ID.Hash(),
		Enabled:   ws.Enabled,
		Nodegroup: string(ws.Nodegroup),
		Quotas:    make(map[string]string, len(ws.Quotas)),
		Users:     model.Usernames(ws.Users),
	}
	for k, v := range ws.Quotas {
		switch k {
		case model.ResMemoryLimit, model.ResMemoryRequest, model.ResStorageRequest:
			vws.Quotas[string(k)] = strconv.FormatUint(v, 10) + "Gi"
		case model.ResGPUMemoryRequest:
			vws.Quotas[string(k)] = strconv.FormatUint(v, 10) + "Gi"
		default:
			vws.Quotas[string(k)] = strconv.FormatUint(v, 10)
		}
	}
	return vws
}

func toVWorkspaces(wss []*model.Workspace) ValueWorkspaces {
	vwss := ValueWorkspaces{make([]ValueWorkspace, len(wss))}
	for i, ws := range wss {
		vwss.Workspaces[i] = toVWorkspace(ws)
	}
	return vwss
}

func CmdWorker(command string) WorkerFunc {
	return func(ctx context.Context, vwss ValueWorkspaces) error {
		b, err := json.Marshal(vwss)
		if err != nil {
			return err
		}

		cmd := exec.CommandContext(ctx, "bash", "-c", command)
		cmd.Stdin = bytes.NewReader(b)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return errors.Join(
				fmt.Errorf("cmd worker: %v; %v", err, string(out)),
				ctx.Err())
		}
		log.Println(string(out))

		return nil
	}
}

type Config struct {
	Command string `mapstructure:"command"`
}

func (c *Config) Bind() {
	viper.BindEnv("worker.command", "SGS_WORKER_COMMAND")
}

func (c *Config) Validate() error {
	if c.Command == "" {
		return fmt.Errorf("command is required")
	}
	return nil
}
