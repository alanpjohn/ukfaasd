package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/alanpjohn/ukfaas/pkg/api"
	"github.com/openfaas/faas-provider/types"
)

func MakeDeployHandler(manager api.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			http.Error(w, "expected a body", http.StatusBadRequest)
			return
		}

		defer r.Body.Close()

		body, _ := io.ReadAll(r.Body)
		log.Printf("[Deploy] request: %s\n", string(body))

		req := types.FunctionDeployment{}
		err := json.Unmarshal(body, &req)
		if err != nil {
			log.Printf("[Deploy] - error parsing input: %s\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// validate namespaces

		err = manager.Deploy(req)
		if err != nil {
			log.Printf("[Deploy] - error deploying function: %s\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
}
