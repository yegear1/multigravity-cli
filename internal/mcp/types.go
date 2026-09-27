package mcp

import "encoding/json"

// MCP Protocol Version
const (
	ProtocolVersion = "2024-11-05"
	ServerName      = "multigravity-mcp"
)

// Standard JSON-RPC 2.0 error codes
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// JSONRPCRequest represents a JSON-RPC 2.0 request or notification
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError holds standard JSON-RPC 2.0 error details
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Implementation defines the client or server identity
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeParams is sent by the client during handshake
type InitializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      Implementation `json:"clientInfo"`
}

// ToolsCapability indicates server tool support
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerCapabilities defines features supported by this MCP server
type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// InitializeResult is the server response to initialize
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
}

// PropertySchema defines a JSON Schema property for tool inputs
type PropertySchema struct {
	Type        string                    `json:"type"`
	Description string                    `json:"description,omitempty"`
	Enum        []string                  `json:"enum,omitempty"`
	Default     any                       `json:"default,omitempty"`
	Items       *PropertySchema           `json:"items,omitempty"`
	Properties  map[string]PropertySchema `json:"properties,omitempty"`
}

// ToolInputSchema defines the input argument contract for an MCP tool
type ToolInputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]PropertySchema `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
}

// Tool defines an invokable MCP tool
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema ToolInputSchema `json:"inputSchema"`
}

// ListToolsResult is the response to tools/list
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams contains parameters sent to tools/call
type CallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ContentItem represents a text or media chunk in a tool result
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CallToolResult is the response returned to the caller of tools/call
type CallToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// NewTextResult creates a successful CallToolResult containing formatted text
func NewTextResult(text string) *CallToolResult {
	return &CallToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: text,
			},
		},
		IsError: false,
	}
}

// NewErrorResult creates an error CallToolResult containing error text
func NewErrorResult(errMsg string) *CallToolResult {
	return &CallToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: errMsg,
			},
		},
		IsError: true,
	}
}
