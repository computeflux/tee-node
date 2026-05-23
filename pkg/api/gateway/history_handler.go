package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
)

// ── Response helpers ─────────────────────────────────────

type historyResponse struct {
	Data  any    `json:"data"`
	Total uint64 `json:"total,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

// ── GET /v1/logs ─────────────────────────────────────────
// 查询用户的调用日志（复用合约用量数据，确保所有节点可见）

func HandleLogs(w http.ResponseWriter, r *http.Request) {
	token := GetTokenFromContext(r)
	if token == nil {
		writeError(w, http.StatusUnauthorized, "missing api key", "auth_error")
		return
	}

	q := r.URL.Query()
	providerID, _ := strconv.ParseUint(q.Get("provider_id"), 10, 64)
	modelID, _ := strconv.ParseUint(q.Get("model_id"), 10, 64)
	startDay := q.Get("start_day")
	endDay := q.Get("end_day")

	if startDay == "" {
		startDay = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}
	if endDay == "" {
		endDay = time.Now().Format("2006-01-02")
	}

	userAddr := model.UniAddr{V: []byte(token.Owner)}

	bridge := getBridge()
	if bridge == nil {
		writeJSON(w, http.StatusOK, historyResponse{Data: []any{}})
		return
	}

	records, err := bridge.GatewayGetUsageHistory(userAddr, providerID, modelID, startDay, endDay)
	if err != nil {
		GATEWAYLOG().Printf("[HISTORY] logs (usage) err: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to query logs", "server_error")
		return
	}
	writeJSON(w, http.StatusOK, historyResponse{Data: records})
}

// ── POST /v1/data/export ─────────────────────────────────
// 数据导出：基于合约数据组装导出内容，确保所有节点一致

type exportRequest struct {
	Type       string `json:"type"` // usage | recharge | settlement | logs
	StartDay   string `json:"start_day"`
	EndDay     string `json:"end_day"`
	Format     string `json:"format"` // json | csv
	ProviderID uint64 `json:"provider_id,omitempty"`
}

func HandleDataExport(w http.ResponseWriter, r *http.Request) {
	token := GetTokenFromContext(r)
	if token == nil {
		writeError(w, http.StatusUnauthorized, "missing api key", "auth_error")
		return
	}

	var req exportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "invalid_request")
		return
	}
	if req.Type == "" {
		writeError(w, http.StatusBadRequest, "missing export type", "invalid_request")
		return
	}
	if req.Format == "" {
		req.Format = "json"
	}
	if req.StartDay == "" {
		req.StartDay = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if req.EndDay == "" {
		req.EndDay = time.Now().Format("2006-01-02")
	}

	userAddr := model.UniAddr{V: []byte(token.Owner)}
	bridge := getBridge()
	if bridge == nil {
		writeError(w, http.StatusServiceUnavailable, "chain not ready", "server_error")
		return
	}

	var data any
	switch req.Type {
	case "usage", "logs":
		records, err := bridge.GatewayGetUsageHistory(userAddr, 0, 0, req.StartDay, req.EndDay)
		if err != nil {
			GATEWAYLOG().Printf("[EXPORT] usage err: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to query usage data", "server_error")
			return
		}
		data = records
	case "recharge":
		records, err := bridge.GatewayGetRechargeHistory(userAddr, 0, 1000)
		if err != nil {
			GATEWAYLOG().Printf("[EXPORT] recharge err: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to query recharge data", "server_error")
			return
		}
		data = records
	case "settlement":
		// settlement 是 provider 视角，普通用户一般无权限查看；若用户是 provider owner 可传 provider_id
		if req.ProviderID == 0 {
			writeError(w, http.StatusBadRequest, "provider_id required for settlement export", "invalid_request")
			return
		}
		records, err := bridge.GatewayGetSettlementHistory(req.ProviderID, 0, 1000)
		if err != nil {
			GATEWAYLOG().Printf("[EXPORT] settlement err: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to query settlement data", "server_error")
			return
		}
		data = records
	default:
		writeError(w, http.StatusBadRequest, "unknown export type", "invalid_request")
		return
	}

	// 同步返回导出内容（JSON 格式）
	writeJSON(w, http.StatusOK, map[string]any{
		"type":   req.Type,
		"format": req.Format,
		"data":   data,
	})
}
