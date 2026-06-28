package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"branchscribe/backend/internal/generation"
)

//go:embed web/*
var webFiles embed.FS

type debugSession struct {
	RequestID    string                   `json:"request_id"`
	Timestamp    time.Time                `json:"timestamp"`
	Provider     string                   `json:"provider"`
	BaseURL      string                   `json:"base_url"`
	Model        string                   `json:"model"`
	Messages     []generation.ChatMessage `json:"messages"`
	Temperature  float64                  `json:"temperature"`
	TopP         float64                  `json:"top_p"`
	MaxTokens    int                      `json:"max_tokens"`
	Stream       bool                     `json:"stream"`
	Content      string                   `json:"content"`
	Reasoning    string                   `json:"reasoning"`
	InputTokens  int                      `json:"input_tokens"`
	OutputTokens int                      `json:"output_tokens"`
	Status       string                   `json:"status"`
	Error        string                   `json:"error"`
}

type debugHub struct {
	mu       sync.RWMutex
	sessions map[string]*debugSession
	order    []string
	clients  map[chan []byte]struct{}
}

func newDebugHub() *debugHub {
	return &debugHub{
		sessions: make(map[string]*debugSession),
		clients:  make(map[chan []byte]struct{}),
	}
}

func main() {
	addr := flag.String("addr", "127.0.0.1:6069", "debug listener address")
	flag.Parse()

	hub := newDebugHub()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", hub.handleEvent)
	mux.HandleFunc("GET /api/sessions", hub.handleSessions)
	mux.HandleFunc("DELETE /api/sessions", hub.handleClear)
	mux.HandleFunc("GET /api/stream", hub.handleStream)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	webRoot, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(webRoot)))

	log.Printf("LLM debug UI: http://%s", *addr)
	log.Printf("Start the backend with LLM_DEBUG_URL=http://%s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

func (h *debugHub) handleEvent(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var event generation.DebugEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid event", http.StatusBadRequest)
		return
	}
	h.apply(event)
	h.printSummary(event)
	payload, _ := json.Marshal(event)
	h.broadcast(payload)
	w.WriteHeader(http.StatusNoContent)
}

func (h *debugHub) apply(event generation.DebugEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	session, exists := h.sessions[event.RequestID]
	if !exists {
		session = &debugSession{RequestID: event.RequestID, Timestamp: event.Timestamp, Status: "running"}
		h.sessions[event.RequestID] = session
		h.order = append([]string{event.RequestID}, h.order...)
		if len(h.order) > 100 {
			delete(h.sessions, h.order[len(h.order)-1])
			h.order = h.order[:len(h.order)-1]
		}
	}
	switch event.Type {
	case "request":
		session.Timestamp = event.Timestamp
		session.Provider = event.Provider
		session.BaseURL = event.BaseURL
		session.Model = event.Model
		session.Messages = event.Messages
		session.Temperature = event.Temperature
		session.TopP = event.TopP
		session.MaxTokens = event.MaxTokens
		session.Stream = event.Stream
		session.Status = "running"
	case "delta":
		session.Content += event.Content
	case "reasoning":
		session.Reasoning += event.Reasoning
	case "response":
		session.Content = event.Content
		session.Reasoning = event.Reasoning
		session.InputTokens = event.InputTokens
		session.OutputTokens = event.OutputTokens
		session.Status = "done"
	case "done":
		session.InputTokens = event.InputTokens
		session.OutputTokens = event.OutputTokens
		session.Status = "done"
	case "error":
		session.Error = event.Error
		session.Status = "error"
	}
}

func (h *debugHub) handleSessions(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	sessions := make([]debugSession, 0, len(h.order))
	for _, id := range h.order {
		if session := h.sessions[id]; session != nil {
			sessions = append(sessions, *session)
		}
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (h *debugHub) handleClear(w http.ResponseWriter, _ *http.Request) {
	h.mu.Lock()
	h.sessions = make(map[string]*debugSession)
	h.order = nil
	h.mu.Unlock()
	h.broadcast([]byte(`{"type":"clear"}`))
	w.WriteHeader(http.StatusNoContent)
}

func (h *debugHub) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	client := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, client)
		h.mu.Unlock()
	}()

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	for {
		select {
		case payload := <-client:
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *debugHub) broadcast(payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		select {
		case client <- payload:
		default:
		}
	}
}

func (h *debugHub) printSummary(event generation.DebugEvent) {
	switch event.Type {
	case "request":
		fmt.Fprintf(os.Stdout, "[%s] request model=%s messages=%d stream=%t\n", event.RequestID, event.Model, len(event.Messages), event.Stream)
	case "done", "response":
		fmt.Fprintf(os.Stdout, "[%s] done input_tokens=%d output_tokens=%d\n", event.RequestID, event.InputTokens, event.OutputTokens)
	case "error":
		fmt.Fprintf(os.Stdout, "[%s] error: %s\n", event.RequestID, event.Error)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
