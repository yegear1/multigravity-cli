package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type sseSession struct {
	id     string
	sendCh chan *JSONRPCResponse
}

// HTTPHandler manages HTTP-based transports (stateless JSON-RPC and SSE) for MCP
type HTTPHandler struct {
	server   *Server
	mu       sync.RWMutex
	sessions map[string]*sseSession
}

// NewHTTPHandler creates a new HTTPHandler wrapping an MCP Server
func NewHTTPHandler(s *Server) *HTTPHandler {
	return &HTTPHandler{
		server:   s,
		sessions: make(map[string]*sseSession),
	}
}

// HandleJSONRPC handles direct stateless JSON-RPC POST requests
func (h *HTTPHandler) HandleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed: MCP JSON-RPC endpoint requires POST", http.StatusMethodNotAllowed)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(&JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &JSONRPCError{
				Code:    CodeParseError,
				Message: fmt.Sprintf("Parse error: %v", err),
			},
		})
		return
	}

	resp := h.server.HandleRequest(r.Context(), &req)
	w.Header().Set("Content-Type", "application/json")
	if resp == nil {
		// Notification without response
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// HandleSSE handles the SSE endpoint (GET) for stateful MCP streaming connections
func (h *HTTPHandler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported by client connection", http.StatusInternalServerError)
		return
	}

	sessionID := generateSessionID()
	sess := &sseSession{
		id:     sessionID,
		sendCh: make(chan *JSONRPCResponse, 32),
	}

	h.mu.Lock()
	h.sessions[sessionID] = sess
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.sessions, sessionID)
		h.mu.Unlock()
		close(sess.sendCh)
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Determine messages endpoint based on request URI path
	basePath := r.URL.Path
	messagesEndpoint := "/mcp/messages"
	if strings.HasSuffix(basePath, "/sse") {
		messagesEndpoint = strings.TrimSuffix(basePath, "/sse") + "/messages"
		if !strings.HasPrefix(messagesEndpoint, "/") {
			messagesEndpoint = "/" + messagesEndpoint
		}
	}
	endpointURL := fmt.Sprintf("%s?sessionId=%s", messagesEndpoint, sessionID)

	// Send endpoint event according to MCP specification
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case resp, ok := <-sess.sendCh:
			if !ok {
				return
			}
			if resp != nil {
				data, err := json.Marshal(resp)
				if err == nil {
					fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(data))
					flusher.Flush()
				}
			}
		}
	}
}

// HandleMessage handles message posts (POST) associated with an active SSE session
func (h *HTTPHandler) HandleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "Missing required query parameter sessionId", http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	sess, exists := h.sessions[sessionID]
	h.mu.RUnlock()

	if !exists {
		http.Error(w, "Invalid or expired sessionId", http.StatusNotFound)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(&JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &JSONRPCError{
				Code:    CodeParseError,
				Message: fmt.Sprintf("Parse error: %v", err),
			},
		})
		return
	}

	resp := h.server.HandleRequest(r.Context(), &req)

	// Dispatch to SSE stream if session is open
	if resp != nil {
		select {
		case sess.sendCh <- resp:
		default:
			// Buffer full, response will also be returned in HTTP response
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if resp != nil {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	} else {
		w.WriteHeader(http.StatusAccepted)
	}
}

// StartHTTPServer runs a standalone HTTP MCP server on the specified host and port
func (s *Server) StartHTTPServer(ctx context.Context, host string, port int) error {
	if host == "" {
		host = "127.0.0.1"
	}
	if port <= 0 {
		port = 8990
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	handler := NewHTTPHandler(s)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /mcp", handler.HandleJSONRPC)
	mux.HandleFunc("GET /mcp/sse", handler.HandleSSE)
	mux.HandleFunc("POST /mcp/messages", handler.HandleMessage)
	mux.HandleFunc("GET /sse", handler.HandleSSE)
	mux.HandleFunc("POST /messages", handler.HandleMessage)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind MCP server to %s: %w", addr, err)
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		return err
	}
}
