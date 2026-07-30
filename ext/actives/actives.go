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
)

const eaNamespace = "EARLY_POINTS"

var (
	Store           = &EarlyAccessStore{}
	CheckinTestMode bool
	pebbleDB        *pebble.DB
)

// InitDB sets the pebble DB instance for the actives package.
func InitDB(db *pebble.DB) {
	pebbleDB = db
}

func eaKey(key string) []byte {
	return []byte(eaNamespace + "/" + key)
}

func dbGetJson[T any](key string) (*T, error) {
	v, closer, err := pebbleDB.Get(eaKey(key))
	if err != nil {
		if err == pebble.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	defer closer.Close()
	if len(v) == 0 {
		return nil, nil
	}
	var val T
	if err := json.Unmarshal(v, &val); err != nil {
		return nil, err
	}
	return &val, nil
}

func dbSetJson(key string, val any) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return pebbleDB.Set(eaKey(key), b, pebble.Sync)
}

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
	WeekCheckin  int    `json:"weekCheckin"` // check-in count this week (max 7)
	WeekKey      string `json:"weekKey"`     // ISO year-week "2006-W05"
	UnlockedDay3 bool   `json:"unlockedDay3"`
	UnlockedDay7 bool   `json:"unlockedDay7"`
}

const (
	checkinPoints  = 5
	day3QuizPoints = 50
	day7QuizPoints = 100
	maxWeekCheckin = 7
)

func todayKey() string {
	return time.Now().UTC().Format("2006-01-02")
}

func weekKey() string {
	y, w := time.Now().UTC().ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

func (s *EarlyAccessStore) loadStreak(wallet string) (*StreakInfo, error) {
	v, _ := dbGetJson[StreakInfo]("streak:" + strings.ToLower(wallet))
	return v, nil
}

func (s *EarlyAccessStore) saveStreak(wallet string, si *StreakInfo) error {
	return dbSetJson("streak:"+strings.ToLower(wallet), si)
}

func (s *EarlyAccessStore) DailyCheckin(wallet, dateStr string) (int, int64, bool, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet = strings.ToLower(wallet)

	si, _ := s.loadStreak(wallet)
	if si == nil {
		si = &StreakInfo{}
	}

	// ── Weekly limit ──────────────────────────────────────
	currentWeek := weekKey()
	if si.WeekKey != currentWeek {
		// New week: reset counter
		si.WeekCheckin = 0
		si.WeekKey = currentWeek
	}

	if dateStr == "" {
		if CheckinTestMode && si.LastCheckin != "" {
			lastDt, _ := time.Parse("2006-01-02", si.LastCheckin)
			dateStr = lastDt.AddDate(0, 0, 1).Format("2006-01-02")
		} else {
			dateStr = todayKey()
		}
	}
	fmt.Printf("[checkin] DailyCheckin wallet=%s date=%s testMode=%v week=%s count=%d\n",
		wallet, dateStr, CheckinTestMode, si.WeekKey, si.WeekCheckin)

	// Weekly limit applies in ALL modes (including test mode)
	if si.WeekCheckin >= maxWeekCheckin {
		return 0, 0, false, false, fmt.Errorf("weekly limit reached: max %d check-ins per week", maxWeekCheckin)
	}

	if !CheckinTestMode {
		dt, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return 0, 0, false, false, fmt.Errorf("invalid date format, use YYYY-MM-DD")
		}
		today := time.Now().UTC().Truncate(24 * time.Hour)
		if dt.After(today) {
			return 0, 0, false, false, fmt.Errorf("cannot check in for a future date")
		}

		if si.LastCheckin == dateStr {
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
		si.Streak = 1
	} else {
		si.Streak = 1
	}
	si.LastCheckin = dateStr
	si.WeekCheckin++

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

	// Async on-chain reward via contract
	go Distribute([]string{wallet}, []int64{checkinPoints})

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

func (s *EarlyAccessStore) GetCheckinStatus(wallet string) (checkedIn bool, canCheckin bool, weekCount int, weekMax int, streak int) {
	wallet = strings.ToLower(wallet)
	weekMax = maxWeekCheckin
	si, _ := s.loadStreak(wallet)
	if si == nil {
		canCheckin = true
		return
	}
	today := todayKey()
	currentWeek := weekKey()
	if si.WeekKey != currentWeek {
		weekCount = 0
	} else {
		weekCount = si.WeekCheckin
	}
	checkedIn = si.LastCheckin == today
	canCheckin = !checkedIn && weekCount < weekMax
	streak = si.Streak
	if !CheckinTestMode && si.LastCheckin != today {
		streak = 0
	}
	return
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
	answered, _ := dbGetJson[map[string]bool](fmt.Sprintf("answered:%s:day%d", wallet, day))
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
	answeredSet, _ := dbGetJson[map[string]bool](fmt.Sprintf("answered:%s:day%d", wallet, day))
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
	answered, _ := dbGetJson[map[string]bool](fmt.Sprintf("answered:%s:day%d", wallet, day))
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
	dbSetJson(fmt.Sprintf("answered:%s:day%d", wallet, day), answered)

	// Async on-chain reward via contract
	go Distribute([]string{wallet}, []int64{pts})

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
	if existing, _ := dbGetJson[DbUser]("user:" + wallet); existing != nil {
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
	dbSetJson("points:"+wallet, &pts)
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

	dbSetJson("user:"+wallet, user)
	s.saveCodeToWallet(user.ReferralCode, wallet)
	if email != "" {
		s.saveEmailToWallet(email, wallet)
	}
	s.saveStats(stats)
	tasks := map[string]string{"JOIN_EARLY_ACCESS": now}
	dbSetJson("tasks:"+wallet, &tasks)
	return true, nil
}

func (s *EarlyAccessStore) ClaimTask(wallet, task string) (bool, int64, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet = strings.ToLower(wallet)
	if u, _ := dbGetJson[DbUser]("user:" + wallet); u == nil {
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

	go Distribute([]string{wallet}, []int64{pts})

	t, _ := s.loadPoints(wallet)
	return true, pts, t, nil
}

func (s *EarlyAccessStore) UpdateReferralCode(wallet, referralCode string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet, code := strings.ToLower(wallet), strings.ToLower(referralCode)
	user, _ := dbGetJson[DbUser]("user:" + wallet)
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
	dbSetJson("user:"+wallet, user)
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
	v, _ := dbGetJson[DbUser]("user:" + strings.ToLower(wallet))
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
	u, _ := dbGetJson[DbUser]("user:" + strings.ToLower(wallet))
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
	list, _ := dbGetJson[[]DbHistory]("history:" + strings.ToLower(wallet))
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
	v, _ := dbGetJson[int64]("points:" + w)
	if v == nil {
		return 0, nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) addPoints(w string, d int64) error {
	c, _ := s.loadPoints(w)
	n := c + d
	return dbSetJson("points:"+w, &n)
}

func (s *EarlyAccessStore) appendHistory(w, amount, reason, ts string, related *string) error {
	list, _ := dbGetJson[[]DbHistory]("history:" + w)
	var r []DbHistory
	if list != nil {
		r = *list
	}
	r = append(r, DbHistory{Amount: amount, Reason: reason, Timestamp: ts, RelatedUser: related})
	return dbSetJson("history:"+w, &r)
}

func (s *EarlyAccessStore) loadStats() (StatsRecord, error) {
	v, _ := dbGetJson[DbStats]("stats")
	if v == nil {
		return StatsRecord{}, nil
	}
	return StatsRecord{UserCount: v.UserCount, TotalPointsDistributed: v.TotalPointsDistributed, TotalReferrals: v.TotalReferrals}, nil
}

func (s *EarlyAccessStore) saveStats(v StatsRecord) error {
	return dbSetJson("stats", &DbStats{UserCount: v.UserCount, TotalPointsDistributed: v.TotalPointsDistributed, TotalReferrals: v.TotalReferrals})
}

func (s *EarlyAccessStore) loadCodeToWallet(code string) (string, error) {
	v, _ := dbGetJson[string]("code:" + code)
	if v == nil {
		return "", nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) saveCodeToWallet(code, wallet string) error {
	return dbSetJson("code:"+code, &wallet)
}

func (s *EarlyAccessStore) loadEmailToWallet(email string) (string, error) {
	v, _ := dbGetJson[string]("email:" + email)
	if v == nil {
		return "", nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) saveEmailToWallet(email, wallet string) error {
	return dbSetJson("email:"+email, &wallet)
}

func (s *EarlyAccessStore) loadReferrals(code string) ([]string, error) {
	v, _ := dbGetJson[[]string]("referrals:" + code)
	if v == nil {
		return nil, nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) appendReferral(code, wallet string) error {
	l, _ := s.loadReferrals(code)
	l = append(l, wallet)
	return dbSetJson("referrals:"+code, &l)
}

func (s *EarlyAccessStore) loadTasks(wallet string) (map[string]string, error) {
	v, _ := dbGetJson[map[string]string]("tasks:" + wallet)
	if v == nil {
		return nil, nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) saveTasks(wallet string, tasks map[string]string) error {
	return dbSetJson("tasks:"+wallet, &tasks)
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
	ps := []byte(eaNamespace + "/")
	iter, err := pebbleDB.NewIter(&pebble.IterOptions{LowerBound: ps})
	if err != nil {
		return "{}"
	}
	defer iter.Close()
	for iter.First(); iter.Valid(); iter.Next() {
		k := string(iter.Key())
		if !strings.HasPrefix(k, eaNamespace+"/") {
			break
		}
		key := k[len(ps):]
		val := make([]byte, len(iter.Value()))
		copy(val, iter.Value())

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
		flag, _ := dbGetJson[bool]("imported")
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
			if err := dbSetJson("user:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.Points {
			if err := dbSetJson("points:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.History {
			if err := dbSetJson("history:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.Tasks {
			if err := dbSetJson("tasks:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.Codes {
			if err := dbSetJson("code:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.Emails {
			if err := dbSetJson("email:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.Referrals {
			if err := dbSetJson("referrals:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		if d.Stats != nil {
			if err := dbSetJson("stats", d.Stats); err != nil {
				errResult = err
				return
			}
		}
		importedFlag := true
		dbSetJson("imported", &importedFlag)
	})
	return errResult
}

// SyncMintHistory scans all historical points and batch-mints FLUX_TEST tokens.
// Only mints wallets that haven't been minted yet. Max batchSize per on-chain tx.
func RunSyncMintHistory(batchSize int) (int, error) {
	if !IsFluxChainReady() {
		return 0, fmt.Errorf("FLUX chain not configured")
	}
	if batchSize <= 0 {
		batchSize = 50
	}

	ps := []byte(eaNamespace + "/points:")
	iter, err := pebbleDB.NewIter(&pebble.IterOptions{LowerBound: ps})
	if err != nil {
		return 0, fmt.Errorf("iter: %w", err)
	}
	defer iter.Close()

	var (
		wallets []string
		points  []int64
		minted  int
	)

	for iter.First(); iter.Valid(); iter.Next() {
		k := string(iter.Key())
		if !strings.HasPrefix(k, eaNamespace+"/points:") {
			break
		}
		wallet := k[len(eaNamespace+"/points:"):]

		flag, _ := dbGetJson[bool]("minted:" + wallet)
		if flag != nil && *flag {
			continue
		}

		val := make([]byte, len(iter.Value()))
		copy(val, iter.Value())
		var pts int64
		if err := json.Unmarshal(val, &pts); err != nil || pts <= 0 {
			continue
		}

		wallets = append(wallets, wallet)
		points = append(points, pts)
	}

	if len(wallets) == 0 {
		return 0, nil
	}

	for i := 0; i < len(wallets); i += batchSize {
		end := i + batchSize
		if end > len(wallets) {
			end = len(wallets)
		}
		batchWallets := wallets[i:end]
		batchPoints := points[i:end]

		fmt.Printf("[mint] batch %d/%d: %d wallets\n", i/batchSize+1, (len(wallets)+batchSize-1)/batchSize, len(batchWallets))

		if err := Distribute(batchWallets, batchPoints); err != nil {
			return minted, fmt.Errorf("batch mint failed at %d: %w", i, err)
		}

		// Mark as minted
		for _, w := range batchWallets {
			t := true
			dbSetJson("minted:"+w, &t)
			minted++
		}
	}

	return minted, nil
}

// StartMintWorker starts a background goroutine that auto-discovers unminted points
// and batch-mints FLUX_TEST tokens every 5 seconds.
func StartMintWorker() {
	go func() {
		if !IsFluxChainReady() {
			fmt.Println("[mint-worker] FLUX chain not ready, worker disabled")
			return
		}
		fmt.Println("[mint-worker] started (interval: 5s)")
		for {
			time.Sleep(5 * time.Second)
			n, err := RunSyncMintHistory(50)
			if err != nil {
				fmt.Printf("[mint-worker] error: %v\n", err)
			} else if n > 0 {
				fmt.Printf("[mint-worker] minted %d wallets\n", n)
			}
		}
	}()
}
