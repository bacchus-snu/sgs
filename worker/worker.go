package worker

import (
	"bytes"
	"context"
	"encoding/json"
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
		Users:     ws.Users,
	}
	for k, v := range ws.Quotas {
		switch k {
		case model.ResMemoryLimit, model.ResMemoryRequest, model.ResStorageRequest:
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

func CmdWorker(name string, args ...string) WorkerFunc {
	return func(ctx context.Context, vwss ValueWorkspaces) error {
		b, err := json.Marshal(vwss)
		if err != nil {
			return err
		}

		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Stdin = bytes.NewReader(b)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("cmd worker: %v; %v", err, string(out))
		}
		log.Println(string(out))

		return nil
	}
}

type Config struct {
	Apply     bool   `mapstructure:"apply"`
	ChartPath string `mapstructure:"chart_path"`
}

func (c *Config) Bind() {
	viper.SetDefault("worker.apply", false)
	viper.BindEnv("worker.apply", "SGS_WORKER_APPLY")
	viper.BindEnv("worker.chart_path", "SGS_WORKER_CHART_PATH")
}

func (c *Config) Validate() error {
	if c.ChartPath == "" {
		return fmt.Errorf("chart_path is required")
	}
	return nil
}

func ApplyWorker(cfg Config) WorkerFunc {
	cmd := "helm template sgs \"$SGS_WORKER_CHART_PATH\" -f -"
	applyCmd := ""
	if cfg.Apply {
		applyCmd = "KUBECTL_APPLYSET=true kubectl apply --applyset workspacesets.sgs.snucse.org/sgs --prune -f -"
	} else {
		applyCmd = "KUBECTL_APPLYSET=true kubectl apply --applyset workspacesets.sgs.snucse.org/sgs --prune --dry-run=client -f -"
	}
	return CmdWorker("bash", "-c", fmt.Sprintf("%s | %s", cmd, applyCmd))
}
