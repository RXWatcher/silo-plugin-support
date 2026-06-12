package runtime

import (
	"context"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestConfigureRejectsMissingDatabaseURL(t *testing.T) {
	s := New(nil, nil)
	req := &pluginv1.ConfigureRequest{Config: []*pluginv1.ConfigEntry{}}
	if _, err := s.Configure(context.Background(), req); err == nil {
		t.Fatal("expected missing database_url to fail; got nil")
	}
}

func TestConfigureDefaultsKBSpeedtestTicketsOnAIOff(t *testing.T) {
	var observed Config
	s := New(nil, func(cfg Config) error {
		observed = cfg
		return nil
	})
	if _, err := s.Configure(context.Background(), configureRequest()); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if !observed.Modules.KB || !observed.Modules.Speedtest || !observed.Modules.Tickets {
		t.Fatalf("KB + Speedtest + Tickets should default ON; got %+v", observed.Modules)
	}
	if observed.Modules.AI {
		t.Fatalf("AI should default off (not shipped); got %+v", observed.Modules)
	}
	if observed.DatabaseURL != "postgres://x" {
		t.Fatalf("DatabaseURL = %q, want postgres://x", observed.DatabaseURL)
	}
}

func TestNormalizeBackfillsSpamLimitsFromZero(t *testing.T) {
	// A config persisted before the spam/quota keys existed deserializes
	// with zero values; normalization must backfill them from defaults so
	// the protections are always active.
	in := Config{AutoStrategy: "latency", ClientIPStorage: "truncated"}
	out, err := NormalizeAppConfig(in)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	def := DefaultAppConfig()
	if out.TicketsMaxOpenPerCustomer != def.TicketsMaxOpenPerCustomer {
		t.Errorf("max open = %d, want %d", out.TicketsMaxOpenPerCustomer, def.TicketsMaxOpenPerCustomer)
	}
	if out.TicketsMinBodyChars != def.TicketsMinBodyChars {
		t.Errorf("min body = %d, want %d", out.TicketsMinBodyChars, def.TicketsMinBodyChars)
	}
	if out.TicketsMaxStorageBytesPerCustomer != def.TicketsMaxStorageBytesPerCustomer {
		t.Errorf("max storage = %d, want %d", out.TicketsMaxStorageBytesPerCustomer, def.TicketsMaxStorageBytesPerCustomer)
	}
}

func TestNormalizeRejectsNegativeLimitsAndInvertedBody(t *testing.T) {
	if _, err := NormalizeAppConfig(Config{TicketsMaxOpenPerCustomer: -1}); err == nil {
		t.Error("negative max open should fail")
	}
	if _, err := NormalizeAppConfig(Config{TicketsMinBodyChars: 100, TicketsMaxBodyChars: 50}); err == nil {
		t.Error("min > max body should fail")
	}
}

func configureRequest() *pluginv1.ConfigureRequest {
	db, err := structpb.NewStruct(map[string]any{"value": "postgres://x"})
	if err != nil {
		panic(err)
	}
	return &pluginv1.ConfigureRequest{
		Config: []*pluginv1.ConfigEntry{
			{Key: "database_url", Value: db},
		},
	}
}
