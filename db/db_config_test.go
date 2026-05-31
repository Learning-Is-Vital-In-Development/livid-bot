package db

import "testing"

func TestNewPoolConfigAttachesPgxTracer(t *testing.T) {
	cfg, err := newPoolConfig("postgres://livid:livid@localhost:15432/livid?sslmode=disable")
	if err != nil {
		t.Fatalf("newPoolConfig returned error: %v", err)
	}
	if cfg.ConnConfig.Tracer == nil {
		t.Fatal("expected pgx tracer to be configured")
	}
}
