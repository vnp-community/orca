package main

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/httpgateway"
)

// TestTraceEventHandler_PublishesPayloadToBroadcast is TASK-BE-FFT-011's
// unit-testable coverage of the actual new logic wired into
// cons.SubscribeEphemeral: each eventbus.Event's Payload must reach every
// subscriber of the TraceBroadcast it was constructed with.
func TestTraceEventHandler_PublishesPayloadToBroadcast(t *testing.T) {
	broadcast := httpgateway.NewTraceBroadcast()
	ch, unsubscribe := broadcast.Subscribe()
	defer unsubscribe()

	handler := traceEventHandler(broadcast)

	payload := []byte(`{"id":"1","flow":"svc:span","level":"ok","fields":{},"ts":1}`)
	if err := handler(context.Background(), eventbus.Event{Payload: payload}); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	select {
	case got := <-ch:
		if string(got) != string(payload) {
			t.Fatalf("subscriber got %q, want %q", got, payload)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber never received the published payload")
	}
}

// TestTraceEventHandler_NeverErrors guards the Handler contract used by
// SubscribeEphemeral: a nil return means "ack this message" — trace
// forwarding must never cause message redelivery/backoff regardless of
// payload content.
func TestTraceEventHandler_NeverErrors(t *testing.T) {
	broadcast := httpgateway.NewTraceBroadcast()
	handler := traceEventHandler(broadcast)

	if err := handler(context.Background(), eventbus.Event{Payload: nil}); err != nil {
		t.Fatalf("handler with nil payload returned error: %v", err)
	}
	if err := handler(context.Background(), eventbus.Event{Payload: []byte("not json")}); err != nil {
		t.Fatalf("handler with malformed payload returned error: %v", err)
	}
}
