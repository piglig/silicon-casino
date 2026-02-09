package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"silicon-casino/internal/logging"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
)

func apiLogMiddleware() func(http.Handler) http.Handler {
	return httplog.RequestLogger(
		slog.New(slog.NewJSONHandler(logging.Writer(), &slog.HandlerOptions{})),
		&httplog.Options{
			Level:              slog.LevelInfo,
			Schema:             httplog.Schema{ResponseStatus: "status", ResponseDuration: "duration_ms"},
			LogRequestBody:     func(*http.Request) bool { return false },
			LogResponseBody:    func(*http.Request) bool { return false },
			LogRequestHeaders:  []string{},
			LogResponseHeaders: []string{},
			LogExtraAttrs: func(req *http.Request, _ string, _ int) []slog.Attr {
				rc := chi.RouteContext(req.Context())
				route := req.URL.Path
				if rc != nil && rc.RoutePattern() != "" {
					route = rc.RoutePattern()
				}
				return []slog.Attr{
					slog.String("request_id", chimw.GetReqID(req.Context())),
					slog.String("method", req.Method),
					slog.String("route", route),
					slog.String("path", req.URL.Path),
				}
			},
		},
	)
}

func bodyCaptureMiddleware(maxCaptureBytes int) func(http.Handler) http.Handler {
	if maxCaptureBytes <= 0 {
		maxCaptureBytes = 4096
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSSERequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			reqBody, err := io.ReadAll(r.Body)
			if err != nil {
				reqBody = nil
			}
			r.Body = io.NopCloser(bytes.NewReader(reqBody))

			cw := &captureWriter{ResponseWriter: w, maxBytes: maxCaptureBytes}
			next.ServeHTTP(cw, r)

			reqLog := reqBody
			if len(reqLog) > maxCaptureBytes {
				reqLog = reqLog[:maxCaptureBytes]
			}
			if len(reqLog) > 0 {
				httplog.SetAttrs(r.Context(), slog.Any("request_body", parseMaybeJSON(reqLog)))
			} else {
				httplog.SetAttrs(r.Context(), slog.Any("request_body", ""))
			}

			respLog := cw.body.Bytes()
			httplog.SetAttrs(r.Context(), slog.Any("response_body", parseMaybeJSON(respLog)))
			httplog.SetAttrs(r.Context(), slog.Bool("request_body_truncated", len(reqBody) > maxCaptureBytes))
			httplog.SetAttrs(r.Context(), slog.Bool("response_body_truncated", cw.truncated))
		})
	}
}

type captureWriter struct {
	http.ResponseWriter
	body      bytes.Buffer
	maxBytes  int
	truncated bool
}

func (c *captureWriter) Write(p []byte) (int, error) {
	if !c.truncated {
		remain := c.maxBytes - c.body.Len()
		if remain > 0 {
			if len(p) <= remain {
				_, _ = c.body.Write(p)
			} else {
				_, _ = c.body.Write(p[:remain])
				c.truncated = true
			}
		} else {
			c.truncated = true
		}
	}
	return c.ResponseWriter.Write(p)
}

func (c *captureWriter) Flush() {
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func parseMaybeJSON(b []byte) any {
	if len(b) == 0 {
		return ""
	}
	var out any
	if err := json.Unmarshal(b, &out); err == nil {
		return out
	}
	return string(b)
}

func writeHTTPError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": code})
}

func isSSERequest(r *http.Request) bool {
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		return true
	}
	path := r.URL.Path
	if strings.HasSuffix(path, "/events") && strings.Contains(path, "/api/agent/sessions/") {
		return true
	}
	if path == "/api/public/spectate/events" {
		return true
	}
	return false
}
