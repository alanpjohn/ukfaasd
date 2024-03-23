package handlers

import (
	"net/http"
)

func MakeSecretHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {}
}
