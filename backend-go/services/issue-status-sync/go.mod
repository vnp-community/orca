module github.com/stablyai/orca-go/services/issue-status-sync

go 1.25.0

require (
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.10.0
	github.com/stablyai/orca-go/common v0.0.0
	github.com/stablyai/orca-go/proto v0.0.0
	google.golang.org/grpc v1.83.0
)

replace github.com/stablyai/orca-go/common => ../../common

replace github.com/stablyai/orca-go/proto => ../../proto
