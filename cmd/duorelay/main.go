package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/holi/duorelay/internal/api"
	"github.com/holi/duorelay/internal/configuration"
	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/speedtest"
	"github.com/holi/duorelay/internal/store"
	"github.com/holi/duorelay/internal/subscription"
	"github.com/holi/duorelay/internal/task"
	"github.com/holi/duorelay/internal/transparent"
	"github.com/holi/duorelay/internal/update"
	"github.com/holi/duorelay/internal/variant"
)

var (
	Version   = "0.0.1-dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--apply-update" {
		if err := update.RunHelper(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
			os.Exit(1)
		}
		return
	}
	fmt.Println("DuoRelay - Dual-Core Universal Proxy Panel")
	fmt.Printf("Version: %s (commit %s, built %s)\n", Version, Commit, BuildTime)

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	var updates *update.Service
	defer func() {
		if updates != nil {
			if err := updates.Finalize(); err != nil {
				runErr = errors.Join(runErr, err)
			}
		}
	}()

	flags := flag.NewFlagSet("duorelay", flag.ContinueOnError)
	listen := flags.String("listen", "127.0.0.1:8080", "HTTP listen address")
	dbPath := flags.String("db", filepath.Join("data", "duorelay.db"), "SQLite database file")
	kernels := flags.String("kernels", "kernels", "kernel executable directory")
	configDir := flags.String("config-dir", filepath.Join("data", "configs"), "generated configuration directory")
	workers := flags.Int("workers", 4, "background worker count")
	cookieSecure := flags.Bool("cookie-secure", false, "mark session cookies Secure (use behind an HTTPS reverse proxy)")
	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *workers < 1 || *workers > 64 {
		return fmt.Errorf("workers must be 1..64")
	}
	if err := kernel.PrepareData(context.Background(), *configDir); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*dbPath), 0700); err != nil {
		return err
	}
	database, err := store.New(&store.Config{Path: *dbPath, WAL: true})
	if err != nil {
		return err
	}
	defer database.Close()
	queue := task.NewQueue(*workers)
	defer queue.Close()
	runtime := kernel.NewManager(*kernels)
	if err := runtime.SetLogDirectory(filepath.Join(filepath.Dir(*dbPath), "logs")); err != nil {
		return err
	}
	config := configuration.New(database, runtime, *configDir)
	config.SetTransparent(transparent.New())
	updates = update.New(Version, variant.Name)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := config.Shutdown(cleanupCtx); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("stop runtime during shutdown: %w", err))
		}
	}()
	subs := subscription.NewManager(database)
	speed := speedtest.NewEngine(database, queue)
	speed.ProbeHTTP = config.ProbeHTTP
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := config.Resume(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not resume previous configuration: %v\n", err)
	}
	handler := api.NewServer(database, subs, speed, config, runtime)
	handler.Context = ctx
	handler.CookieSecure = *cookieSecure
	handler.BuildInfo = map[string]string{"version": Version, "commit": Commit, "build_time": BuildTime, "variant": variant.Name}
	handler.Updates = updates
	handler.Updates.Restart = cancel
	server := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	scheduled := make(chan error, 1)
	go func() { scheduled <- subs.Run(ctx, time.Minute) }()
	serving := make(chan error, 1)
	go func() { serving <- server.ListenAndServe() }()
	fmt.Printf("API listening at http://%s\n", *listen)
	var result error
	select {
	case <-ctx.Done():
	case result = <-serving:
	case result = <-scheduled:
		scheduled = nil
	}
	cancel()
	shutdownCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		result = errors.Join(result, err)
	}
	if scheduled != nil {
		<-scheduled
	}
	if errors.Is(result, http.ErrServerClosed) || errors.Is(result, context.Canceled) {
		return nil
	}
	return result
}
