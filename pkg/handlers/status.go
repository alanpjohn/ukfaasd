package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/alanpjohn/ukfaas/pkg/api"
	"github.com/gorilla/mux"
)

func MakeFunctionStatusHandler(manager api.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		service := vars["name"]

		// validate namespace

		res, err := manager.Get(service)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
		} else {
			functionBytes, _ := json.Marshal(res)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(functionBytes)
		}

	}
}
