package graph

import (
	"encoding/json"
	"net/http"
)

// RegisterRESTRoutes mounts REST endpoints that the dapp's Web3Auth SDK needs.
func RegisterRESTRoutes(mux *http.ServeMux, r *Resolver) {
	mux.HandleFunc("/api/blockchain/token", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"success": false, "message": "method not allowed"})
			return
		}
		var body struct {
			Address   string `json:"address"`
			Signature string `json:"signature"`
			Timestamp int64  `json:"timestamp"`
			OrderID   int    `json:"order_id"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "invalid body"})
			return
		}

		// TODO: verify signature, check order on-chain, generate API key
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false, "message": "not yet implemented",
		})
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
