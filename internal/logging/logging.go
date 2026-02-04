package logging

import (
	"io"
	"os"

	"silicon-casino/internal/config"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Init(cfg config.LogConfig) {
	level := zerolog.InfoLevel
	if parsed, err := zerolog.ParseLevel(cfg.Level); err == nil {
		level = parsed
	}

	var output io.Writer = os.Stdout
	if cfg.Pretty {
		output = zerolog.ConsoleWriter{Out: os.Stdout}
	}

	zerolog.SetGlobalLevel(level)
	logger := zerolog.New(output).With().Timestamp().Logger()
	if n := cfg.SampleEvery; n > 1 {
		logger = logger.Sample(&zerolog.BasicSampler{N: uint32(n)})
	}
	log.Logger = logger
}
