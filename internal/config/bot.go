package config

import "github.com/caarlos0/env/v11"

type BotConfig struct {
	WSURL   string `env:"WS_URL" envDefault:"ws://localhost:8080/ws"`
	AgentID string `env:"AGENT_ID" envDefault:"bot"`
	APIKey  string `env:"API_KEY" envDefault:""`
}

func LoadBot() (BotConfig, error) {
	var cfg BotConfig
	err := env.Parse(&cfg)
	return cfg, err
}
