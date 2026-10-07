package http

import "net/http"

func HandleRequestWebhook(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
