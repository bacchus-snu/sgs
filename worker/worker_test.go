package worker

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCmdWorker(t *testing.T) {
	f, err := os.CreateTemp("", "worker-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	t.Cleanup(func() { f.Close() })

	vwss := ValueWorkspaces{
		Workspaces: []ValueWorkspace{
			{ID: 1},
			{ID: 2},
		},
	}

	w := CmdWorker("tee", f.Name())
	if err := w.Work(context.Background(), vwss); err != nil {
		t.Fatal(err)
	}

	got := ValueWorkspaces{}
	if err := json.NewDecoder(f).Decode(&got); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(got, vwss); diff != "" {
		t.Fatalf("output mismatch\n%s", diff)
	}
}
