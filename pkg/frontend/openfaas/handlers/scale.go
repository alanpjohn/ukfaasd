package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/alanpjohn/ukfaas/pkg/api"
	"github.com/openfaas/faas-provider/types"
)

func MakeScaleHandler(manager api.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			http.Error(w, "expected a body", http.StatusBadRequest)
			return
		}

		defer r.Body.Close()

		body, _ := io.ReadAll(r.Body)
		log.Printf("[Scale] request: %s\n", string(body))

		req := types.ScaleServiceRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			log.Printf("[Scale] error parsing input: %s\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		// validate namespace

		err := manager.Scale(req)
		if err != nil {
			msg := fmt.Sprintf("Function scale failed: %v", err)
			log.Printf("[Scale] %s\n", msg)
			http.Error(w, msg, http.StatusInternalServerError)
		}
	}
}
