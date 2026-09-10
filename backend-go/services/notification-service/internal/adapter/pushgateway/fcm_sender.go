package pushgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2/google"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

const fcmProductionBaseURL = "https://fcm.googleapis.com"

// FCMSender sends push messages via FCM's HTTP v1 API for
// subscription.Channel == android. The credential blob PushCredentialResolver
// returns for owner_id "fcm" is the FCM service-account JSON as-is
// (project_id, private_key, client_email, token_uri, ...) —
// google.CredentialsFromJSON parses it directly, no custom struct needed
// for the OAuth2 fields (unlike apnsCredential, which does need one — APNs
// has no equivalent standard JSON shape to parse against).
type FCMSender struct {
	credentials usecase.PushCredentialResolver
	httpClient  *http.Client
	// fcmBaseURL overrides fcmProductionBaseURL — test-only, mirrors
	// APNsSender.apnsBaseURL's reasoning.
	fcmBaseURL string
}

func NewFCMSender(credentials usecase.PushCredentialResolver, httpClient *http.Client) *FCMSender {
	return &FCMSender{credentials: credentials, httpClient: httpClient, fcmBaseURL: fcmProductionBaseURL}
}

var _ usecase.PushSender = (*FCMSender)(nil)

func (s *FCMSender) Send(ctx context.Context, sub domain.PushSubscription, event domain.NotificationEvent) error {
	raw, err := s.credentials.Resolve(ctx, sub.TenantID, "fcm")
	if err != nil {
		return fmt.Errorf("pushgateway: resolving fcm credential: %w", err)
	}

	creds, err := google.CredentialsFromJSON(ctx, raw, "https://www.googleapis.com/auth/firebase.messaging")
	if err != nil {
		return fmt.Errorf("pushgateway: parsing fcm service account: %w", err)
	}
	token, err := creds.TokenSource.Token()
	if err != nil {
		return fmt.Errorf("pushgateway: fetching fcm oauth2 token: %w", err)
	}

	var projectID struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(raw, &projectID); err != nil {
		return fmt.Errorf("pushgateway: reading fcm project_id: %w", err)
	}

	payload, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"token":        sub.Endpoint,
			"notification": map[string]string{"title": event.Title, "body": event.Body},
		},
	})
	if err != nil {
		return fmt.Errorf("pushgateway: encoding fcm payload: %w", err)
	}

	url := fmt.Sprintf("%s/v1/projects/%s/messages:send", s.fcmBaseURL, projectID.ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("pushgateway: building fcm request: %w", err)
	}
	req.Header.Set("authorization", "Bearer "+token.AccessToken)
	req.Header.Set("content-type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pushgateway: fcm request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound { // FCM UNREGISTERED
		return usecase.ErrDeviceTokenInvalid
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pushgateway: fcm returned %d: %s", resp.StatusCode, body)
	}
	return nil
}
