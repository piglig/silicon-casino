package config

import "github.com/caarlos0/env/v11"

type ServerConfig struct {
	PostgresDSN string `env:"POSTGRES_DSN,required,notEmpty"`
	HTTPAddr    string `env:"HTTP_ADDR" envDefault:":8080"`

	AdminAPIKey string `env:"ADMIN_API_KEY"`

	Agent1Name string `env:"AGENT1_NAME"`
	Agent1Key  string `env:"AGENT1_KEY"`
	Agent2Name string `env:"AGENT2_NAME"`
	Agent2Key  string `env:"AGENT2_KEY"`

	CCPerUSD          float64 `env:"CC_PER_USD" envDefault:"1000"`
	OpenAIPricePer1K  float64 `env:"OPENAI_PRICE_PER_1K_USD" envDefault:"0.0001"`
	KimiPricePer1K    float64 `env:"KIMI_PRICE_PER_1K_USD" envDefault:"0.0001"`
	OpenAIWeight      float64 `env:"OPENAI_WEIGHT" envDefault:"1.0"`
	KimiWeight        float64 `env:"KIMI_WEIGHT" envDefault:"1.0"`
	MaxBudgetUSD      float64 `env:"MAX_BUDGET_USD" envDefault:"20"`
	BindCooldownMins  int     `env:"BIND_KEY_COOLDOWN_MINUTES" envDefault:"60"`
	OpenAIBaseURL     string  `env:"OPENAI_BASE_URL" envDefault:"https://api.openai.com/v1"`
	KimiBaseURL       string  `env:"KIMI_BASE_URL" envDefault:"https://api.moonshot.ai/v1"`
	AllowAnyVendorKey bool    `env:"ALLOW_ANY_VENDOR_KEY" envDefault:"false"`

	SpectatorPushEnabled               bool   `env:"SPECTATOR_PUSH_ENABLED" envDefault:"false"`
	SpectatorPushConfigPath            string `env:"SPECTATOR_PUSH_CONFIG_PATH"`
	SpectatorPushConfigJSON            string `env:"SPECTATOR_PUSH_CONFIG_JSON"`
	SpectatorPushConfigReloadMS        int    `env:"SPECTATOR_PUSH_CONFIG_RELOAD_MS" envDefault:"1000"`
	SpectatorPushWorkers               int    `env:"SPECTATOR_PUSH_WORKERS" envDefault:"4"`
	SpectatorPushRetryMax              int    `env:"SPECTATOR_PUSH_RETRY_MAX" envDefault:"5"`
	SpectatorPushRetryBaseMS           int    `env:"SPECTATOR_PUSH_RETRY_BASE_MS" envDefault:"500"`
	SpectatorPushSnapshotMinIntervalMS int    `env:"SPECTATOR_PUSH_SNAPSHOT_MIN_INTERVAL_MS" envDefault:"3000"`
}

func LoadServer() (ServerConfig, error) {
	var cfg ServerConfig
	err := env.Parse(&cfg)
	return cfg, err
}
