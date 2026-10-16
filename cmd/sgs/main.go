package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/bacchus-snu/sgs/controller"
	"github.com/bacchus-snu/sgs/model/postgres"
	"github.com/bacchus-snu/sgs/pkg/auth"
	"github.com/bacchus-snu/sgs/pkg/config"
	"github.com/bacchus-snu/sgs/worker"
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

func run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	authSvc, err := auth.New(ctx, cfg.Auth)
	if err != nil {
		return err
	}

	repo, err := postgres.New(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer repo.Close()

	queue := worker.NewQueue(
		repo.Workspaces(),
		worker.CmdWorker(cfg.Worker.Command),
		time.Minute, 5*time.Minute,
	)
	queue.Enqueue() // enqueue update on startup

	queueErrCh := make(chan error, 1)
	go func() {
		defer cancel()
		queueErrCh <- queue.Start(ctx)
	}()

	e := echo.New()
	controller.AddRoutes(e, cfg.Controller, queue, authSvc, repo.Workspaces())

	startErrCh := make(chan error, 1)
	go func() {
		defer cancel()
		startErrCh <- e.Start(":8080")
	}()

	<-ctx.Done()
	shutErr := e.Shutdown(context.Background())
	startErr := <-startErrCh
	queueErr := <-queueErrCh

	return errors.Join(shutErr, startErr, queueErr)
}
