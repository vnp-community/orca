package domain

// AdminStats is the admin-console dashboard's summary counts — see
// GetAdminStatsResponse in proto/orca/auth/v1/auth.proto.
type AdminStats struct {
	TotalUsers     int32
	ActiveSessions int32
	TotalPolicies  int32
}
