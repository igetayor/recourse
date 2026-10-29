package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"

	"github.com/igetayor/recourse/internal/api/controllers"
	"github.com/igetayor/recourse/internal/api/services"
	"github.com/igetayor/recourse/internal/auth"
	"github.com/igetayor/recourse/internal/config"
	"github.com/igetayor/recourse/internal/graph"
	"github.com/igetayor/recourse/internal/logger"
)

// NewRouter builds and returns the chi router with full Controller-Service architecture.
// Wire order: Schema → Services → Controllers → Routes.
func NewRouter(cfg *config.Config, schema *graph.Schema, jwtService *auth.JWTService) http.Handler {
	r := chi.NewRouter()

	// ─── Core middleware ───────────────────────────────────────────
	r.Use(middleware.RealIP)
	r.Use(requestIDMiddleware)
	r.Use(requestLoggerMiddleware)
	r.Use(timeoutMiddleware(cfg.RequestTimeout))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(middleware.Recoverer)

	// ─── JWT injection (non-enforcing — resolvers enforce) ────────
	r.Use(auth.Middleware(jwtService))

	// ─── Wire services and controllers ────────────────────────────
	authSvc := services.NewAuthService(schema)
	disputeSvc := services.NewDisputeService(schema)
	authCtrl := controllers.NewAuthController(authSvc)
	disputeCtrl := controllers.NewDisputeController(disputeSvc)

	// ─── Health probes ────────────────────────────────────────────
	r.Get("/health", healthHandler)
	r.Get("/ready", healthHandler)

	// ─── GraphQL ──────────────────────────────────────────────────
	r.Post("/graphql", graph.Handler(schema).ServeHTTP)
	r.Get("/graphql", graphiQLHandler)

	// ─── REST convenience endpoints (Controller layer) ────────────
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", authCtrl.HandleRegister)
		r.Post("/auth/login", authCtrl.HandleLogin)
		r.Get("/disputes", disputeCtrl.HandleListDisputes)
		r.Get("/disputes/{id}", disputeCtrl.HandleGetDispute)
	})

	// ─── Static documentation frontend from web/ ─────────────────
	webDir := "web"
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		// Try relative to workspace or binary
		if _, err2 := os.Stat("igeTayor/recourse/web"); err2 == nil {
			webDir = "igeTayor/recourse/web"
		} else {
			webDir = "/web" // container path
		}
	}
	fileServer := http.FileServer(http.Dir(webDir))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
	})
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
	})
	r.Get("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		http.ServeFile(w, r, filepath.Join(webDir, "style.css"))
	})
	r.Get("/web/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		http.ServeFile(w, r, filepath.Join(webDir, "style.css"))
	})
	r.Handle("/web/*", http.StripPrefix("/web/", fileServer))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	return r
}

// ─────────────────────────── Middleware ───────────────────────────

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Request-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}
		w.Header().Set("X-Request-ID", traceID)
		ctx := logger.WithTraceID(r.Context(), traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		logger.FromContext(r.Context()).Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"latency_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

func timeoutMiddleware(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ─────────────────────────── System Handlers ───────────────────────────

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "recourse-api",
	})
}

func graphiQLHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(graphiQLPage))
}

// graphiQLPage is the embedded GraphiQL IDE HTML.
const graphiQLPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Recourse — GraphiQL</title>
<link rel="stylesheet" href="https://unpkg.com/graphiql/graphiql.min.css" />
</head>
<body style="margin:0">
<div id="graphiql" style="height:100vh"></div>
<script crossorigin src="https://unpkg.com/react/umd/react.production.min.js"></script>
<script crossorigin src="https://unpkg.com/react-dom/umd/react-dom.production.min.js"></script>
<script crossorigin src="https://unpkg.com/graphiql/graphiql.min.js"></script>
<script>
  const fetcher = GraphiQL.createFetcher({ url: '/graphql' });
  ReactDOM.render(React.createElement(GraphiQL, { fetcher }), document.getElementById('graphiql'));
</script>
</body></html>`
