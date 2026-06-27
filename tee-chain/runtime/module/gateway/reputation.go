package gateway

import (
	"fmt"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// ── User Report ───────────────────────────────────────────

// ReportModel 用户举报模型造假，触发系统检查。
// reason 不得为空且长度受限（最大 500 字符）。
func (g GatewayMutation) ReportModel(modelID uint64, reason string) (uint64, error) {
	if len(reason) == 0 || len(reason) > 500 {
		return 0, fmt.Errorf("reason must be 1-500 characters")
	}

	caller := g.api.GetCaller()
	txn := g.api.GetTxn()

	// 校验模型存在
	model, err := g.models.Get(txn, modelID)
	if err != nil || model == nil {
		return 0, fmt.Errorf("model %d not found", modelID)
	}

	id, err := g.modelReports.Insert(txn, ModelReport{
		ModelID:   modelID,
		Reporter:  caller,
		Reason:    reason,
		Resolved:  false,
		CreatedAt: uint64(g.api.GetBlockTime()),
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ResolveReport 处理举报并联动更新模型信誉（仅系统可调用）。
// valid=true 表示举报不实（模型通过检查），valid=false 表示举报属实（模型造假）。
func (g GatewayMutation) ResolveReport(reportID uint64, valid bool, detail string) error {
	caller := g.api.GetCaller()
	if caller.T != 0 {
		return fmt.Errorf("ResolveReport: must be called by system")
	}

	txn := g.api.GetTxn()

	report, err := g.modelReports.Get(txn, reportID)
	if err != nil || report == nil {
		return fmt.Errorf("report %d not found", reportID)
	}
	if report.Resolved {
		return fmt.Errorf("report %d already resolved", reportID)
	}

	// 标记举报已处理
	if err := g.modelReports.UpdateFn(txn, reportID, func(r *ModelReport) {
		r.Resolved = true
	}); err != nil {
		return err
	}

	// 联动信誉：举报属实 = 模型未通过检查（valid=false → passed=false, model fails）
	passed := valid
	_, err = g.AuditModel(report.ModelID, AuditTriggerReport, passed, detail)
	return err
}

// ── Audit ─────────────────────────────────────────────────

// AuditModel 系统对模型能力进行检查，更新模型信誉分（仅系统可调用）。
// triggerBy: AuditTriggerReport(1) 举报触发 / AuditTriggerRandom(2) 随机抽检。
func (g GatewayMutation) AuditModel(modelID uint64, triggerBy uint8, passed bool, detail string) (uint64, error) {
	caller := g.api.GetCaller()
	if caller.T != 0 {
		return 0, fmt.Errorf("AuditModel: must be called by system")
	}

	txn := g.api.GetTxn()

	// 校验模型存在，避免产生孤儿审计记录
	if m, err := g.models.Get(txn, modelID); err != nil || m == nil {
		return 0, fmt.Errorf("AuditModel: model %d not found", modelID)
	}

	// 记录检查
	auditID, err := g.modelAudits.Insert(txn, ModelAudit{
		ModelID:   modelID,
		TriggerBy: triggerBy,
		Passed:    passed,
		Detail:    detail,
		CreatedAt: uint64(g.api.GetBlockTime()),
	})
	if err != nil {
		return 0, err
	}

	// 更新模型信誉分（EMA，最近 100 次权重主导）
	if err := g.models.UpdateFn(txn, modelID, func(m *ModelInfo) {
		m.TotalAudits++
		if passed {
			m.PassedAudits++
		}

		// EMA: score = oldScore * (n-1)/n + result * 1/n, n=min(totalAudits, 100)
		const window = uint64(100)
		n := m.TotalAudits
		if n > window {
			n = window
		}
		oldScore := uint64(m.ReputationScore)
		var result uint64
		if passed {
			result = 10000
		}
		m.ReputationScore = uint32((oldScore*(n-1) + result) / n)
	}); err != nil {
		return auditID, err
	}

	return auditID, nil
}

// ── Query ─────────────────────────────────────────────────

// GetModelReports 查询模型已处理的举报（公开可见，隐藏未处理举报）。
func (g GatewayQuery) GetModelReports(modelID uint64, offset uint64, limit uint32) ([]ModelReport, error) {
	ids, err := g.modelReports.Query(g.api.GetTxn(), store.QueryAND{
		store.QueryCond{Field: "ModelID", Val: modelID},
		store.QueryCond{Field: "Resolved", Val: uint64(1)},
	})
	if err != nil {
		return nil, err
	}
	if uint64(len(ids)) <= offset {
		return nil, nil
	}
	end := offset + uint64(limit)
	if end > uint64(len(ids)) {
		end = uint64(len(ids))
	}
	txn := g.api.GetTxn()
	out := make([]ModelReport, 0, end-offset)
	for _, id := range ids[offset:end] {
		r, err := g.modelReports.Get(txn, id)
		if err != nil || r == nil {
			continue
		}
		out = append(out, *r)
	}
	return out, nil
}

// GetModelAudits 查询模型检查记录（公开可见）。
func (g GatewayQuery) GetModelAudits(modelID uint64, offset uint64, limit uint32) ([]ModelAudit, error) {
	ids, err := g.modelAudits.Query(g.api.GetTxn(), store.QueryCond{Field: "ModelID", Val: modelID})
	if err != nil {
		return nil, err
	}
	if uint64(len(ids)) <= offset {
		return nil, nil
	}
	end := offset + uint64(limit)
	if end > uint64(len(ids)) {
		end = uint64(len(ids))
	}
	txn := g.api.GetTxn()
	out := make([]ModelAudit, 0, end-offset)
	for _, id := range ids[offset:end] {
		a, err := g.modelAudits.Get(txn, id)
		if err != nil || a == nil {
			continue
		}
		out = append(out, *a)
	}
	return out, nil
}

// ── GatewaySecret ──────────────────────────────────────────
// 系统内部使用，可查看全部举报（含未处理）。

func (g GatewaySecret) GetModelReportsAll(modelID uint64, offset uint64, limit uint32) ([]ModelReport, error) {
	ids, err := g.modelReports.Query(g.api.GetTxn(), store.QueryCond{Field: "ModelID", Val: modelID})
	if err != nil {
		return nil, err
	}
	if uint64(len(ids)) <= offset {
		return nil, nil
	}
	end := offset + uint64(limit)
	if end > uint64(len(ids)) {
		end = uint64(len(ids))
	}
	txn := g.api.GetTxn()
	out := make([]ModelReport, 0, end-offset)
	for _, id := range ids[offset:end] {
		r, err := g.modelReports.Get(txn, id)
		if err != nil || r == nil {
			continue
		}
		out = append(out, *r)
	}
	return out, nil
}
