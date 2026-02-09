package spectatorpush

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"silicon-casino/internal/config"
)

func ConfigFromServer(cfg config.ServerConfig) (Config, error) {
	out := Config{
		Enabled:             cfg.SpectatorPushEnabled,
		ConfigPath:          strings.TrimSpace(cfg.SpectatorPushConfigPath),
		ConfigReload:        time.Duration(cfg.SpectatorPushConfigReloadMS) * time.Millisecond,
		Workers:             cfg.SpectatorPushWorkers,
		RetryMax:            cfg.SpectatorPushRetryMax,
		RetryBase:           time.Duration(cfg.SpectatorPushRetryBaseMS) * time.Millisecond,
		SnapshotMinInterval: time.Duration(cfg.SpectatorPushSnapshotMinIntervalMS) * time.Millisecond,
		PanelUpdateInterval: time.Second,
		PanelRecentActions:  5,
		FailureThreshold:    3,
		CircuitOpenDuration: 30 * time.Second,
		RequestTimeout:      5 * time.Second,
		DispatchBuffer:      2048,
	}
	if !out.Enabled {
		return out, nil
	}

	if out.Workers <= 0 {
		out.Workers = 4
	}
	if out.RetryMax < 0 {
		out.RetryMax = 0
	}
	if out.RetryBase <= 0 {
		out.RetryBase = 500 * time.Millisecond
	}
	if out.SnapshotMinInterval <= 0 {
		out.SnapshotMinInterval = 3 * time.Second
	}
	if out.PanelUpdateInterval <= 0 {
		out.PanelUpdateInterval = time.Second
	}
	if out.PanelRecentActions <= 0 {
		out.PanelRecentActions = 5
	}
	if out.ConfigReload <= 0 {
		out.ConfigReload = time.Second
	}

	jsonRaw, err := loadTargetsConfigJSON(cfg)
	if err != nil {
		return Config{}, err
	}
	if jsonRaw == "" {
		return out, nil
	}
	targets, err := parseTargetsJSON(jsonRaw)
	if err != nil {
		return Config{}, err
	}
	out.Targets = targets
	return out, nil
}

func loadTargetsConfigJSON(cfg config.ServerConfig) (string, error) {
	path := strings.TrimSpace(cfg.SpectatorPushConfigPath)
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read spectator push config path %q: %w", path, err)
		}
		return strings.TrimSpace(string(raw)), nil
	}
	return strings.TrimSpace(cfg.SpectatorPushConfigJSON), nil
}

func parseTargetsJSON(jsonRaw string) ([]PushTarget, error) {
	var targets []PushTarget
	if err := json.Unmarshal([]byte(jsonRaw), &targets); err != nil {
		return nil, fmt.Errorf("parse spectator push targets: %w", err)
	}
	filtered := make([]PushTarget, 0, len(targets))
	for _, target := range targets {
		target.Platform = strings.ToLower(strings.TrimSpace(target.Platform))
		target.ScopeType = strings.ToLower(strings.TrimSpace(target.ScopeType))
		if target.ScopeType == "" {
			target.ScopeType = "room"
		}
		if target.ScopeType != "room" && target.ScopeType != "table" && target.ScopeType != "all" {
			continue
		}
		target.Endpoint = strings.TrimSpace(target.Endpoint)
		if target.Endpoint == "" {
			continue
		}
		if !target.Enabled {
			continue
		}
		for i := range target.EventAllowlist {
			target.EventAllowlist[i] = strings.TrimSpace(strings.ToLower(target.EventAllowlist[i]))
		}
		filtered = append(filtered, target)
	}
	return filtered, nil
}
