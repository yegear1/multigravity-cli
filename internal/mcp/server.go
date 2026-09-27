package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
