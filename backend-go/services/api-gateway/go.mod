module github.com/stablyai/orca-go/services/api-gateway

go 1.25.0

require github.com/stablyai/orca-go/common v0.0.0

require (
	github.com/coder/websocket v1.8.15
	github.com/go-chi/chi/v5 v5.3.1
	github.com/stablyai/orca-go/proto v0.0.0
	golang.org/x/time v0.15.0
	google.golang.org/grpc v1.83.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.45.0 // indirect
	go.opentelemetry.io/otel/metric v1.45.0 // indirect
	go.opentelemetry.io/otel/sdk v1.45.0 // indirect
	go.opentelemetry.io/otel/trace v1.45.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
)

replace github.com/stablyai/orca-go/common => ../../common

replace github.com/stablyai/orca-go/proto => ../../proto
