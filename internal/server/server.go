// Package server wires the configuration, acme client, job manager and API
// handlers into an HTTP server and runs its lifecycle.
package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/bright-color/acmesh-ui/internal/acme"
	"github.com/bright-color/acmesh-ui/internal/api"
	"github.com/bright-color/acmesh-ui/internal/config"
	"github.com/bright-color/acmesh-ui/internal/dnsproviders"
	"github.com/bright-color/acmesh-ui/internal/jobs"
	"github.com/bright-color/acmesh-ui/internal/secrets"
	"github.com/bright-color/acmesh-ui/internal/storage"
)

// App owns the long-lived components.
type App struct {
	cfg      config.Config
	store    *storage.Store
	jobs     *jobs.Manager
	handlers *api.Handlers
	http     *http.Server
}

// New builds the application from a validated config.
func New(cfg config.Config, configPath, uiVersion string) (*App, error) {
	// Secret key + cipher.
	key, err := secrets.LoadOrCreateKey(cfg.Security.SecretKeyFile)
	if err != nil {
		return nil, fmt.Errorf("secret key: %w", err)
	}
	cipher, err := secrets.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher: %w", err)
	}

	// Storage.
	store, err := storage.Open(cfg.Data.Dir + "/acmesh-ui.db")
	if err != nil {
		return nil, err
	}

	builder := acme.Builder{DefaultCA: cfg.Acme.DefaultCA, DefaultKeyType: cfg.Acme.DefaultKeyType}
	client := acme.NewClient(cfg.Acme.Binary, cfg.Acme.Home, builder)
	scanner := acme.NewScanner(cfg.Acme.Home, cfg.Certs.ExpiringSoonDays)
	masker := secrets.NewMasker()

	jm := jobs.NewManager(client, masker, store,
		cfg.Jobs.MaxParallel, time.Duration(cfg.Jobs.TimeoutSeconds)*time.Second)
	dnsStore := dnsproviders.NewStore(store, cipher)

	h := &api.Handlers{
		Cfg:        cfg,
		Client:     client,
		Scanner:    scanner,
		Builder:    builder,
		Jobs:       jm,
		DNS:        dnsStore,
		Masker:     masker,
		Started:    time.Now(),
		UIVersion:  uiVersion,
		ConfigPath: configPath,
	}

	handler := withMiddleware(routes(h))
	addr := net.JoinHostPort(cfg.Server.Bind, strconv.Itoa(cfg.Server.Port))

	return &App{
		cfg:      cfg,
		store:    store,
		jobs:     jm,
		handlers: h,
		http: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}, nil
}

// Run starts the HTTP server and blocks until ctx is cancelled.
func (a *App) Run(ctx context.Context) error {
	a.logStartupWarnings()

	// Background prune of old job logs.
	go a.pruneLoop(ctx)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("acmesh-ui listening on http://%s (auth.mode=%s)", a.http.Addr, a.cfg.Auth.Mode)
		if err := a.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		log.Printf("shutting down...")
		_ = a.http.Shutdown(shutCtx)
		_ = a.store.Close()
		return nil
	case err := <-errCh:
		_ = a.store.Close()
		return err
	}
}

func (a *App) pruneLoop(ctx context.Context) {
	retention := time.Duration(a.cfg.Jobs.LogRetentionDays) * 24 * time.Hour
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	_ = a.jobs.Prune(retention)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.jobs.Prune(retention); err != nil {
				log.Printf("prune jobs: %v", err)
			}
		}
	}
}

// logStartupWarnings emits the mandatory security warnings at boot.
func (a *App) logStartupWarnings() {
	if a.cfg.AuthDisabled() {
		log.Printf("WARNING: internal authentication is DISABLED (auth.mode=none). " +
			"Only run acmesh-ui behind a VPN, SSH tunnel, or an authenticating reverse proxy.")
	}
	if a.cfg.IsOpenBind() && a.cfg.AuthDisabled() {
		log.Printf("WARNING: bound to %s without authentication. allow_open_without_auth is set; "+
			"ensure network access is restricted.", a.cfg.Server.Bind)
	}
}
