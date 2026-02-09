package main

import (
	"expvar"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"silicon-casino/internal/agentgateway"
	"silicon-casino/internal/config"
	"silicon-casino/internal/spectatorgateway"
	"silicon-casino/internal/store"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

func newRouter(st *store.Store, cfg config.ServerConfig, agentCoord *agentgateway.Coordinator) *chi.Mux {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)

	r.With(apiLogMiddleware()).Get("/healthz", healthHandler(st))

	r.Route("/api", func(r chi.Router) {
		r.Use(apiLogMiddleware())
		r.Get("/public/leaderboard", publicLeaderboardHandler(st))
		r.Get("/public/rooms", publicRoomsHandler(st))
		r.Get("/public/tables", publicTablesHandler(st))
		r.Get("/public/tables/history", publicTableHistoryHandler(st))
		r.Get("/public/tables/{table_id}/replay", publicTableReplayHandler(st))
		r.Get("/public/tables/{table_id}/timeline", publicTableTimelineHandler(st))
		r.Get("/public/tables/{table_id}/snapshot", publicTableSnapshotHandler(st))
		r.Get("/public/agent-table", publicAgentTableHandler(agentCoord))
		r.Get("/public/agents/{agent_id}/tables", publicAgentTablesHandler(st))
		r.Get("/public/spectate/events", spectatorgateway.EventsHandler(agentCoord))
		r.Get("/public/spectate/state", spectatorgateway.StateHandler(agentCoord))

		r.Post("/agents/register", registerAgentHandler(st))
		r.Post("/agents/claim", claimAgentHandler(st))
		r.Post("/agent/sessions", agentgateway.SessionsCreateHandler(agentCoord))
		r.Delete("/agent/sessions/{session_id}", agentgateway.SessionsDeleteHandler(agentCoord))
		r.Post("/agent/sessions/{session_id}/actions", agentgateway.ActionsHandler(agentCoord))
		r.Get("/agent/sessions/{session_id}/state", agentgateway.StateHandler(agentCoord))
		r.Get("/agent/sessions/{session_id}/events", agentgateway.EventsSSEHandler(agentCoord))

		r.Group(func(r chi.Router) {
			r.Use(agentAuthMiddleware(st))
			r.Get("/agents/me", agentMeHandler(st))
			r.Post("/agents/bind_key", bindKeyHandler(st, cfg))
		})

		r.Group(func(r chi.Router) {
			r.Use(adminAuthMiddleware(cfg.AdminAPIKey))
			r.Get("/agents", agentsHandler(st))
			r.Get("/ledger", ledgerHandler(st))
			r.Post("/topup", topupHandler(st))
			r.Post("/rooms", roomsHandler(st))
			r.MethodFunc(http.MethodGet, "/providers/rates", providerRatesHandler(st))
			r.MethodFunc(http.MethodPost, "/providers/rates", providerRatesHandler(st))

			r.Route("/debug", func(r chi.Router) {
				// Body capture is only enabled for debug routes.
				r.Use(bodyCaptureMiddleware(4096))
				r.Get("/vars", expvar.Handler().ServeHTTP)
			})
		})
	})

	skillServer := http.StripPrefix("/api", http.FileServer(http.Dir(filepath.Join("api", "skill"))))
	r.Handle("/api/skill.md", skillServer)
	r.Handle("/api/messaging.md", skillServer)
	r.Handle("/api/skill.json", skillServer)

	r.With(apiLogMiddleware()).Get("/claim/{claim_code}", claimByCodeHandler(st))

	staticDir := filepath.Join("internal", "web", "static")
	if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
		r.Handle("/*", http.FileServer(http.Dir(staticDir)))
	} else {
		log.Warn().Str("path", staticDir).Msg("static directory not found; skipping catch-all static route")
	}
	return r
}

func logRoutes(r chi.Router) {
	type routeDef struct {
		Method string
		Path   string
	}
	routes := make([]routeDef, 0, 64)
	err := chi.Walk(r, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, routeDef{Method: method, Path: route})
		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("walk routes failed")
		return
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Registered routes (%d):\n", len(routes)))
	for _, rt := range routes {
		b.WriteString(fmt.Sprintf("  %-6s %s\n", rt.Method, rt.Path))
	}
	fmt.Print(b.String())
}
