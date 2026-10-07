package http

import "net/http"

func HandleOrcaResources(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func HandleSourceTools(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
