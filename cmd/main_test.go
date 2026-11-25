package main

import (
	"github.com/caarlos0/env/v11"
	"testing"
)

func TestResponseEnvironment(t *testing.T) {
	values := map[string]string{"IPQUERY_DEFAULT_FORMAT": "auto", "IPQUERY_COLOR_ENABLED": "true", "IPQUERY_ALIGN_KEYS": "true", "IPQUERY_JSON_INDENT": "2"}
	for key, value := range values {
		t.Setenv(key, value)
	}
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Response.DefaultFormat != "auto" || !cfg.Response.ColorEnabled || !cfg.Response.AlignKeys || cfg.Response.Indent != 2 {
		t.Fatalf("%+v", cfg.Response)
	}
	t.Setenv("IPQUERY_DEFAULT_FORMAT", "json")
	t.Setenv("IPQUERY_COLOR_ENABLED", "false")
	t.Setenv("IPQUERY_ALIGN_KEYS", "false")
	t.Setenv("IPQUERY_JSON_INDENT", "4")
	if err := env.Parse(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Response.DefaultFormat != "json" || cfg.Response.ColorEnabled || cfg.Response.AlignKeys || cfg.Response.Indent != 4 {
		t.Fatalf("%+v", cfg.Response)
	}
	t.Setenv("IPQUERY_COLOR_ENABLED", "invalid")
	if err := env.Parse(&cfg); err == nil {
		t.Fatal("invalid boolean accepted")
	}
}
