package server

import (
	"net/http"
)

func (s *Server) handleMCPJSONRPC(w http.ResponseWriter, r *http.Request) {
	if s.mcpHandler != nil {
		s.mcpHandler.HandleJSONRPC(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "MCP server is not initialized")
}

func (s *Server) handleMCPSSE(w http.ResponseWriter, r *http.Request) {
	if s.mcpHandler != nil {
		s.mcpHandler.HandleSSE(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "MCP server is not initialized")
}

func (s *Server) handleMCPMessage(w http.ResponseWriter, r *http.Request) {
	if s.mcpHandler != nil {
		s.mcpHandler.HandleMessage(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "MCP server is not initialized")
}

func (s *Server) handleGatewayChatCompletions(w http.ResponseWriter, r *http.Request) {
	if s.gateway != nil {
		s.gateway.HandleChatCompletions(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "gateway is not initialized")
}

func (s *Server) handleGatewayMessages(w http.ResponseWriter, r *http.Request) {
	if s.gateway != nil {
		s.gateway.HandleMessages(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "gateway is not initialized")
}

func (s *Server) handleGatewayModels(w http.ResponseWriter, r *http.Request) {
	if s.gateway != nil {
		s.gateway.HandleModels(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "gateway is not initialized")
}

func (s *Server) handleGatewayRouterStatus(w http.ResponseWriter, r *http.Request) {
	if s.gateway != nil {
		s.gateway.HandleRouterStatus(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "gateway is not initialized")
}

func (s *Server) handleGatewayRouterReset(w http.ResponseWriter, r *http.Request) {
	if s.gateway != nil {
		s.gateway.HandleRouterReset(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "gateway is not initialized")
}

func (s *Server) handleGatewayRouterStrategy(w http.ResponseWriter, r *http.Request) {
	if s.gateway != nil {
		s.gateway.HandleRouterStrategy(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "gateway is not initialized")
}
