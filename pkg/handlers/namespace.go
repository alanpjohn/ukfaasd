package handlers

import (
	"net/http"
)

func MakeNamespaceListerHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {}
}

func MakeNamespaceMutateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {}
}
