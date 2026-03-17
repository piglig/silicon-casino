package main

import (
	"ai-porker-arena/internal/agentgateway"
	"ai-porker-arena/internal/config"
	"ai-porker-arena/internal/store"
	httptransport "ai-porker-arena/internal/transport/http"

	"github.com/go-chi/chi/v5"
)

func newRouter(st *store.Store, cfg config.ServerConfig, agentCoord *agentgateway.Coordinator) *chi.Mux {
	return httptransport.NewRouter(st, cfg, agentCoord)
}

func logRoutes(r chi.Router) {
	httptransport.LogRoutes(r)
}
