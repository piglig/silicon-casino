package logging

import (
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Init() {
	level := zerolog.InfoLevel
	if v := strings.TrimSpace(os.Getenv("LOG_LEVEL")); v != "" {
		if parsed, err := zerolog.ParseLevel(strings.ToLower(v)); err == nil {
			level = parsed
		}
	}

	var output io.Writer = os.Stdout
	if isPretty(os.Getenv("LOG_PRETTY")) {
		output = zerolog.ConsoleWriter{Out: os.Stdout}
	}

	zerolog.SetGlobalLevel(level)
	logger := zerolog.New(output).With().Timestamp().Logger()
	if n := parseSampleEvery(); n > 1 {
		logger = logger.Sample(&zerolog.BasicSampler{N: uint32(n)})
	}
	log.Logger = logger
}

func isPretty(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}

func parseSampleEvery() int {
	raw := strings.TrimSpace(os.Getenv("LOG_SAMPLE_EVERY"))
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0
	}
	return n
}
