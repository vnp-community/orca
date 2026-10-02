package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Resume (GET + Last-Event-ID) is served by the gateway, not the SDK: the SDK
// can only replay streams its own connection created, and only as a snapshot.
// Following the shared buffer instead means a client that lost its POST stream
// on replica A can resume on replica B and receive exactly the events after
// the id it saw, live, until the request's response has been delivered.

const sseHeartbeat = 25 * time.Second

// parseEventID parses "<streamID>_<index>" (the SDK's id format).
func parseEventID(id string) (stream string, idx int, ok bool) {
	parts := strings.Split(id, "_")
	if len(parts) != 2 {
		return "", 0, false
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil || n < 0 {
		return "", 0, false
	}
	return parts[0], n, true
}

// isFinalResponse reports whether an SSE payload is the JSON-RPC response that
// ends a request stream.
func isFinalResponse(data []byte) bool {
	var m struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if json.Unmarshal(data, &m) != nil {
		return false
	}
	return m.Method == "" && len(m.ID) > 0 && (len(m.Result) > 0 || len(m.Error) > 0)
}

func (s *sessionHost) serveResume(w http.ResponseWriter, r *http.Request, ls *localSession, lastEventID string) {
	streamID, idx, ok := parseEventID(lastEventID)
	if !ok {
		http.Error(w, fmt.Sprintf("malformed Last-Event-ID %q", lastEventID), http.StatusBadRequest)
		return
	}
	if err := s.resum.Check(r.Context(), ls.secret, streamID, idx); err != nil {
		if s.srec != nil {
			s.srec.Resume("gap")
		}
		// 404 tells the client to start a new session: the buffer no longer
		// holds the events it missed (bounded by design).
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	// A request stream whose response the client already holds is complete:
	// answer with an empty stream instead of waiting for events that never come.
	if streamID != "" && s.streamComplete(r.Context(), ls.secret, streamID, idx) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		return
	}
	rc := http.NewResponseController(w)
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	var wmu sync.Mutex
	write := func(format string, a ...any) error {
		wmu.Lock()
		defer wmu.Unlock()
		if _, err := fmt.Fprintf(w, format, a...); err != nil {
			return err
		}
		return rc.Flush()
	}
	if err := write(": ok\n\n"); err != nil {
		return
	}
	if s.srec != nil {
		s.srec.Resume("ok")
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() { // a closed session ends every follower
		select {
		case <-ls.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	hb := time.NewTicker(sseHeartbeat)
	defer hb.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-hb.C:
				if write(": ping\n\n") != nil {
					cancel()
					return
				}
			}
		}
	}()

	err := s.resum.Follow(ctx, ls.secret, streamID, idx, func(i int, data []byte) (bool, error) {
		if len(data) == 0 {
			return false, nil
		}
		if err := write("id: %s_%d\nevent: message\ndata: %s\n\n", streamID, i, data); err != nil {
			return true, err
		}
		return streamID != "" && isFinalResponse(data), nil
	})
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, ErrResumeGap) {
		s.log.WarnContext(r.Context(), "mcp resume stream ended with error", "error", err)
	}
}

// streamComplete: the event the client last saw (ordinal idx) is the final response.
func (s *sessionHost) streamComplete(ctx context.Context, secret, streamID string, idx int) bool {
	for data, err := range s.resum.After(ctx, secret, streamID, idx-1) {
		return err == nil && isFinalResponse(data)
	}
	return false
}
