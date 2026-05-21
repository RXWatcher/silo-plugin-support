package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	goruntime "runtime"
	"sync/atomic"

	"github.com/hashicorp/go-hclog"
	"github.com/jackc/pgx/v5/pgxpool"

	pluginv1 "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginproto/continuum/plugin/v1"
	publicmanifest "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginsdk/manifest"
	sdkruntime "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginsdk/runtime"

	"github.com/ContinuumApp/continuum-plugin-support/internal/httproutes"
	"github.com/ContinuumApp/continuum-plugin-support/internal/migrate"
	pluginrt "github.com/ContinuumApp/continuum-plugin-support/internal/runtime"
	"github.com/ContinuumApp/continuum-plugin-support/internal/server"
	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

//go:embed manifest.json
var manifestRaw []byte

func main() {
	logger := hclog.New(&hclog.LoggerOptions{Name: "continuum-plugin-support"})
	manifest, err := loadManifest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load manifest: %v\n", err)
		os.Exit(1)
	}

	httpSrv := httproutes.NewServer()
	var poolPtr atomic.Pointer[pgxpool.Pool]

	applyConfig := func(cfg pluginrt.Config) error {
		ctx := context.Background()
		if err := migrate.Run(ctx, cfg.DatabaseURL); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		pcfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("parse database_url: %w", err)
		}
		if pcfg.MaxConns < 4 {
			pcfg.MaxConns = 4
		}
		pool, err := pgxpool.NewWithConfig(ctx, pcfg)
		if err != nil {
			return fmt.Errorf("connect database: %w", err)
		}
		st := store.New(pool)
		cfg, err = st.Bootstrap(ctx, cfg)
		if err != nil {
			pool.Close()
			return fmt.Errorf("bootstrap config: %w", err)
		}
		httpSrv.SetHandler(server.New(server.Deps{
			DatabaseURL:    cfg.DatabaseURL,
			Logger:         logger,
			ConfigStore:    st,
			EventPublisher: hostEventPublisher{},
		}))
		if old := poolPtr.Swap(pool); old != nil {
			old.Close()
		}
		logger.Info("configured support plugin")
		return nil
	}

	rt := pluginrt.New(manifest, applyConfig)

	sdkruntime.Serve(sdkruntime.ServeConfig{
		Logger: logger,
		Servers: sdkruntime.CapabilityServers{
			Runtime:    rt,
			HttpRoutes: httpSrv,
		},
	})
}

// hostEventPublisher is an EventPublisher that delegates to the SDK's
// runtime-host client. sdkruntime.Host() is resolved lazily on each call so
// that events published after BindHostBroker still reach the host even if the
// client was not yet dialled when applyConfig ran.
type hostEventPublisher struct{}

func (hostEventPublisher) PublishEvent(ctx context.Context, name string, payload map[string]any) error {
	h := sdkruntime.Host()
	if h == nil {
		return nil // broker not yet bound; skip silently
	}
	return h.PublishEvent(ctx, name, payload)
}

func loadManifest() (*pluginv1.PluginManifest, error) {
	manifest, err := publicmanifest.Load(manifestRaw)
	if err != nil {
		return nil, fmt.Errorf("load embedded manifest: %w", err)
	}
	executablePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	binaryData, err := os.ReadFile(executablePath)
	if err != nil {
		return nil, fmt.Errorf("read executable %q: %w", executablePath, err)
	}
	checksum := sha256.Sum256(binaryData)
	manifest.Checksum = hex.EncodeToString(checksum[:])
	if len(manifest.GetSupportedPlatforms()) == 0 {
		manifest.SupportedPlatforms = []*pluginv1.SupportedPlatform{
			{Os: goruntime.GOOS, Arch: goruntime.GOARCH},
		}
	}
	return manifest, nil
}
