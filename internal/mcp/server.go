package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

// ToolHandler represents an execution function for an MCP tool
type ToolHandler func(ctx context.Context, args map[string]any) (*CallToolResult, error)

type registeredTool struct {
	tool    Tool
	handler ToolHandler
}

// Server handles JSON-RPC 2.0 requests following the MCP specification
type Server struct {
	mu      sync.RWMutex
	tools   map[string]registeredTool
	name    string
	version string
}

// NewServer creates a new Server instance with empty tool registry
func NewServer() *Server {
	ver := config.Version
	if ver == "" {
		ver = "2.1.0"
	}
	return &Server{
		tools:   make(map[string]registeredTool),
		name:    ServerName,
		version: ver,
	}
}

// RegisterTool registers an invokable MCP tool with its metadata schema and handler
func (s *Server) RegisterTool(tool Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = registeredTool{
		tool:    tool,
		handler: handler,
	}
}

// GetTools returns a sorted list of registered tools
func (s *Server) GetTools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]Tool, 0, len(s.tools))
	for _, rt := range s.tools {
		tools = append(tools, rt.tool)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools
}

// HandleRequest processes an incoming JSON-RPC 2.0 request and returns the response.
// If the message is a notification (no ID), it returns nil.
func (s *Server) HandleRequest(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	if req == nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &JSONRPCError{
				Code:    CodeInvalidRequest,
				Message: "Invalid Request: null request body",
			},
		}
	}

	isNotification := (req.ID == nil)

	switch req.Method {
	case "initialize":
		res := InitializeResult{
			ProtocolVersion: ProtocolVersion,
			Capabilities: ServerCapabilities{
				Tools: &ToolsCapability{
					ListChanged: false,
				},
			},
			ServerInfo: Implementation{
				Name:    s.name,
				Version: s.version,
			},
		}
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  res,
		}

	case "notifications/initialized":
		if isNotification {
			return nil
		}
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{},
		}

	case "ping":
		if isNotification {
			return nil
		}
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{},
		}

	case "tools/list":
		if isNotification {
			return nil
		}
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: ListToolsResult{
				Tools: s.GetTools(),
			},
		}

	case "tools/call":
		if isNotification {
			return nil
		}

		var params CallToolParams
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return &JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &JSONRPCError{
						Code:    CodeInvalidParams,
						Message: fmt.Sprintf("Failed to parse tool call params: %v", err),
					},
				}
			}
		}

		if params.Name == "" {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    CodeInvalidParams,
					Message: "Missing required tool name in params",
				},
			}
		}

		s.mu.RLock()
		rt, exists := s.tools[params.Name]
		s.mu.RUnlock()

		if !exists {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  NewErrorResult(fmt.Sprintf("Tool %q not found", params.Name)),
			}
		}

		if params.Arguments == nil {
			params.Arguments = make(map[string]any)
		}

		result, err := rt.handler(ctx, params.Arguments)
		if err != nil {
			// MCP standard: tool execution errors are reported as CallToolResult with isError: true
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  NewErrorResult(err.Error()),
			}
		}

		if result == nil {
			result = NewTextResult("")
		}

		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}

	default:
		if isNotification {
			return nil
		}
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    CodeMethodNotFound,
				Message: fmt.Sprintf("Method %q not supported", req.Method),
			},
		}
	}
}

// RegisterTypedTool registers a tool whose input schema is derived from the
// struct tags of T and whose arguments are decoded into T before invoking handler.
//
// Supported field tags: `json:"name"` (required; "-" skips), `description:"..."`,
// `required:"true"` and `enum:"a,b"`. Supported field kinds: string, bool, int,
// []string, map[string]string, structs and slices of structs, and pointers to
// the scalar kinds (nil means the argument was absent, enabling defaults).
// Decoding coerces loosely-typed values exactly like the legacy getString,
// getBool, getInt, getStringSlice and getStringMap helpers.
func RegisterTypedTool[T any](s *Server, name, description string, handler func(ctx context.Context, params T) (*CallToolResult, error)) {
	var zero T
	schema := ToolInputSchema{Type: "object"}
	obj := schemaForStruct(reflect.TypeOf(zero))
	schema.Properties = obj.Properties
	schema.Required = obj.Required
	s.RegisterTool(Tool{Name: name, Description: description, InputSchema: schema}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		var params T
		decodeArgs(reflect.ValueOf(&params).Elem(), args)
		return handler(ctx, params)
	})
}

func jsonFieldName(f reflect.StructField) (string, bool) {
	if !f.IsExported() {
		return "", false
	}
	name := strings.Split(f.Tag.Get("json"), ",")[0]
	if name == "" || name == "-" {
		return "", false
	}
	return name, true
}

func schemaForStruct(t reflect.Type) PropertySchema {
	ps := PropertySchema{Type: "object", Properties: map[string]PropertySchema{}}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, ok := jsonFieldName(f)
		if !ok {
			continue
		}
		prop := schemaForType(f.Type)
		prop.Description = f.Tag.Get("description")
		if e := f.Tag.Get("enum"); e != "" {
			prop.Enum = strings.Split(e, ",")
		}
		ps.Properties[name] = prop
		if f.Tag.Get("required") == "true" {
			ps.Required = append(ps.Required, name)
		}
	}
	return ps
}

func schemaForType(t reflect.Type) PropertySchema {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return PropertySchema{Type: "string"}
	case reflect.Bool:
		return PropertySchema{Type: "boolean"}
	case reflect.Int, reflect.Int64:
		return PropertySchema{Type: "integer"}
	case reflect.Slice:
		item := schemaForType(t.Elem())
		return PropertySchema{Type: "array", Items: &item}
	case reflect.Map:
		return PropertySchema{Type: "object"}
	case reflect.Struct:
		return schemaForStruct(t)
	}
	return PropertySchema{Type: "string"}
}

func decodeArgs(dst reflect.Value, args map[string]any) {
	t := dst.Type()
	for i := 0; i < t.NumField(); i++ {
		name, ok := jsonFieldName(t.Field(i))
		if !ok {
			continue
		}
		decodeValue(dst.Field(i), args[name])
	}
}

func decodeValue(dst reflect.Value, val any) {
	if dst.Kind() == reflect.Ptr {
		if val == nil {
			return
		}
		dst.Set(reflect.New(dst.Type().Elem()))
		decodeValue(dst.Elem(), val)
		return
	}
	switch dst.Kind() {
	case reflect.String:
		if val == nil {
			return
		}
		if s, ok := val.(string); ok {
			dst.SetString(strings.TrimSpace(s))
		} else {
			dst.SetString(fmt.Sprintf("%v", val))
		}
	case reflect.Bool:
		switch v := val.(type) {
		case bool:
			dst.SetBool(v)
		case string:
			b, _ := strconv.ParseBool(v)
			dst.SetBool(b)
		}
	case reflect.Int, reflect.Int64:
		switch v := val.(type) {
		case float64:
			dst.SetInt(int64(v))
		case int:
			dst.SetInt(int64(v))
		case int64:
			dst.SetInt(v)
		case string:
			if parsed, err := strconv.Atoi(v); err == nil {
				dst.SetInt(int64(parsed))
			}
		}
	case reflect.Map:
		res := reflect.MakeMap(dst.Type())
		if m, ok := val.(map[string]any); ok {
			for k, v := range m {
				res.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(fmt.Sprintf("%v", v)))
			}
		}
		dst.Set(res)
	case reflect.Slice:
		items, ok := val.([]any)
		if !ok {
			if ss, isSS := val.([]string); isSS && dst.Type().Elem().Kind() == reflect.String {
				dst.Set(reflect.ValueOf(ss))
			}
			return
		}
		out := reflect.MakeSlice(dst.Type(), 0, len(items))
		for _, item := range items {
			if item == nil {
				continue
			}
			elem := reflect.New(dst.Type().Elem()).Elem()
			decodeValue(elem, item)
			out = reflect.Append(out, elem)
		}
		dst.Set(out)
	case reflect.Struct:
		if m, ok := val.(map[string]any); ok {
			decodeArgs(dst, m)
		}
	}
}

// intOr returns *p, or def when the argument was absent.
func intOr(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}
