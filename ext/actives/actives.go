package actives

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"

	dbstore "github.com/computeflux/tee-provider/pkg/types"
)

const eaNamespace = "EARLY_POINTS"

var (
	Store           = &EarlyAccessStore{}
	CheckinTestMode bool
)

func init() {
	if v := os.Getenv("CHECKIN_TEST_MODE"); v == "true" || v == "1" {
		CheckinTestMode = true
	}
}

type EarlyAccessStore struct {
	mu sync.Mutex
}

// ── DB types ─────────────────────────────────────────────

type DbUser struct {
	Wallet       string `json:"wallet"`
	JoinedAt     string `json:"joinedAt"`
	Referrer     string `json:"referrer"`
	ReferralCode string `json:"referralCode"`
	Email        string `json:"email"`
}

type DbHistory struct {
	Amount      string  `json:"amount"`
	Reason      string  `json:"reason"`
	Timestamp   string  `json:"timestamp"`
	RelatedUser *string `json:"relatedUser,omitempty"`
}

type DbStats struct {
	UserCount              int   `json:"userCount"`
	TotalPointsDistributed int64 `json:"totalPointsDistributed"`
	TotalReferrals         int   `json:"totalReferrals"`
}

type UserRecord struct {
	Wallet, JoinedAt, Referrer, ReferralCode, Email string
}

type HistoryRecord struct {
	Amount, Reason, Timestamp string
	RelatedUser               *string
	RelatedEmail              *string
}

type StatsRecord struct {
	UserCount              int   `json:"userCount"`
	TotalPointsDistributed int64 `json:"totalPointsDistributed"`
	TotalReferrals         int   `json:"totalReferrals"`
}

// PointsHistoryResult wraps paginated history results.
type PointsHistoryResult struct {
	Items []*HistoryRecord `json:"items"`
	Total int              `json:"total"`
}

type TaskStatusInfo struct {
	Task      string
	Completed bool
	ClaimedAt *string
}

// ── Streak / Daily Checkin ───────────────────────────

type StreakInfo struct {
	Streak       int    `json:"streak"`
	LastCheckin  string `json:"lastCheckin"` // YYYY-MM-DD
	UnlockedDay3 bool   `json:"unlockedDay3"`
	UnlockedDay7 bool   `json:"unlockedDay7"`
}

const checkinPoints = 5
const day3QuizPoints = 50
const day7QuizPoints = 100

func todayKey() string {
	return time.Now().UTC().Format("2006-01-02")
}

func (s *EarlyAccessStore) loadStreak(wallet string) (*StreakInfo, error) {
	v, _ := dbstore.DBGetJson[StreakInfo](dbstore.SECRET_DB, eaNamespace, "streak:"+strings.ToLower(wallet))
	return v, nil
}

func (s *EarlyAccessStore) saveStreak(wallet string, si *StreakInfo) error {
	return dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "streak:"+strings.ToLower(wallet), si)
}

func (s *EarlyAccessStore) DailyCheckin(wallet, dateStr string) (int, int64, bool, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet = strings.ToLower(wallet)

	si, _ := s.loadStreak(wallet)
	if si == nil {
		si = &StreakInfo{}
	}

	if dateStr == "" {
		if CheckinTestMode && si.LastCheckin != "" {
			// Test mode: auto-advance date so each call simulates a new day
			lastDt, _ := time.Parse("2006-01-02", si.LastCheckin)
			dateStr = lastDt.AddDate(0, 0, 1).Format("2006-01-02")
		} else {
			dateStr = todayKey()
		}
	}
	fmt.Printf("[checkin] DailyCheckin wallet=%s date=%s testMode=%v\n", wallet, dateStr, CheckinTestMode)

	if !CheckinTestMode {
		// Validate date format (skip in test mode)
		dt, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return 0, 0, false, false, fmt.Errorf("invalid date format, use YYYY-MM-DD")
		}
		today := time.Now().UTC().Truncate(24 * time.Hour)
		if dt.After(today) {
			return 0, 0, false, false, fmt.Errorf("cannot check in for a future date")
		}

		if si.LastCheckin == dateStr {
			// Already checked in for this date, just return current state
			pts, _ := s.loadPoints(wallet)
			return si.Streak, pts, si.UnlockedDay3, si.UnlockedDay7, nil
		}
	}

	// Compute expected next date (day after last checkin)
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	lastDt, _ := time.Parse("2006-01-02", si.LastCheckin)
	expectedNext := lastDt.AddDate(0, 0, 1).Format("2006-01-02")

	if si.LastCheckin == "" || dateStr == expectedNext {
		si.Streak++
	} else if dateStr == yesterday {
		// Catching up on yesterday: reset streak counter but keep any earned quiz unlocks.
		si.Streak = 1
	} else {
		si.Streak = 1
	}
	si.LastCheckin = dateStr

	if si.Streak == 3 {
		si.UnlockedDay3 = true
	}
	if si.Streak == 7 {
		si.UnlockedDay7 = true
	}

	if err := s.saveStreak(wallet, si); err != nil {
		return 0, 0, false, false, err
	}

	s.addPoints(wallet, checkinPoints)
	now := time.Now().UTC().Format(time.RFC3339)
	s.appendHistory(wallet, fmt.Sprintf("%d", checkinPoints), "daily_checkin", now, nil)

	// Update stats
	stats, _ := s.loadStats()
	stats.TotalPointsDistributed += checkinPoints
	s.saveStats(stats)

	total, _ := s.loadPoints(wallet)
	return si.Streak, total, si.UnlockedDay3, si.UnlockedDay7, nil
}

func (s *EarlyAccessStore) GetStreak(wallet string) int {
	wallet = strings.ToLower(wallet)
	si, _ := s.loadStreak(wallet)
	if si == nil {
		fmt.Printf("[streak] GetStreak wallet=%s -> nil (return 0)\n", wallet)
		return 0
	}
	today := todayKey()
	fmt.Printf("[streak] GetStreak wallet=%s today=%s lastCheckin=%s streak=%d testMode=%v\n", wallet, today, si.LastCheckin, si.Streak, CheckinTestMode)
	if CheckinTestMode || si.LastCheckin == today {
		return si.Streak
	}
	return 0
}

// ── Quiz Questions ──────────────────────────────────────

type QuizQuestionRecord struct {
	ID            string   `json:"id"`
	Day           int      `json:"day"`
	Difficulty    string   `json:"difficulty"`
	Category      string   `json:"category"`
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	CorrectAnswer int      `json:"correctAnswer"`
	Explanation   string   `json:"explanation"`
}

// GetQuestionsForDay returns unanswered questions for a given unlock day (3 or 7).
func (s *EarlyAccessStore) GetQuestionsForDay(wallet string, day int) []*QuizQuestionRecord {
	wallet = strings.ToLower(wallet)
	fmt.Printf("[quiz] GetQuestionsForDay wallet=%s day=%d\n", wallet, day)

	si, _ := s.loadStreak(wallet)
	if si == nil {
		fmt.Printf("[quiz]  streak=nil, checking if user exists...\n")
		user := s.GetUser(wallet)
		if user == nil {
			fmt.Printf("[quiz]  user NOT found, returning nil\n")
			return nil
		}
		fmt.Printf("[quiz]  user found, lazily init streak with UnlockedDay3=true\n")
		si = &StreakInfo{UnlockedDay3: true, UnlockedDay7: false}
		s.saveStreak(wallet, si)
	} else {
		fmt.Printf("[quiz]  streak found: streak=%d lastCheckin=%s unlocked3=%v unlocked7=%v\n",
			si.Streak, si.LastCheckin, si.UnlockedDay3, si.UnlockedDay7)
	}

	if day == 3 && !si.UnlockedDay3 {
		fmt.Printf("[quiz]  day=3 but UnlockedDay3=false, returning nil\n")
		return nil
	}
	if day == 7 && !si.UnlockedDay7 {
		fmt.Printf("[quiz]  day=7 but UnlockedDay7=false, returning nil\n")
		return nil
	}

	// Get compiled-in questions
	var all map[string]*QuizQuestionRecord
	if day == 3 {
		all = Day3QuestionsData
		fmt.Printf("[quiz]  Day3QuestionsData len=%d\n", len(all))
	} else if day == 7 {
		all = Day7QuestionsData
		fmt.Printf("[quiz]  Day7QuestionsData len=%d\n", len(all))
	} else {
		fmt.Printf("[quiz]  unsupported day=%d, returning nil\n", day)
		return nil
	}

	if all == nil {
		fmt.Printf("[quiz]  WARN: compiled-in questions map is NIL for day=%d!\n", day)
		return nil
	}

	// Filter out already answered (stored in DB per user)
	answered, _ := dbstore.DBGetJson[map[string]bool](dbstore.SECRET_DB, eaNamespace, fmt.Sprintf("answered:%s:day%d", wallet, day))
	answeredCount := 0
	if answered != nil {
		answeredCount = len(*answered)
	}
	fmt.Printf("[quiz]  answered count=%d\n", answeredCount)

	var out []*QuizQuestionRecord
	for id, q := range all {
		if answered != nil && (*answered)[id] {
			continue
		}
		out = append(out, q)
	}
	fmt.Printf("[quiz]  returning %d unanswered questions\n", len(out))
	return out
}

// GetQuizProgress returns total, answered count, and points earned for a quiz day.
func (s *EarlyAccessStore) GetQuizProgress(wallet string, day int) (total, answered int, earned int64) {
	wallet = strings.ToLower(wallet)
	var all map[string]*QuizQuestionRecord
	if day == 3 {
		all = Day3QuestionsData
	} else if day == 7 {
		all = Day7QuestionsData
	} else {
		return 0, 0, 0
	}
	total = len(all)
	answeredSet, _ := dbstore.DBGetJson[map[string]bool](dbstore.SECRET_DB, eaNamespace, fmt.Sprintf("answered:%s:day%d", wallet, day))
	if answeredSet != nil {
		answered = len(*answeredSet)
	}
	// The answered set only records correctly-answered questions (wrong answers not stored),
	// so each entry = ptsPerQ earned.
	ptsPerQ := int64(day3QuizPoints)
	if day == 7 {
		ptsPerQ = day7QuizPoints
	}
	earned = int64(answered) * ptsPerQ
	return total, answered, earned
}

func (s *EarlyAccessStore) SubmitQuizAnswer(wallet, questionID string, answerIndex int) (bool, int64, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet = strings.ToLower(wallet)

	// Find question in compiled-in data
	q, ok := Day3QuestionsData[questionID]
	day := 3
	if !ok {
		q, ok = Day7QuestionsData[questionID]
		day = 7
		if !ok {
			return false, 0, "", fmt.Errorf("question not found")
		}
	}

	// Ensure the quiz is unlocked for this wallet
	si, _ := s.loadStreak(wallet)
	if si == nil {
		user := s.GetUser(wallet)
		if user == nil {
			return false, 0, "", fmt.Errorf("user not found")
		}
		si = &StreakInfo{UnlockedDay3: true, UnlockedDay7: false}
		s.saveStreak(wallet, si)
	}
	if (day == 3 && !si.UnlockedDay3) || (day == 7 && !si.UnlockedDay7) {
		return false, 0, "", fmt.Errorf("quiz not unlocked yet")
	}

	// Check if already answered
	answered, _ := dbstore.DBGetJson[map[string]bool](dbstore.SECRET_DB, eaNamespace, fmt.Sprintf("answered:%s:day%d", wallet, day))
	if answered != nil && (*answered)[questionID] {
		return false, 0, "", fmt.Errorf("question already answered")
	}

	correct := answerIndex == q.CorrectAnswer
	if !correct {
		return false, 0, q.Explanation, nil
	}

	// Award points
	pts := int64(day3QuizPoints)
	if day == 7 {
		pts = day7QuizPoints
	}
	s.addPoints(wallet, pts)

	now := time.Now().UTC().Format(time.RFC3339)
	reason := fmt.Sprintf("quiz_day%d_correct", day)
	s.appendHistory(wallet, fmt.Sprintf("%d", pts), reason, now, nil)

	stats, _ := s.loadStats()
	stats.TotalPointsDistributed += pts
	s.saveStats(stats)

	// Mark as answered
	if answered == nil {
		answered = &map[string]bool{}
	}
	(*answered)[questionID] = true
	dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, fmt.Sprintf("answered:%s:day%d", wallet, day), answered)

	total, _ := s.loadPoints(wallet)
	_ = total

	return true, pts, q.Explanation, nil
}

var taskPoints = map[string]int64{
	"JOIN_EARLY_ACCESS": 100, "VERIFY_EMAIL": 50, "CONNECT_WALLET": 50,
	"JOIN_DISCORD": 100, "JOIN_TELEGRAM": 100, "INVITE_FRIEND": 200,
}

var allTaskTypes = []string{"JOIN_EARLY_ACCESS", "VERIFY_EMAIL", "CONNECT_WALLET", "JOIN_DISCORD", "JOIN_TELEGRAM", "INVITE_FRIEND"}

func generateReferralCode(wallet string) string {
	a := strings.ToLower(wallet)
	if strings.HasPrefix(a, "0x") && len(a) >= 10 {
		return "ref_" + a[len(a)-8:]
	}
	if len(a) >= 8 {
		return "ref_" + a[len(a)-8:]
	}
	h := sha256.Sum256([]byte(wallet))
	return "ref_" + hex.EncodeToString(h[:])[:8]
}

func (s *EarlyAccessStore) Join(wallet, email string, referralCode *string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet = strings.ToLower(wallet)
	if existing, _ := dbstore.DBGetJson[DbUser](dbstore.SECRET_DB, eaNamespace, "user:"+wallet); existing != nil {
		return false, nil
	}

	email = strings.ToLower(email)
	if email != "" {
		if ew, _ := s.loadEmailToWallet(email); ew != "" && ew != wallet {
			return false, fmt.Errorf("email already registered")
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	user := &DbUser{Wallet: wallet, JoinedAt: now, ReferralCode: generateReferralCode(wallet), Email: email}

	pts := int64(100)
	dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "points:"+wallet, &pts)
	s.appendHistory(wallet, "100", "join_early_access", now, nil)

	stats, _ := s.loadStats()
	stats.UserCount++
	stats.TotalPointsDistributed += pts

	if referralCode != nil && *referralCode != "" {
		if rw, _ := s.loadCodeToWallet(strings.ToLower(*referralCode)); rw != "" && rw != wallet {
			user.Referrer = rw
			s.appendReferral(strings.ToLower(*referralCode), wallet)
			b1, b2 := int64(50), int64(200)
			s.addPoints(wallet, b1)
			s.appendHistory(wallet, "50", "referral_reward_invitee", now, &rw)
			s.addPoints(rw, b2)
			s.appendHistory(rw, "200", "referral_reward_inviter", now, &wallet)
			stats.TotalPointsDistributed += b1 + b2
			stats.TotalReferrals++
		}
	}

	dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "user:"+wallet, user)
	s.saveCodeToWallet(user.ReferralCode, wallet)
	if email != "" {
		s.saveEmailToWallet(email, wallet)
	}
	s.saveStats(stats)
	tasks := map[string]string{"JOIN_EARLY_ACCESS": now}
	dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "tasks:"+wallet, &tasks)
	return true, nil
}

func (s *EarlyAccessStore) ClaimTask(wallet, task string) (bool, int64, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet = strings.ToLower(wallet)
	if u, _ := dbstore.DBGetJson[DbUser](dbstore.SECRET_DB, eaNamespace, "user:"+wallet); u == nil {
		return false, 0, 0, fmt.Errorf("user not joined")
	}

	pts, ok := taskPoints[task]
	if !ok {
		return false, 0, 0, fmt.Errorf("invalid task: %s", task)
	}

	tasks, _ := s.loadTasks(wallet)
	if tasks == nil {
		tasks = make(map[string]string)
	}
	if _, done := tasks[task]; done {
		t, _ := s.loadPoints(wallet)
		return false, 0, t, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tasks[task] = now
	s.addPoints(wallet, pts)
	s.appendHistory(wallet, fmt.Sprintf("%d", pts), strings.ToLower(task), now, nil)

	stats, _ := s.loadStats()
	stats.TotalPointsDistributed += pts
	s.saveStats(stats)
	s.saveTasks(wallet, tasks)

	t, _ := s.loadPoints(wallet)
	return true, pts, t, nil
}

func (s *EarlyAccessStore) UpdateReferralCode(wallet, referralCode string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet, code := strings.ToLower(wallet), strings.ToLower(referralCode)
	user, _ := dbstore.DBGetJson[DbUser](dbstore.SECRET_DB, eaNamespace, "user:"+wallet)
	if user == nil {
		return false, fmt.Errorf("user not joined")
	}
	if user.Referrer != "" {
		return false, fmt.Errorf("referral already set")
	}
	if user.ReferralCode == code {
		return false, fmt.Errorf("cannot use own code")
	}

	rw, _ := s.loadCodeToWallet(code)
	if rw == "" || rw == wallet {
		return false, fmt.Errorf("invalid code")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	user.Referrer = rw
	s.appendReferral(code, wallet)
	s.addPoints(wallet, 50)
	s.appendHistory(wallet, "50", "referral_reward_invitee", now, &rw)
	s.addPoints(rw, 200)
	s.appendHistory(rw, "200", "referral_reward_inviter", now, &wallet)

	stats, _ := s.loadStats()
	stats.TotalPointsDistributed += 250
	stats.TotalReferrals++
	dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "user:"+wallet, user)
	s.saveStats(stats)
	return true, nil
}

func (s *EarlyAccessStore) GetTasks(wallet string) []*TaskStatusInfo {
	tasks, _ := s.loadTasks(strings.ToLower(wallet))
	var r []*TaskStatusInfo
	for _, t := range allTaskTypes {
		st := &TaskStatusInfo{Task: t}
		if tasks != nil {
			if v, ok := tasks[t]; ok {
				st.Completed = true
				st.ClaimedAt = &v
			}
		}
		r = append(r, st)
	}
	return r
}

// ── Read-only ────────────────────────────────────────────

func (s *EarlyAccessStore) GetUser(wallet string) *UserRecord {
	v, _ := dbstore.DBGetJson[DbUser](dbstore.SECRET_DB, eaNamespace, "user:"+strings.ToLower(wallet))
	if v == nil {
		return nil
	}
	return &UserRecord{Wallet: v.Wallet, JoinedAt: v.JoinedAt, Referrer: v.Referrer, ReferralCode: v.ReferralCode, Email: v.Email}
}

func (s *EarlyAccessStore) GetUserByReferralCode(code string) *UserRecord {
	w, _ := s.loadCodeToWallet(strings.ToLower(code))
	if w == "" {
		return nil
	}
	return s.GetUser(w)
}

func (s *EarlyAccessStore) GetUserByEmail(email string) *UserRecord {
	w, _ := s.loadEmailToWallet(strings.ToLower(email))
	if w == "" {
		return nil
	}
	return s.GetUser(w)
}

func (s *EarlyAccessStore) GetPoints(wallet string) string {
	v, _ := s.loadPoints(strings.ToLower(wallet))
	return fmt.Sprintf("%d", v)
}

func (s *EarlyAccessStore) GetReferralInfo(wallet string) (code, referrer string, earnings int64, referrals []string) {
	u, _ := dbstore.DBGetJson[DbUser](dbstore.SECRET_DB, eaNamespace, "user:"+strings.ToLower(wallet))
	if u != nil {
		code, referrer = u.ReferralCode, u.Referrer
	}
	r, _ := s.loadReferrals(code)
	if len(r) > 0 {
		referrals = make([]string, len(r))
		copy(referrals, r)
		earnings = int64(len(r)) * 200
	}
	return
}

func (s *EarlyAccessStore) GetPointsHistory(wallet string, offset, limit int) ([]*HistoryRecord, int) {
	list, _ := dbstore.DBGetJson[[]DbHistory](dbstore.SECRET_DB, eaNamespace, "history:"+strings.ToLower(wallet))
	if list == nil {
		return nil, 0
	}
	recs := *list
	total := len(recs)
	out := make([]*HistoryRecord, total)
	for i := 0; i < total; i++ {
		r := recs[total-1-i]
		item := &HistoryRecord{Amount: r.Amount, Reason: r.Reason, Timestamp: r.Timestamp, RelatedUser: r.RelatedUser}
		if r.RelatedUser != nil && *r.RelatedUser != "" {
			if rel := s.GetUser(*r.RelatedUser); rel != nil && rel.Email != "" {
				item.RelatedEmail = &rel.Email
			}
		}
		out[i] = item
	}
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}
	return out[offset:end], total
}

func (s *EarlyAccessStore) GetStats() StatsRecord {
	v, _ := s.loadStats()
	return v
}

// ── Internal helpers ─────────────────────────────────────

func (s *EarlyAccessStore) loadPoints(w string) (int64, error) {
	v, _ := dbstore.DBGetJson[int64](dbstore.SECRET_DB, eaNamespace, "points:"+w)
	if v == nil {
		return 0, nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) addPoints(w string, d int64) error {
	c, _ := s.loadPoints(w)
	n := c + d
	return dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "points:"+w, &n)
}

func (s *EarlyAccessStore) appendHistory(w, amount, reason, ts string, related *string) error {
	list, _ := dbstore.DBGetJson[[]DbHistory](dbstore.SECRET_DB, eaNamespace, "history:"+w)
	var r []DbHistory
	if list != nil {
		r = *list
	}
	r = append(r, DbHistory{Amount: amount, Reason: reason, Timestamp: ts, RelatedUser: related})
	return dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "history:"+w, &r)
}

func (s *EarlyAccessStore) loadStats() (StatsRecord, error) {
	v, _ := dbstore.DBGetJson[DbStats](dbstore.SECRET_DB, eaNamespace, "stats")
	if v == nil {
		return StatsRecord{}, nil
	}
	return StatsRecord{UserCount: v.UserCount, TotalPointsDistributed: v.TotalPointsDistributed, TotalReferrals: v.TotalReferrals}, nil
}

func (s *EarlyAccessStore) saveStats(v StatsRecord) error {
	return dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "stats", &DbStats{UserCount: v.UserCount, TotalPointsDistributed: v.TotalPointsDistributed, TotalReferrals: v.TotalReferrals})
}

func (s *EarlyAccessStore) loadCodeToWallet(code string) (string, error) {
	v, _ := dbstore.DBGetJson[string](dbstore.SECRET_DB, eaNamespace, "code:"+code)
	if v == nil {
		return "", nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) saveCodeToWallet(code, wallet string) error {
	return dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "code:"+code, &wallet)
}

func (s *EarlyAccessStore) loadEmailToWallet(email string) (string, error) {
	v, _ := dbstore.DBGetJson[string](dbstore.SECRET_DB, eaNamespace, "email:"+email)
	if v == nil {
		return "", nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) saveEmailToWallet(email, wallet string) error {
	return dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "email:"+email, &wallet)
}

func (s *EarlyAccessStore) loadReferrals(code string) ([]string, error) {
	v, _ := dbstore.DBGetJson[[]string](dbstore.SECRET_DB, eaNamespace, "referrals:"+code)
	if v == nil {
		return nil, nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) appendReferral(code, wallet string) error {
	l, _ := s.loadReferrals(code)
	l = append(l, wallet)
	return dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "referrals:"+code, &l)
}

func (s *EarlyAccessStore) loadTasks(wallet string) (map[string]string, error) {
	v, _ := dbstore.DBGetJson[map[string]string](dbstore.SECRET_DB, eaNamespace, "tasks:"+wallet)
	if v == nil {
		return nil, nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) saveTasks(wallet string, tasks map[string]string) error {
	return dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "tasks:"+wallet, &tasks)
}

// ── Export / Import ───────────────────────────────────────

type ExportedData struct {
	Users     map[string]DbUser            `json:"users"`
	Points    map[string]int64             `json:"points"`
	History   map[string][]DbHistory       `json:"history"`
	Tasks     map[string]map[string]string `json:"tasks"`
	Codes     map[string]string            `json:"codes"`
	Emails    map[string]string            `json:"emails"`
	Referrals map[string][]string          `json:"referrals"`
	Stats     *DbStats                     `json:"stats"`
}

func (s *EarlyAccessStore) Export() string {
	d := ExportedData{
		Users: make(map[string]DbUser), Points: make(map[string]int64),
		History: make(map[string][]DbHistory), Tasks: make(map[string]map[string]string),
		Codes: make(map[string]string), Emails: make(map[string]string),
		Referrals: make(map[string][]string),
	}
	ps := []byte(eaNamespace + "_")
	iter, err := dbstore.SECRET_DB.NewIter(&pebble.IterOptions{LowerBound: ps})
	if err != nil {
		return "{}"
	}
	defer iter.Close()
	for iter.First(); iter.Valid(); iter.Next() {
		k := string(iter.Key())
		if !strings.HasPrefix(k, eaNamespace+"_") {
			break
		}
		key := k[len(ps):]
		raw := make([]byte, len(iter.Value()))
		copy(raw, iter.Value())
		val, err := dbstore.SECRET_DB.Unseal(raw)
		if err != nil {
			continue
		}

		switch {
		case strings.HasPrefix(key, "user:"):
			var v DbUser
			if json.Unmarshal(val, &v) == nil {
				d.Users[key[5:]] = v
			}
		case strings.HasPrefix(key, "points:"):
			var v int64
			if json.Unmarshal(val, &v) == nil {
				d.Points[key[7:]] = v
			}
		case strings.HasPrefix(key, "history:"):
			var v []DbHistory
			if json.Unmarshal(val, &v) == nil {
				d.History[key[8:]] = v
			}
		case strings.HasPrefix(key, "tasks:"):
			var v map[string]string
			if json.Unmarshal(val, &v) == nil {
				d.Tasks[key[6:]] = v
			}
		case strings.HasPrefix(key, "code:"):
			var v string
			if json.Unmarshal(val, &v) == nil {
				d.Codes[key[5:]] = v
			}
		case strings.HasPrefix(key, "email:"):
			var v string
			if json.Unmarshal(val, &v) == nil {
				d.Emails[key[6:]] = v
			}
		case strings.HasPrefix(key, "referrals:"):
			var v []string
			if json.Unmarshal(val, &v) == nil {
				d.Referrals[key[10:]] = v
			}
		case key == "stats":
			var v DbStats
			if json.Unmarshal(val, &v) == nil {
				d.Stats = &v
			}
		}
	}
	b, _ := json.Marshal(d)
	return string(b)
}

var importOnce sync.Once

func TryImport(data string) error {
	var errResult error
	importOnce.Do(func() {
		flag, _ := dbstore.DBGetJson[bool](dbstore.SECRET_DB, eaNamespace, "imported")
		if flag != nil && *flag {
			errResult = fmt.Errorf("import already completed")
			return
		}
		var d ExportedData
		if err := json.Unmarshal([]byte(data), &d); err != nil {
			errResult = fmt.Errorf("unmarshal: %w", err)
			return
		}
		for k, v := range d.Users {
			if err := dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "user:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.Points {
			if err := dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "points:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.History {
			if err := dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "history:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.Tasks {
			if err := dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "tasks:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.Codes {
			if err := dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "code:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.Emails {
			if err := dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "email:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.Referrals {
			if err := dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "referrals:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		if d.Stats != nil {
			if err := dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "stats", d.Stats); err != nil {
				errResult = err
				return
			}
		}
		importedFlag := true
		dbstore.DBSetJson(dbstore.SECRET_DB, eaNamespace, "imported", &importedFlag)
	})
	return errResult
}
