// Package graph provides the GraphQL execution engine, handler, and type system
// for the Recourse API. It implements a hand-rolled GraphQL server compatible
// with the standard GraphQL-over-HTTP spec, avoiding gqlgen codegen dependencies.
package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ─────────────────────── GraphQL Request/Response ───────────────────────

// Request is a standard GraphQL HTTP request body.
type Request struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName,omitempty"`
	Variables     map[string]any `json:"variables,omitempty"`
}

// Response is a standard GraphQL HTTP response.
type Response struct {
	Data   any      `json:"data,omitempty"`
	Errors []GQLErr `json:"errors,omitempty"`
}

// GQLErr is a GraphQL error object per spec.
type GQLErr struct {
	Message string `json:"message"`
}

// ─────────────────────── ExecutionSchema ───────────────────────

// Schema wraps the root resolver and dispatches operations.
type Schema struct {
	resolver *Resolver
}

// NewSchema constructs the executable schema.
func NewSchema(r *Resolver) *Schema {
	return &Schema{resolver: r}
}

// Execute dispatches a GraphQL request to the appropriate resolver.
func (s *Schema) Execute(ctx context.Context, req *Request) *Response {
	q := strings.TrimSpace(req.Query)

	// Naive operation type detection
	op := "query"
	if strings.HasPrefix(strings.ToLower(q), "mutation") {
		op = "mutation"
	}

	switch op {
	case "query":
		return s.execQuery(ctx, req)
	case "mutation":
		return s.execMutation(ctx, req)
	default:
		return errResp("unknown operation type")
	}
}

func (s *Schema) execQuery(ctx context.Context, req *Request) *Response {
	q := s.resolver.Query()
	name := extractOperationName(req)

	switch name {
	case "me", "Me":
		user, err := q.Me(ctx)
		return respond("me", user, err)

	case "users", "Users":
		users, err := q.Users(ctx)
		return respond("users", users, err)

	case "dispute", "Dispute":
		id, _ := req.Variables["id"].(string)
		d, err := q.Dispute(ctx, id)
		return respond("dispute", d, err)

	case "disputes", "Disputes":
		var statusFilter *DisputeStatus
		if sv, ok := req.Variables["status"].(string); ok && sv != "" {
			s2 := DisputeStatus(sv)
			statusFilter = &s2
		}
		disputes, err := q.Disputes(ctx, statusFilter)
		return respond("disputes", disputes, err)

	default:
		// Try to infer from query text
		if strings.Contains(req.Query, "me {") || strings.Contains(req.Query, "me{") {
			user, err := q.Me(ctx)
			return respond("me", user, err)
		}
		if strings.Contains(req.Query, "users {") || strings.Contains(req.Query, "users{") {
			users, err := q.Users(ctx)
			return respond("users", users, err)
		}
		if strings.Contains(req.Query, "disputes") {
			disputes, err := q.Disputes(ctx, nil)
			return respond("disputes", disputes, err)
		}
		return errResp(fmt.Sprintf("unknown query operation: %s", name))
	}
}

func (s *Schema) execMutation(ctx context.Context, req *Request) *Response {
	m := s.resolver.Mutation()
	name := extractOperationName(req)
	vars := req.Variables

	switch name {
	case "register", "Register":
		input := mapToRegisterInput(vars)
		payload, err := m.Register(ctx, input)
		return respond("register", payload, err)

	case "login", "Login":
		input := LoginInput{
			Email:    getString(vars, "email"),
			Password: getString(vars, "password"),
		}
		if inp, ok := vars["input"].(map[string]any); ok {
			input.Email = getString(inp, "email")
			input.Password = getString(inp, "password")
		}
		payload, err := m.Login(ctx, input)
		return respond("login", payload, err)

	case "createDispute", "CreateDispute":
		input := mapToCreateDisputeInput(vars)
		d, err := m.CreateDispute(ctx, input)
		return respond("createDispute", d, err)

	case "updateDispute", "UpdateDispute":
		id, _ := vars["id"].(string)
		input := mapToUpdateDisputeInput(vars)
		d, err := m.UpdateDispute(ctx, id, input)
		return respond("updateDispute", d, err)

	case "deleteDispute", "DeleteDispute":
		id, _ := vars["id"].(string)
		ok, err := m.DeleteDispute(ctx, id)
		return respond("deleteDispute", ok, err)

	case "updateUserRole", "UpdateUserRole":
		userID, _ := vars["userId"].(string)
		roleStr, _ := vars["role"].(string)
		user, err := m.UpdateUserRole(ctx, userID, Role(roleStr))
		return respond("updateUserRole", user, err)

	default:
		return errResp(fmt.Sprintf("unknown mutation: %s", name))
	}
}

// ─────────────────────── HTTP Handler ───────────────────────

// Handler returns an http.Handler serving the GraphQL endpoint.
func Handler(schema *Schema) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Query().Get("query") == "" {
			http.Error(w, "GraphQL requires POST with JSON body", http.StatusMethodNotAllowed)
			return
		}

		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(Response{Errors: []GQLErr{{Message: "invalid JSON: " + err.Error()}}})
			return
		}

		resp := schema.Execute(r.Context(), &req)
		w.Header().Set("Content-Type", "application/json")
		if len(resp.Errors) > 0 {
			w.WriteHeader(http.StatusOK) // GraphQL returns 200 even for logical errors
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// ─────────────────────── Helpers ───────────────────────

func respond(field string, data any, err error) *Response {
	if err != nil {
		return &Response{Errors: []GQLErr{{Message: err.Error()}}}
	}
	return &Response{Data: map[string]any{field: data}}
}

func errResp(msg string) *Response {
	return &Response{Errors: []GQLErr{{Message: msg}}}
}

func extractOperationName(req *Request) string {
	if req.OperationName != "" {
		return req.OperationName
	}
	// Cheap text extraction: find first word after query/mutation keyword
	q := strings.TrimSpace(req.Query)
	// Remove leading keyword
	for _, kw := range []string{"mutation ", "query "} {
		q = strings.TrimPrefix(strings.ToLower(q), kw)
	}
	// First token is operation name
	fields := strings.Fields(q)
	if len(fields) > 0 {
		name := strings.TrimPrefix(fields[0], "{")
		name = strings.Split(name, "(")[0]
		name = strings.Split(name, "{")[0]
		return strings.TrimSpace(name)
	}
	return ""
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getFloat(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0
}

func mapToRegisterInput(vars map[string]any) RegisterInput {
	inp := RegisterInput{}
	if input, ok := vars["input"].(map[string]any); ok {
		inp.Email = getString(input, "email")
		inp.Password = getString(input, "password")
		inp.FullName = getString(input, "fullName")
		if roleStr := getString(input, "role"); roleStr != "" {
			r := Role(roleStr)
			inp.Role = &r
		}
	}
	return inp
}

func mapToCreateDisputeInput(vars map[string]any) CreateDisputeInput {
	inp := CreateDisputeInput{}
	if input, ok := vars["input"].(map[string]any); ok {
		inp.Title = getString(input, "title")
		inp.Description = getString(input, "description")
		inp.Amount = getFloat(input, "amount")
		inp.Currency = getString(input, "currency")
	}
	return inp
}

func mapToUpdateDisputeInput(vars map[string]any) UpdateDisputeInput {
	inp := UpdateDisputeInput{}
	if input, ok := vars["input"].(map[string]any); ok {
		inp.Status = DisputeStatus(getString(input, "status"))
		if notes := getString(input, "resolutionNotes"); notes != "" {
			inp.ResolutionNotes = &notes
		}
		if aid := getString(input, "assigneeId"); aid != "" {
			inp.AssigneeID = &aid
		}
	}
	return inp
}
