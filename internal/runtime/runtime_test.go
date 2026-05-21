package runtime

import (
	"context"
	"testing"

	pluginv1 "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginproto/continuum/plugin/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestConfigureRejectsMissingDatabaseURL(t *testing.T) {
	s := New(nil, nil)
	req := &pluginv1.ConfigureRequest{Config: []*pluginv1.ConfigEntry{}}
	if _, err := s.Configure(context.Background(), req); err == nil {
		t.Fatal("expected missing database_url to fail; got nil")
	}
}

func TestConfigureDefaultsKBOnOthersOff(t *testing.T) {
	var observed Config
	s := New(nil, func(cfg Config) error {
		observed = cfg
		return nil
	})
	if _, err := s.Configure(context.Background(), configureRequest()); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if !observed.Modules.KB {
		t.Fatalf("KB should default ON now that the module ships; got %+v", observed.Modules)
	}
	if observed.Modules.Speedtest || observed.Modules.Tickets || observed.Modules.AI {
		t.Fatalf("non-shipped modules should still default off; got %+v", observed.Modules)
	}
	if observed.DatabaseURL != "postgres://x" {
		t.Fatalf("DatabaseURL = %q, want postgres://x", observed.DatabaseURL)
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
