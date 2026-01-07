package mcp

import (
	"net/http"

	"github.com/goccy/go-json"
)

// HTTPHandler provides HTTP endpoints for MCP management.
type HTTPHandler struct {
	manager Manager
}

// NewHTTPHandler creates a new HTTP handler for MCP management.
func NewHTTPHandler(manager Manager) *HTTPHandler {
	return &HTTPHandler{manager: manager}
}

// ListClients handles GET /mcp/clients
func (h *HTTPHandler) ListClients(w http.ResponseWriter, r *http.Request) {
	clients := h.manager.GetClients()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"clients": clients,
		"count":   len(clients),
	})
}

// GetClient handles GET /mcp/clients/{id}
func (h *HTTPHandler) GetClient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	client, err := h.manager.GetClient(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(client)
}

// AddClient handles POST /mcp/clients
func (h *HTTPHandler) AddClient(w http.ResponseWriter, r *http.Request) {
	var cfg ClientConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.manager.AddClient(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "created",
		"id":     cfg.ID,
	})
}

// RemoveClient handles DELETE /mcp/clients/{id}
func (h *HTTPHandler) RemoveClient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	if err := h.manager.RemoveClient(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ReconnectClient handles POST /mcp/clients/{id}/reconnect
func (h *HTTPHandler) ReconnectClient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	if err := h.manager.ReconnectClient(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "reconnected",
		"id":     id,
	})
}

// ListTools handles GET /mcp/tools
func (h *HTTPHandler) ListTools(w http.ResponseWriter, r *http.Request) {
	tools := h.manager.GetAvailableTools(r.Context())

	// Extract tool names for simpler response
	toolNames := make([]string, len(tools))
	for i, t := range tools {
		toolNames[i] = t.Function.Name
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tools": toolNames,
		"count": len(tools),
	})
}
