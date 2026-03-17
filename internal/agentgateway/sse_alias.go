package agentgateway

import (
	"net/http"

	agstream "ai-porker-arena/internal/agentgateway/stream"
)

func WriteSSE(w http.ResponseWriter, ev StreamEvent) error {
	return agstream.WriteSSE(w, ev)
}
