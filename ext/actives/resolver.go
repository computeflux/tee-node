package actives

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// Resolver implements the generated ResolverRoot interface.
type Resolver struct{}

func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }
func (r *Resolver) Query() QueryResolver       { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }

// touchClientIp 记录请求来源 IP：用户刷新页面/活跃时保持其 IP 记录新鲜。
func (r *Resolver) touchClientIp(ctx context.Context, wallet string) {
	Store.RecordClientIp(wallet, ClientIPFromContext(ctx))
}

// ── Query ────────────────────────────────────────────────

func (r *queryResolver) EarlyAccessUser(ctx context.Context, user string) (*EarlyAccessUser, error) {
	r.touchClientIp(ctx, user)
	u := Store.GetUser(user)
	if u == nil {
		return &EarlyAccessUser{IsUser: false}, nil
	}
	return &EarlyAccessUser{
		IsUser: true, Wallet: strPtr(u.Wallet), JoinedAt: strPtr(u.JoinedAt),
		Referrer: strPtr(u.Referrer), ReferralCode: strPtr(u.ReferralCode), Email: strPtr(u.Email),
	}, nil
}

func (r *queryResolver) EarlyAccessUserByEmail(ctx context.Context, email string) (*EarlyAccessUser, error) {
	u := Store.GetUserByEmail(email)
	if u == nil {
		return &EarlyAccessUser{IsUser: false}, nil
	}
	return &EarlyAccessUser{IsUser: true, Wallet: strPtr(u.Wallet), Email: strPtr(u.Email)}, nil
}

func (r *queryResolver) EarlyAccessUserByReferralCode(ctx context.Context, referralCode string) (*EarlyAccessUser, error) {
	u := Store.GetUserByReferralCode(referralCode)
	if u == nil {
		return &EarlyAccessUser{IsUser: false}, nil
	}
	return &EarlyAccessUser{IsUser: true, Wallet: strPtr(u.Wallet), ReferralCode: strPtr(u.ReferralCode)}, nil
}

func (r *queryResolver) EarlyAccessPoints(ctx context.Context, user string) (*EarlyAccessPoints, error) {
	r.touchClientIp(ctx, user)
	return &EarlyAccessPoints{Total: Store.GetPoints(user)}, nil
}

func (r *queryResolver) EarlyAccessReferral(ctx context.Context, user string) (*ReferralInfo, error) {
	r.touchClientIp(ctx, user)
	code, alias, referrer, earnings, pending, referrals := Store.GetReferralInfo(user)
	res := &ReferralInfo{
		Earnings:  fmt.Sprintf("%d", earnings),
		Pending:   pending,
		Referrals: referrals,
	}
	if code != "" {
		res.Code = &code
	}
	if alias != "" {
		res.Alias = &alias
	}
	if referrer != "" {
		res.Referrer = &referrer
	}
	return res, nil
}

func (r *queryResolver) EarlyAccessPointsHistory(ctx context.Context, user string, offset *int, limit *int) (*PointsHistory, error) {
	r.touchClientIp(ctx, user)
	o, l := 0, 20
	if offset != nil {
		o = *offset
	}
	if limit != nil && *limit > 0 {
		l = *limit
	}
	items, total := Store.GetPointsHistory(user, o, l)
	out := make([]*PointsHistoryItem, len(items))
	for i, it := range items {
		out[i] = &PointsHistoryItem{
			Amount: it.Amount, Reason: it.Reason, Timestamp: it.Timestamp,
			RelatedUser: it.RelatedUser, RelatedEmail: it.RelatedEmail,
		}
	}
	return &PointsHistory{Items: out, Total: total}, nil
}

func (r *queryResolver) EarlyAccessStats(ctx context.Context) (*EarlyAccessStats, error) {
	s := Store.GetStats()
	return &EarlyAccessStats{
		UserCount: s.UserCount, TotalPointsDistributed: fmt.Sprintf("%d", s.TotalPointsDistributed),
		TotalReferrals: s.TotalReferrals,
	}, nil
}

func (r *queryResolver) EarlyAccessTasks(ctx context.Context, wallet string) ([]*TaskStatus, error) {
	r.touchClientIp(ctx, wallet)
	tasks := Store.GetTasks(wallet)
	out := make([]*TaskStatus, len(tasks))
	for i, t := range tasks {
		out[i] = &TaskStatus{
			Task: EarlyAccessTaskType(t.Task), Completed: t.Completed, ClaimedAt: t.ClaimedAt,
		}
	}
	return out, nil
}

func (r *queryResolver) EarlyAccessExport(ctx context.Context) (string, error) {
	return Store.Export(), nil
}

func (r *queryResolver) AntiSybilConfig(ctx context.Context) (*AntiSybilConfig, error) {
	cfg := Store.GetAntiSybilConfig()
	res := &AntiSybilConfig{
		IPCheckEnabled:            cfg.IpCheckEnabled,
		DeferredSettlementEnabled: cfg.DeferredSettlementEnabled,
	}
	if cfg.UpdatedBy != "" {
		res.UpdatedBy = &cfg.UpdatedBy
	}
	if cfg.UpdatedAt != "" {
		res.UpdatedAt = &cfg.UpdatedAt
	}
	return res, nil
}

func (r *queryResolver) ReferralBonusConfigs(ctx context.Context) ([]*ReferralBonus, error) {
	items := Store.GetReferralBonusConfigs()
	out := make([]*ReferralBonus, len(items))
	for i, it := range items {
		if it.BonusPoints > math.MaxInt32 {
			return nil, fmt.Errorf("bonus points %d out of range", it.BonusPoints)
		}
		b := &ReferralBonus{Code: it.Code, BonusPoints: int(it.BonusPoints)}
		if it.UpdatedBy != "" {
			b.UpdatedBy = &it.UpdatedBy
		}
		if it.UpdatedAt != "" {
			b.UpdatedAt = &it.UpdatedAt
		}
		out[i] = b
	}
	return out, nil
}

// ReferralEligibility 加入前预检查：当前 IP 用该邀请码加入是否可获得推荐奖励。
// wallet 可空：传入时额外检查已加入/自邀，为空时仅做同 IP 判断。
func (r *queryResolver) ReferralEligibility(ctx context.Context, wallet *string, referralCode string) (*ReferralEligibilityResult, error) {
	ip := ClientIPFromContext(ctx)
	w := ""
	if wallet != nil {
		w = *wallet
	}
	eligible, reason := Store.CheckReferralEligibility(w, ip, referralCode)
	res := &ReferralEligibilityResult{Eligible: eligible}
	if reason != "" {
		res.Reason = &reason
	}
	return res, nil
}

func (r *queryResolver) CheckinConfig(ctx context.Context) (*CheckinConfig, error) {
	return &CheckinConfig{
		TestMode: CheckinTestMode,
	}, nil
}

func (r *queryResolver) CheckinStreak(ctx context.Context, wallet string) (int, error) {
	r.touchClientIp(ctx, wallet)
	return Store.GetStreak(wallet), nil
}

func (r *queryResolver) CheckinStatus(ctx context.Context, wallet string) (*CheckinStatus, error) {
	r.touchClientIp(ctx, wallet)
	checkedIn, canCheckin, weekCount, weekMax, streak := Store.GetCheckinStatus(wallet)
	return &CheckinStatus{
		CheckedIn:  checkedIn,
		CanCheckin: canCheckin,
		WeekCount:  weekCount,
		WeekMax:    weekMax,
		Streak:     streak,
	}, nil
}

func (r *queryResolver) QuizQuestions(ctx context.Context, wallet string, day int) ([]*QuizQuestion, error) {
	r.touchClientIp(ctx, wallet)
	qs := Store.GetQuestionsForDay(wallet, day)
	out := make([]*QuizQuestion, len(qs))
	for i, q := range qs {
		out[i] = &QuizQuestion{
			ID: q.ID, Day: q.Day, Difficulty: q.Difficulty, Category: q.Category,
			Question: q.Question, Options: q.Options,
		}
	}
	return out, nil
}

func (r *queryResolver) QuizProgress(ctx context.Context, wallet string, day int) (*QuizProgress, error) {
	r.touchClientIp(ctx, wallet)
	total, answered, earned := Store.GetQuizProgress(wallet, day)
	return &QuizProgress{Total: total, Answered: answered, Earned: fmt.Sprintf("%d", earned)}, nil
}

// ── Mutation ─────────────────────────────────────────────

func (r *mutationResolver) JoinEarlyAccess(ctx context.Context, wallet string, email *string, referralCode *string) (*JoinEarlyAccessResult, error) {
	e := ""
	if email != nil {
		e = *email
	}
	ip := ClientIPFromContext(ctx)
	ok, warning, awarded, err := Store.Join(wallet, e, ip, referralCode)
	if err != nil {
		return &JoinEarlyAccessResult{Success: false, TxHash: strPtr(err.Error()), ReferralAwarded: false}, nil
	}
	res := &JoinEarlyAccessResult{Success: ok, ReferralAwarded: awarded}
	if warning != "" {
		res.Warning = &warning
	}
	return res, nil
}

func (r *mutationResolver) UpdateReferralCode(ctx context.Context, wallet string, referralCode string) (*JoinEarlyAccessResult, error) {
	ip := ClientIPFromContext(ctx)
	ok, warning, awarded, err := Store.UpdateReferralCode(wallet, ip, referralCode)
	if err != nil {
		return &JoinEarlyAccessResult{Success: false, TxHash: strPtr(err.Error()), ReferralAwarded: false}, nil
	}
	res := &JoinEarlyAccessResult{Success: ok, ReferralAwarded: awarded}
	if warning != "" {
		res.Warning = &warning
	}
	return res, nil
}

func (r *mutationResolver) UpdateReferralAlias(ctx context.Context, wallet string, alias string, signature string) (*JoinEarlyAccessResult, error) {
	r.touchClientIp(ctx, wallet)
	// 签名验证：只有钱包所有者（对 alias 内容的 EIP-191 签名）才能修改自己的别名
	if !verifyEIP191Signature(wallet, signature, alias) {
		return &JoinEarlyAccessResult{Success: false, TxHash: strPtr("invalid signature"), ReferralAwarded: false}, nil
	}
	ok, _, err := Store.UpdateReferralAlias(wallet, alias)
	if err != nil {
		return &JoinEarlyAccessResult{Success: false, TxHash: strPtr(err.Error()), ReferralAwarded: false}, nil
	}
	return &JoinEarlyAccessResult{Success: ok, ReferralAwarded: false}, nil
}

func (r *mutationResolver) UpdateAntiSybilConfig(ctx context.Context, wallet string, password string, ipCheckEnabled bool, deferredSettlementEnabled bool) (*AntiSybilConfigResult, error) {
	ip := ClientIPFromContext(ctx)
	if adminLocked(ip) {
		return &AntiSybilConfigResult{Success: false, Message: "too many failed attempts, try again later"}, nil
	}
	cfg, msg := Store.UpdateAntiSybilConfig(wallet, password, ipCheckEnabled, deferredSettlementEnabled)
	if msg != "" {
		if isAdminAuthError(msg) {
			recordAdminFailure(ip)
		}
		return &AntiSybilConfigResult{Success: false, Message: msg}, nil
	}
	clearAdminFailures(ip)
	_ = cfg
	return &AntiSybilConfigResult{Success: true, Message: "ok"}, nil
}

func (r *mutationResolver) SetReferralBonus(ctx context.Context, wallet string, password string, code string, bonusPoints int) (*ReferralBonusResult, error) {
	ip := ClientIPFromContext(ctx)
	if adminLocked(ip) {
		return &ReferralBonusResult{Success: false, Message: "too many failed attempts, try again later"}, nil
	}
	_, msg := Store.SetReferralBonus(wallet, password, code, int64(bonusPoints))
	if msg != "" {
		if isAdminAuthError(msg) {
			recordAdminFailure(ip)
		}
		return &ReferralBonusResult{Success: false, Message: msg}, nil
	}
	clearAdminFailures(ip)
	return &ReferralBonusResult{Success: true, Message: "ok"}, nil
}

func (r *mutationResolver) ClaimEarlyAccessTask(ctx context.Context, wallet string, task EarlyAccessTaskType) (*ClaimTaskResult, error) {
	r.touchClientIp(ctx, wallet)
	ok, awarded, total, err := Store.ClaimTask(wallet, string(task))
	if err != nil {
		return &ClaimTaskResult{Success: false}, nil
	}
	return &ClaimTaskResult{Success: ok, PointsAwarded: fmt.Sprintf("%d", awarded), TotalPoints: fmt.Sprintf("%d", total)}, nil
}

func (r *mutationResolver) JoinDiscord(ctx context.Context, wallet string, message string, signature string) (*ClaimTaskResult, error) {
	r.touchClientIp(ctx, wallet)
	ok, awarded, total, err := Store.ClaimTask(wallet, "JOIN_DISCORD")
	if err != nil {
		return &ClaimTaskResult{Success: false}, nil
	}
	return &ClaimTaskResult{Success: ok, PointsAwarded: fmt.Sprintf("%d", awarded), TotalPoints: fmt.Sprintf("%d", total)}, nil
}

func (r *mutationResolver) JoinTelegram(ctx context.Context, wallet string, message string, signature string) (*ClaimTaskResult, error) {
	r.touchClientIp(ctx, wallet)
	ok, awarded, total, err := Store.ClaimTask(wallet, "JOIN_TELEGRAM")
	if err != nil {
		return &ClaimTaskResult{Success: false}, nil
	}
	return &ClaimTaskResult{Success: ok, PointsAwarded: fmt.Sprintf("%d", awarded), TotalPoints: fmt.Sprintf("%d", total)}, nil
}

func (r *mutationResolver) DailyCheckin(ctx context.Context, wallet string, date *string) (*CheckinResult, error) {
	r.touchClientIp(ctx, wallet)
	dateStr := ""
	if date != nil {
		dateStr = *date
	}
	streak, total, unlocked3, unlocked7, err := Store.DailyCheckin(wallet, dateStr)
	if err != nil {
		return &CheckinResult{Success: false}, nil
	}
	return &CheckinResult{
		Success: true, Streak: streak, PointsAwarded: fmt.Sprintf("%d", checkinPoints),
		TotalPoints: fmt.Sprintf("%d", total), UnlockedDay3: unlocked3, UnlockedDay7: unlocked7,
	}, nil
}

func (r *mutationResolver) SubmitQuizAnswer(ctx context.Context, wallet string, questionID string, answerIndex int) (*QuizAnswerResult, error) {
	r.touchClientIp(ctx, wallet)
	correct, pts, explanation, err := Store.SubmitQuizAnswer(wallet, questionID, answerIndex)
	if err != nil {
		return &QuizAnswerResult{Correct: false, Explanation: err.Error()}, nil
	}
	total := Store.GetPoints(wallet)
	return &QuizAnswerResult{
		Correct: correct, PointsAwarded: fmt.Sprintf("%d", pts),
		TotalPoints: total, Explanation: explanation,
	}, nil
}

func strPtr(s string) *string { return &s }

// ── 管理操作防爆破：按来源 IP 简单限流 ──────────────────────
// 管理密码来自配置文件（无独立账号体系），这是除常量时间比较外的第一道防线：
// 连续密码验证失败 maxAdminFailAttempts 次后，锁定该 IP lockoutDuration。
// 仅内存态，重启清零；仅对密码类错误计数，不误伤其他参数错误。

const (
	maxAdminFailAttempts = 5
	adminLockoutDuration = 10 * time.Minute
)

type adminFailState struct {
	count int
	until time.Time
}

var (
	adminFailMu   sync.Mutex
	adminFailures = make(map[string]adminFailState)
)

// isAdminAuthError 判断 Store 返回的错误是否为密码验证类错误。
func isAdminAuthError(msg string) bool {
	return msg == "invalid password" || msg == "admin password not configured"
}

// adminLocked 检查该 IP 是否处于锁定状态。
func adminLocked(ip string) bool {
	adminFailMu.Lock()
	defer adminFailMu.Unlock()
	st, ok := adminFailures[ip]
	if !ok {
		return false
	}
	if time.Now().Before(st.until) {
		return true
	}
	delete(adminFailures, ip) // 锁定期已过，清理
	return false
}

// recordAdminFailure 记录一次密码验证失败。
func recordAdminFailure(ip string) {
	adminFailMu.Lock()
	defer adminFailMu.Unlock()
	st := adminFailures[ip]
	st.count++
	if st.count >= maxAdminFailAttempts {
		st.until = time.Now().Add(adminLockoutDuration)
		st.count = 0
		fmt.Printf("[admin] anti-brute-force: ip=%s locked for %s\n", ip, adminLockoutDuration)
	}
	adminFailures[ip] = st
}

// clearAdminFailures 成功后清零该 IP 的失败计数。
func clearAdminFailures(ip string) {
	adminFailMu.Lock()
	defer adminFailMu.Unlock()
	delete(adminFailures, ip)
}
