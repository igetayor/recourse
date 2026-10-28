package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"

	"github.com/igetayor/recourse/internal/auth"
	"github.com/igetayor/recourse/internal/config"
	"github.com/igetayor/recourse/internal/graph"
	"github.com/igetayor/recourse/internal/logger"
)

// NewRouter builds and returns the chi router with all middleware and routes.
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

	// ─── Health ───────────────────────────────────────────────────
	r.Get("/health", healthHandler)
	r.Get("/ready", healthHandler)

	// ─── GraphQL ──────────────────────────────────────────────────
	r.Post("/graphql", graph.Handler(schema).ServeHTTP)
	r.Get("/graphql", graphiQLHandler)

	// ─── REST convenience endpoints ───────────────────────────────
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", restRegisterHandler(schema))
		r.Post("/auth/login", restLoginHandler(schema))
		r.Get("/disputes", restListDisputesHandler(schema))
		r.Get("/disputes/{id}", restGetDisputeHandler(schema))
	})

	// ─── Static documentation frontend ────────────────────────────
	webDir := "web"
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		webDir = "/web" // container path
	}
	fs := http.FileServer(http.Dir(webDir))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
	})
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
	})
	r.Get("/style.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "style.css"))
	})
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

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

// ─────────────────────────── Handlers ───────────────────────────

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

// REST wrappers delegate to the GraphQL resolvers for DRY logic.
func restRegisterHandler(schema *graph.Schema) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			FullName string `json:"fullName"`
			Role     string `json:"role,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		req := &graph.Request{
			OperationName: "Register",
			Query:         "mutation Register($input: RegisterInput!) { register(input: $input) { token expiresIn user { id email fullName role } } }",
			Variables: map[string]any{"input": map[string]any{
				"email":    body.Email,
				"password": body.Password,
				"fullName": body.FullName,
				"role":     body.Role,
			}},
		}
		resp := schema.Execute(r.Context(), req)
		writeGQLResponse(w, resp)
	}
}

func restLoginHandler(schema *graph.Schema) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		req := &graph.Request{
			OperationName: "Login",
			Query:         "mutation Login($input: LoginInput!) { login(input: $input) { token expiresIn user { id email fullName role } } }",
			Variables:     map[string]any{"input": map[string]any{"email": body.Email, "password": body.Password}},
		}
		resp := schema.Execute(r.Context(), req)
		writeGQLResponse(w, resp)
	}
}

func restListDisputesHandler(schema *graph.Schema) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusFilter := r.URL.Query().Get("status")
		vars := map[string]any{}
		if statusFilter != "" {
			vars["status"] = statusFilter
		}
		req := &graph.Request{
			OperationName: "Disputes",
			Query:         "query Disputes($status: DisputeStatus) { disputes(status: $status) { id title amount currency status createdAt } }",
			Variables:     vars,
		}
		resp := schema.Execute(r.Context(), req)
		writeGQLResponse(w, resp)
	}
}

func restGetDisputeHandler(schema *graph.Schema) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		req := &graph.Request{
			OperationName: "Dispute",
			Query:         "query Dispute($id: ID!) { dispute(id: $id) { id title description amount currency status resolutionNotes createdAt updatedAt createdBy { id email fullName } assignee { id email fullName } } }",
			Variables:     map[string]any{"id": id},
		}
		resp := schema.Execute(r.Context(), req)
		writeGQLResponse(w, resp)
	}
}

func writeGQLResponse(w http.ResponseWriter, resp *graph.Response) {
	w.Header().Set("Content-Type", "application/json")
	if len(resp.Errors) > 0 {
		msg := resp.Errors[0].Message
		status := http.StatusBadRequest
		if contains(msg, "unauthenticated", "authentication") {
			status = http.StatusUnauthorized
		} else if contains(msg, "forbidden", "permission", "role") {
			status = http.StatusForbidden
		} else if contains(msg, "not found") {
			status = http.StatusNotFound
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
		return
	}
	_ = json.NewEncoder(w).Encode(resp.Data)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

func contains(s string, subs ...string) bool {
	sl := strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(sl, sub) {
			return true
		}
	}
	return false
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
