package config

import "testing"

func TestLoadBotDefaults(t *testing.T) {
	cfg, err := LoadBot()
	if err != nil {
		t.Fatalf("LoadBot() error = %v", err)
	}
	if cfg.WSURL != "ws://localhost:8080/ws" {
		t.Fatalf("WSURL = %q, want ws://localhost:8080/ws", cfg.WSURL)
	}
	if cfg.AgentID != "bot" {
		t.Fatalf("AgentID = %q, want bot", cfg.AgentID)
	}
}

func TestLoadBotOverrides(t *testing.T) {
	t.Setenv("WS_URL", "ws://127.0.0.1:9000/ws")
	t.Setenv("AGENT_ID", "BotA")
	t.Setenv("API_KEY", "key-a")

	cfg, err := LoadBot()
	if err != nil {
		t.Fatalf("LoadBot() error = %v", err)
	}
	if cfg.WSURL != "ws://127.0.0.1:9000/ws" {
		t.Fatalf("WSURL = %q", cfg.WSURL)
	}
	if cfg.AgentID != "BotA" || cfg.APIKey != "key-a" {
		t.Fatalf("unexpected bot config: %+v", cfg)
	}
}
