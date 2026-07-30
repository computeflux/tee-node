package actives

import (
	"context"
	"fmt"
)

// Resolver implements the generated ResolverRoot interface.
type Resolver struct{}

func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }
func (r *Resolver) Query() QueryResolver       { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }

// ── Query ────────────────────────────────────────────────

func (r *queryResolver) EarlyAccessUser(ctx context.Context, user string) (*EarlyAccessUser, error) {
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
	return &EarlyAccessPoints{Total: Store.GetPoints(user)}, nil
}

func (r *queryResolver) EarlyAccessReferral(ctx context.Context, user string) (*ReferralInfo, error) {
	code, referrer, earnings, referrals := Store.GetReferralInfo(user)
	res := &ReferralInfo{Earnings: fmt.Sprintf("%d", earnings), Referrals: referrals}
	if code != "" {
		res.Code = &code
	}
	if referrer != "" {
		res.Referrer = &referrer
	}
	return res, nil
}

func (r *queryResolver) EarlyAccessPointsHistory(ctx context.Context, user string, offset *int, limit *int) (*PointsHistory, error) {
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

func (r *queryResolver) CheckinConfig(ctx context.Context) (*CheckinConfig, error) {
	return &CheckinConfig{
		TestMode: CheckinTestMode,
	}, nil
}

func (r *queryResolver) CheckinStreak(ctx context.Context, wallet string) (int, error) {
	return Store.GetStreak(wallet), nil
}

func (r *queryResolver) CheckinStatus(ctx context.Context, wallet string) (*CheckinStatus, error) {
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
	total, answered, earned := Store.GetQuizProgress(wallet, day)
	return &QuizProgress{Total: total, Answered: answered, Earned: fmt.Sprintf("%d", earned)}, nil
}

// ── Mutation ─────────────────────────────────────────────

func (r *mutationResolver) JoinEarlyAccess(ctx context.Context, wallet string, email *string, referralCode *string) (*JoinEarlyAccessResult, error) {
	e := ""
	if email != nil {
		e = *email
	}
	ok, err := Store.Join(wallet, e, referralCode)
	if err != nil {
		return &JoinEarlyAccessResult{Success: ok, TxHash: strPtr(err.Error())}, nil
	}
	return &JoinEarlyAccessResult{Success: ok}, nil
}

func (r *mutationResolver) UpdateReferralCode(ctx context.Context, wallet string, referralCode string) (*JoinEarlyAccessResult, error) {
	ok, err := Store.UpdateReferralCode(wallet, referralCode)
	if err != nil {
		return &JoinEarlyAccessResult{Success: ok, TxHash: strPtr(err.Error())}, nil
	}
	return &JoinEarlyAccessResult{Success: ok}, nil
}

func (r *mutationResolver) ClaimEarlyAccessTask(ctx context.Context, wallet string, task EarlyAccessTaskType) (*ClaimTaskResult, error) {
	ok, awarded, total, err := Store.ClaimTask(wallet, string(task))
	if err != nil {
		return &ClaimTaskResult{Success: false}, nil
	}
	return &ClaimTaskResult{Success: ok, PointsAwarded: fmt.Sprintf("%d", awarded), TotalPoints: fmt.Sprintf("%d", total)}, nil
}

func (r *mutationResolver) JoinDiscord(ctx context.Context, wallet string, message string, signature string) (*ClaimTaskResult, error) {
	ok, awarded, total, err := Store.ClaimTask(wallet, "JOIN_DISCORD")
	if err != nil {
		return &ClaimTaskResult{Success: false}, nil
	}
	return &ClaimTaskResult{Success: ok, PointsAwarded: fmt.Sprintf("%d", awarded), TotalPoints: fmt.Sprintf("%d", total)}, nil
}

func (r *mutationResolver) JoinTelegram(ctx context.Context, wallet string, message string, signature string) (*ClaimTaskResult, error) {
	ok, awarded, total, err := Store.ClaimTask(wallet, "JOIN_TELEGRAM")
	if err != nil {
		return &ClaimTaskResult{Success: false}, nil
	}
	return &ClaimTaskResult{Success: ok, PointsAwarded: fmt.Sprintf("%d", awarded), TotalPoints: fmt.Sprintf("%d", total)}, nil
}

func (r *mutationResolver) DailyCheckin(ctx context.Context, wallet string, date *string) (*CheckinResult, error) {
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
