package main

import (
	"encoding/json"
	"net/http"
	"sync"
)

type store struct {
	mu    sync.Mutex
	notes []string
}

func (s *store) handleNotes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var in struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || len(in.Text) == 0 || len(in.Text) > 280 {
			http.Error(w, "text must be 1 to 280 characters", http.StatusUnprocessableEntity)
			return
		}
		s.mu.Lock()
		s.notes = append(s.notes, in.Text)
		s.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	case http.MethodGet:
		s.mu.Lock()
		defer s.mu.Unlock()
		_ = json.NewEncoder(w).Encode(s.notes)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func newMux(s *store) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/notes", s.handleNotes)
	return mux
}

func main() {
	_ = http.ListenAndServe(":8080", newMux(&store{}))
}
