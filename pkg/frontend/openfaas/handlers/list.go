package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/alanpjohn/ukfaas/pkg/api"
)

func MakeListHandler(manager api.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// validate namespace

		res, err := manager.List()
		if err != nil {
			log.Printf("[List] error listing functions. Error: %s\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}
}
