package main

import (
	"context"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/bacchus-snu/sgs/model"
	"github.com/bacchus-snu/sgs/model/mock"
)

// Test environment
var (
	testEnv    *envtest.Environment
	k8sClient  client.Client
	k8sManager ctrl.Manager
	wsSvc      model.WorkspaceService

	ctx    context.Context
	cancel context.CancelFunc
)

func setupEnv(t *testing.T) {
	t.Helper()

	ctx = ctrl.SetupSignalHandler()
	ctx, cancel = context.WithCancel(ctx)
	t.Cleanup(cancel)

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	// run  only once
	if testEnv != nil {
		return
	}

	testEnv = &envtest.Environment{}
	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := testEnv.Stop(); err != nil {
			t.Fatal(err)
		}
	})

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{})
	if err != nil {
		t.Fatal(err)
	}
	k8sClient = k8sManager.GetClient()

	repo := mock.New()
	wsSvc = repo.Workspaces

	ws := &model.Workspace{
		Nodegroup: "undergraduate",
		Quotas:    map[model.Resource]uint64{model.ResGPURequest: 1},
		Users:     []string{"user1"},
	}
	ws, err = wsSvc.CreateWorkspace(ctx, ws)
	if err != nil {
		t.Fatal(err)
	}

	ws, err = wsSvc.UpdateWorkspace(ctx, ws.InitialRequest())
	if err != nil {
		t.Fatal(err)
	}

	req := ws.InitialRequest()
	req.Enabled = false

	ws, err = wsSvc.UpdateWorkspace(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(ws.ID.Hash())

	err = (&workspaceReconciler{
		Client: k8sClient,
		Scheme: k8sManager.GetScheme(),
		WSSvc:  wsSvc,
	}).SetupWithManager(k8sManager)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		err := k8sManager.Start(ctx)
		if err != nil {
			t.Errorf("failed to run manager: %v", err)
		}
	}()

	t.Cleanup(cancel) // ensure manager is stopped
}

func TestController(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
	setupEnv(t)

	time.Sleep(10 * time.Second)
}
