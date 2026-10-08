package eventbus

// requestBindings are the request-service subjects. The stream name must match
// request-service's EnsureStream("REQUEST", "orca.request.>"); a wrong name
// fails silently, so TestSubjects_RequestBindings pins it.
var requestBindings = []SubjectBinding{
	// Durable: losing an approval gate or a clarification ask while the service
	// is down leaves a Request waiting on someone who was never told.
	{StreamName: "REQUEST", Subject: "orca.request.approval.requested", Durable: "notification-service-request-approval-requested"},
	// Ephemeral: a missed "decided" only delays display, the state lives in the DB.
	{StreamName: "REQUEST", Subject: "orca.request.approval.decided"},
	{StreamName: "REQUEST", Subject: "orca.request.clarification.requested", Durable: "notification-service-request-clarification-requested"},
	{StreamName: "REQUEST", Subject: "orca.request.clarification.expired", Durable: "notification-service-request-clarification-expired"},
}

func withRequestBindings(core []SubjectBinding) []SubjectBinding {
	return append(core, requestBindings...)
}
