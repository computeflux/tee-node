package actives

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
)

// ── 客户端 IP（HTTP 中间件注入）──────────────────────────

type ctxKey string

const clientIPKey ctxKey = "actives_client_ip"

// WithClientIP 将客户端 IP 注入请求 context（由 cmd/server 中间件调用）。
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey, ip)
}

// ClientIPFromContext 从请求 context 读取客户端 IP。
func ClientIPFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(clientIPKey).(string); ok {
		return v
	}
	return ""
}

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
	Alias        string `json:"alias,omitempty"`
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

// DbConfig 防女巫开关 + 更新审计（key: "config"）。
type DbConfig struct {
	IpCheckEnabled            bool   `json:"ipCheckEnabled"`
	DeferredSettlementEnabled bool   `json:"deferredSettlementEnabled"`
	UpdatedBy                 string `json:"updatedBy"`
	UpdatedAt                 string `json:"updatedAt"`
}

// DbIpRecord 用户 IP 记录（key: "ip:<wallet>"）。
type DbIpRecord struct {
	FirstIP   string   `json:"firstIp"`
	LastIPs   []string `json:"lastIps"`
	FirstSeen string   `json:"firstSeen"`
	LastSeen  string   `json:"lastSeen"`
}

// DbReferral 推荐关系（key: "referrals:<code>"）。Settled=false 表示邀请人积分仍在 pending。
type DbReferral struct {
	Wallet    string `json:"wallet"`
	Settled   bool   `json:"settled"`
	CreatedAt string `json:"createdAt"`
}

// DbPendingRef 待结算推荐（key: "pendingref:<inviteeWallet>"）。
type DbPendingRef struct {
	Inviter   string `json:"inviter"`
	Code      string `json:"code"`
	CreatedAt string `json:"createdAt"`
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
	day3QuizPoints = 10
	day7QuizPoints = 20
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

// resetWeekIfNeeded 检测跨周并重置周签到计数,写回 DB。
// 返回是否发生了重置。
func (s *EarlyAccessStore) resetWeekIfNeeded(wallet string, si *StreakInfo) bool {
	currentWeek := weekKey()
	if si.WeekKey != currentWeek {
		si.WeekCheckin = 0
		si.WeekKey = currentWeek
		_ = s.saveStreak(wallet, si)
		fmt.Printf("[checkin] resetWeek wallet=%s oldWeek=%s newWeek=%s oldCount=%d -> 0\n",
			wallet, si.WeekKey, currentWeek, si.WeekCheckin)
		return true
	}
	fmt.Printf("[checkin] resetWeek wallet=%s week=%s count=%d (no reset)\n", wallet, si.WeekKey, si.WeekCheckin)
	return false
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
	// 跨周自动重置(写回 DB),周一可签到次数/已签到次数归零
	s.resetWeekIfNeeded(wallet, si)

	if dateStr == "" {
		if CheckinTestMode && si.LastCheckin != "" {
			lastDt, _ := time.Parse("2006-01-02", si.LastCheckin)
			next := lastDt.AddDate(0, 0, 1)
			// test mode 快进限制:不能超过本周日,避免 lastCheckin 变成未来无限推进
			now := time.Now().UTC()
			monday := now.AddDate(0, 0, -((int(now.Weekday()) + 6) % 7)) // ISO 周起点(周一)
			sunday := monday.AddDate(0, 0, 6).Truncate(24 * time.Hour)
			if next.After(sunday) {
				return 0, 0, false, false,
					fmt.Errorf("test mode: fast-forward limited to this week (max %s)", sunday.Format("2006-01-02"))
			}
			dateStr = next.Format("2006-01-02")
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

	// 有效行为：结算待发的推荐奖励（邀请人 +75）
	s.settlePending(wallet)

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

	// 同上:返回存储的连续签到天数,而不是在未当天签到时报 0
	return si.Streak
}

func (s *EarlyAccessStore) GetCheckinStatus(wallet string) (checkedIn bool, canCheckin bool, weekCount int, weekMax int, streak int) {
	wallet = strings.ToLower(wallet)
	weekMax = maxWeekCheckin
	si, _ := s.loadStreak(wallet)
	if si == nil {
		canCheckin = true
		fmt.Printf("[checkin] status wallet=%s streak=nil -> canCheckin=true\n", wallet)
		return
	}
	// 跨周立即重置并写回,前端周一即可看到归零后的次数
	s.resetWeekIfNeeded(wallet, si)
	today := todayKey()
	weekCount = si.WeekCheckin
	checkedIn = si.LastCheckin == today
	canCheckin = !checkedIn && weekCount < weekMax

	// 连续签到天数以最后一次签到为准,当天签到前也保持展示,前端需要看到已完成的签到
	streak = si.Streak
	fmt.Printf("[checkin] status wallet=%s today=%s lastCheckin=%s week=%s weekCount=%d checkedIn=%v canCheckin=%v streak=%d\n",
		wallet, today, si.LastCheckin, si.WeekKey, weekCount, checkedIn, canCheckin, streak)
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
	var questionPool map[string]*QuizQuestionRecord
	if day == 3 {
		questionPool = Day3QuestionsData
		fmt.Printf("[quiz]  Day3QuestionsData len=%d\n", len(questionPool))
	} else if day == 7 {
		questionPool = Day7QuestionsData
		fmt.Printf("[quiz]  Day7QuestionsData len=%d\n", len(questionPool))
	} else {
		fmt.Printf("[quiz]  unsupported day=%d, returning nil\n", day)
		return nil
	}

	if questionPool == nil {
		fmt.Printf("[quiz]  WARN: compiled-in questions map is NIL for day=%d!\n", day)
		return nil
	}

	// Filter out already answered
	answered, _ := dbGetJson[map[string]bool](fmt.Sprintf("answered:%s:day%d", wallet, day))

	// Collect unanswered questions
	var unanswered []*QuizQuestionRecord
	for id, q := range questionPool {
		if answered != nil && (*answered)[id] {
			continue
		}
		unanswered = append(unanswered, q)
	}

	// Randomly select up to 5 questions
	count := 5
	if len(unanswered) < count {
		count = len(unanswered)
	}
	var out []*QuizQuestionRecord
	if count > 0 {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		rng.Shuffle(len(unanswered), func(i, j int) { unanswered[i], unanswered[j] = unanswered[j], unanswered[i] })
		out = unanswered[:count]
	}
	fmt.Printf("[quiz]  returning %d questions (from %d unanswered)\n", len(out), len(unanswered))
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
	// Total is at most 5 per round (matches GetQuestionsForDay limit)
	// Each round shows up to 5 questions
	total = 5
	if len(all) < total {
		total = len(all)
	}
	answeredSet, _ := dbGetJson[map[string]bool](fmt.Sprintf("answered:%s:day%d", wallet, day))
	if answeredSet != nil {
		answered = len(*answeredSet)
		if answered > total {
			answered = total
		}
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

	// 有效行为：结算待发的推荐奖励（邀请人 +75）
	s.settlePending(wallet)

	// Async on-chain reward via contract
	go Distribute([]string{wallet}, []int64{pts})

	total, _ := s.loadPoints(wallet)
	_ = total

	return true, pts, q.Explanation, nil
}

var taskPoints = map[string]int64{
	"JOIN_EARLY_ACCESS": 100, "VERIFY_EMAIL": 50, "CONNECT_WALLET": 50,
	"JOIN_DISCORD": 100, "JOIN_TELEGRAM": 100, "INVITE_FRIEND": 75,
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

// uniqueReferralCode 生成未被占用的邀请码：优先 wallet 后 8 位，
// 碰撞时退化为 sha256 哈希的更长片段，直至唯一（兜底加随机后缀）。
// 调用方需持有锁。
func (s *EarlyAccessStore) uniqueReferralCode(wallet string) string {
	base := generateReferralCode(wallet)
	if owner, _ := s.loadCodeToWallet(base); owner == "" {
		return base
	}
	h := sha256.Sum256([]byte(wallet))
	hexHash := hex.EncodeToString(h[:])
	for i := 8; i <= len(hexHash); i += 8 {
		code := "ref_" + hexHash[:i]
		if owner, _ := s.loadCodeToWallet(code); owner == "" {
			return code
		}
	}
	return fmt.Sprintf("ref_%s_%d", hexHash[:8], time.Now().UnixNano()%1000000)
}

func (s *EarlyAccessStore) Join(wallet, email, ip string, referralCode *string) (bool, string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet = strings.ToLower(wallet)
	if existing, _ := dbGetJson[DbUser]("user:" + wallet); existing != nil {
		return false, "", false, nil
	}

	email = strings.ToLower(email)
	if email != "" {
		if ew, _ := s.loadEmailToWallet(email); ew != "" && ew != wallet {
			return false, "", false, fmt.Errorf("email already registered")
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	user := &DbUser{Wallet: wallet, JoinedAt: now, ReferralCode: s.uniqueReferralCode(wallet), Email: email}

	pts := int64(100)
	dbSetJson("points:"+wallet, &pts)
	s.appendHistory(wallet, "100", "join_early_access", now, nil)

	stats, _ := s.loadStats()
	stats.UserCount++
	stats.TotalPointsDistributed += pts

	warning := ""
	referralAwarded := false
	cfg := loadAntiSybilConfig()

	if referralCode != nil && *referralCode != "" {
		code := strings.ToLower(*referralCode)
		if rw, _ := s.loadCodeToWallet(code); rw != "" && rw != wallet {
			// 记录到邀请人当前 code 下：别名变更后老链接（旧码）仍可用，推荐记录不丢失
			inviterCode := code
			if inv := s.GetUser(rw); inv != nil && inv.ReferralCode != "" {
				inviterCode = inv.ReferralCode
			}
			// IP 防女巫：邀请人与被邀请人相同 IP → 不建立推荐关系、不发推荐奖励
			if cfg.IpCheckEnabled && ip != "" && s.ipMatches(ip, rw) {
				warning = "Not eligible for referral reward, only base registration reward"
			} else {
				user.Referrer = rw
				s.appendReferral(inviterCode, wallet, now, !cfg.DeferredSettlementEnabled)
				stats.TotalReferrals++

				if cfg.DeferredSettlementEnabled {
					// 延迟结算：邀请人的 75 分待被邀请人完成签到/答题后发放
					s.savePendingRef(wallet, rw, inviterCode, now)
					s.addPoints(wallet, 50)
					s.appendHistory(wallet, "50", "referral_reward_invitee", now, &rw)
					stats.TotalPointsDistributed += 50
				} else {
					s.addPoints(wallet, 50)
					s.appendHistory(wallet, "50", "referral_reward_invitee", now, &rw)
					s.addPoints(rw, 75)
					s.appendHistory(rw, "75", "referral_reward_inviter", now, &wallet)
					stats.TotalPointsDistributed += 125
				}
				// 特定邀请码额外积分：奖励给通过该码加入的被邀请人
				if bonus := referralBonusFor(code); bonus > 0 {
					s.addPoints(wallet, bonus)
					s.appendHistory(wallet, fmt.Sprintf("%d", bonus), "referral_bonus", now, &rw)
					stats.TotalPointsDistributed += bonus
				}
				referralAwarded = true
			}
		}
	}

	dbSetJson("user:"+wallet, user)
	s.saveCodeToWallet(user.ReferralCode, wallet)
	if email != "" {
		s.saveEmailToWallet(email, wallet)
	}
	s.recordIp(wallet, ip)
	s.saveStats(stats)
	tasks := map[string]string{"JOIN_EARLY_ACCESS": now}
	dbSetJson("tasks:"+wallet, &tasks)
	return true, warning, referralAwarded, nil
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

// CheckReferralEligibility 加入前预检查：查询当前 IP 使用该邀请码加入是否可获得推荐奖励。
// 与 Join / UpdateReferralCode 中的防女巫逻辑保持一致，供前端在提交加入前提示用户。
// wallet 为空时跳过"已加入/自己邀请自己"检查，仅做同 IP 判断。
// 返回 eligible（是否可获得奖励）与 reason（不可获得时的原因文案，空串表示可以）。
func (s *EarlyAccessStore) CheckReferralEligibility(wallet, ip, referralCode string) (bool, string) {
	wallet = strings.ToLower(wallet)
	code := strings.ToLower(referralCode)
	if wallet != "" {
		if u, _ := dbGetJson[DbUser]("user:" + wallet); u != nil {
			return false, "already joined"
		}
	}
	rw, _ := s.loadCodeToWallet(code)
	if rw == "" || (wallet != "" && rw == wallet) {
		return false, "invalid code"
	}
	cfg := loadAntiSybilConfig()
	if cfg.IpCheckEnabled && ip != "" && s.ipMatches(ip, rw) {
		return false, "Not eligible for referral reward, only base registration reward"
	}
	return true, ""
}

func (s *EarlyAccessStore) UpdateReferralCode(wallet, ip, referralCode string) (bool, string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet, code := strings.ToLower(wallet), strings.ToLower(referralCode)
	user, _ := dbGetJson[DbUser]("user:" + wallet)
	if user == nil {
		return false, "", false, fmt.Errorf("user not joined")
	}
	if user.Referrer != "" {
		return false, "", false, fmt.Errorf("referral already set")
	}
	if user.ReferralCode == code {
		return false, "", false, fmt.Errorf("cannot use own code")
	}

	rw, _ := s.loadCodeToWallet(code)
	if rw == "" || rw == wallet {
		return false, "", false, fmt.Errorf("invalid code")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	warning := ""
	referralAwarded := false
	cfg := loadAntiSybilConfig()

	// IP 防女巫：相同 IP 不建立推荐关系、不发推荐奖励
	if cfg.IpCheckEnabled && ip != "" && s.ipMatches(ip, rw) {
		warning = "Not eligible for referral reward, only base registration reward"
	} else {
		// 记录到邀请人当前 code 下：别名变更后老链接（旧码）仍可用，推荐记录不丢失
		inviterCode := code
		if inv := s.GetUser(rw); inv != nil && inv.ReferralCode != "" {
			inviterCode = inv.ReferralCode
		}
		user.Referrer = rw
		s.appendReferral(inviterCode, wallet, now, !cfg.DeferredSettlementEnabled)
		s.addPoints(wallet, 50)
		s.appendHistory(wallet, "50", "referral_reward_invitee", now, &rw)
		stats, _ := s.loadStats()
		stats.TotalReferrals++

		if cfg.DeferredSettlementEnabled {
			// 延迟结算：邀请人的 75 分待被邀请人完成签到/答题后发放
			s.savePendingRef(wallet, rw, inviterCode, now)
			stats.TotalPointsDistributed += 50
		} else {
			s.addPoints(rw, 75)
			s.appendHistory(rw, "75", "referral_reward_inviter", now, &wallet)
			stats.TotalPointsDistributed += 125
		}
		// 特定邀请码额外积分：奖励给通过该码设置推荐关系的被邀请人
		if bonus := referralBonusFor(code); bonus > 0 {
			s.addPoints(wallet, bonus)
			s.appendHistory(wallet, fmt.Sprintf("%d", bonus), "referral_bonus", now, &rw)
			stats.TotalPointsDistributed += bonus
		}
		dbSetJson("user:"+wallet, user)
		s.saveStats(stats)
		referralAwarded = true
	}
	s.recordIp(wallet, ip)
	return true, warning, referralAwarded, nil
}

// ── 自定义邀请链接（别名）────────────────────────────────

var reservedAliases = map[string]bool{
	"admin": true, "support": true, "official": true, "team": true,
	"system": true, "test": true, "invite": true, "join": true,
}

// validAlias 校验自定义邀请别名，返回错误信息（空串表示合法）。
func validAlias(a string) string {
	if len(a) < 4 || len(a) > 32 {
		return "alias must be 4-32 characters"
	}
	if strings.HasPrefix(a, "ref_") {
		return "alias cannot start with ref_"
	}
	for _, r := range a {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return "alias can only contain lowercase letters, digits, - and _"
		}
	}
	if reservedAliases[a] {
		return "alias is reserved"
	}
	return ""
}

// UpdateReferralAlias 设置用户的自定义邀请别名。
// 旧自动码（ref_xxx）保留映射，老链接继续有效；推荐记录迁移到新别名下。
func (s *EarlyAccessStore) UpdateReferralAlias(wallet, alias string) (bool, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet, alias = strings.ToLower(wallet), strings.ToLower(alias)
	user, _ := dbGetJson[DbUser]("user:" + wallet)
	if user == nil {
		return false, "", fmt.Errorf("user not joined")
	}
	if msg := validAlias(alias); msg != "" {
		return false, "", fmt.Errorf("%s", msg)
	}
	if owner, _ := s.loadCodeToWallet(alias); owner != "" && owner != wallet {
		return false, "", fmt.Errorf("alias already taken")
	}

	oldCode := user.ReferralCode
	user.ReferralCode = alias
	user.Alias = alias
	if err := dbSetJson("user:"+wallet, user); err != nil {
		return false, "", err
	}
	if err := s.saveCodeToWallet(alias, wallet); err != nil {
		return false, "", err
	}

	// 迁移推荐记录到新别名，旧 code 映射保留（旧链接继续可用）
	if oldCode != "" && oldCode != alias {
		if refs, _ := s.loadReferrals(oldCode); len(refs) > 0 {
			dbSetJson("referrals:"+alias, &refs)
			pebbleDB.Delete(eaKey("referrals:"+oldCode), pebble.Sync)
		}
	}
	return true, alias, nil
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

func (s *EarlyAccessStore) GetReferralInfo(wallet string) (code, alias, referrer string, earnings int64, pending int, referrals []string) {
	u, _ := dbGetJson[DbUser]("user:" + strings.ToLower(wallet))
	if u != nil {
		code, alias, referrer = u.ReferralCode, u.Alias, u.Referrer
	}
	refs, _ := s.loadReferrals(code)
	if len(refs) > 0 {
		for _, r := range refs {
			referrals = append(referrals, r.Wallet)
			if r.Settled {
				earnings += 75
			} else {
				pending++
			}
		}
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

func (s *EarlyAccessStore) loadReferrals(code string) ([]DbReferral, error) {
	v, closer, err := pebbleDB.Get(eaKey("referrals:" + code))
	if err != nil {
		if err == pebble.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	defer closer.Close()
	val := make([]byte, len(v))
	copy(val, v)

	var refs []DbReferral
	if json.Unmarshal(val, &refs) == nil {
		return refs, nil
	}
	// 兼容旧数据：历史上存的是 []string（加入即结算），迁移为 Settled=true
	var legacy []string
	if json.Unmarshal(val, &legacy) == nil {
		refs = make([]DbReferral, 0, len(legacy))
		for _, w := range legacy {
			refs = append(refs, DbReferral{Wallet: w, Settled: true})
		}
		dbSetJson("referrals:"+code, &refs)
		return refs, nil
	}
	return nil, nil
}

func (s *EarlyAccessStore) appendReferral(code, wallet, ts string, settled bool) error {
	refs, _ := s.loadReferrals(code)
	if refs == nil {
		refs = []DbReferral{}
	}
	refs = append(refs, DbReferral{Wallet: wallet, Settled: settled, CreatedAt: ts})
	return dbSetJson("referrals:"+code, &refs)
}

// ── 防女巫：配置 / IP / 延迟结算 ────────────────────────

const configKey = "config"

// adminPasswordFile 管理密码配置文件：敏感操作（如修改防女巫开关）需传入该密码。
// 密码放在 flux_config.json 的 admin_password 字段中，运维直接编辑该文件即可。
var adminPasswordFile = "flux_config.json"

// loadAdminPassword 从配置文件读取管理密码。文件不存在或解析失败时返回空串。
func loadAdminPassword() string {
	b, err := os.ReadFile(adminPasswordFile)
	if err != nil {
		return ""
	}
	var cfg FluxConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return ""
	}
	return cfg.AdminPassword
}

// checkAdminPassword 校验密码（常量时间比较，避免时序侧信道）。
func checkAdminPassword(pw string) bool {
	want := loadAdminPassword()
	if want == "" || pw == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(pw), []byte(want)) == 1
}

// loadAntiSybilConfig 读取防女巫配置，未配置时默认全部开启。
func loadAntiSybilConfig() DbConfig {
	v, _ := dbGetJson[DbConfig](configKey)
	if v == nil {
		return DbConfig{IpCheckEnabled: true, DeferredSettlementEnabled: true}
	}
	return *v
}

func saveAntiSybilConfig(c DbConfig) error {
	return dbSetJson(configKey, &c)
}

// GetAntiSybilConfig 返回当前防女巫配置。
func (s *EarlyAccessStore) GetAntiSybilConfig() DbConfig {
	return loadAntiSybilConfig()
}

// UpdateAntiSybilConfig 更新防女巫开关。敏感操作：必须传入配置文件（flux_config.json 的 admin_password）中的密码。
func (s *EarlyAccessStore) UpdateAntiSybilConfig(wallet, password string, ipCheck, deferred bool) (DbConfig, string) {
	if !checkAdminPassword(password) {
		cfg := loadAntiSybilConfig()
		if loadAdminPassword() == "" {
			return cfg, "admin password not configured"
		}
		return cfg, "invalid password"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	wallet = strings.ToLower(wallet)
	cfg := loadAntiSybilConfig()
	cfg.IpCheckEnabled = ipCheck
	cfg.DeferredSettlementEnabled = deferred
	cfg.UpdatedBy = wallet
	cfg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := saveAntiSybilConfig(cfg); err != nil {
		return loadAntiSybilConfig(), err.Error()
	}
	return cfg, ""
}

// ── 邀请码额外积分（推荐码加成）───────────────────────────

const refBonusKey = "refbonus"

// DbRefBonus 特定邀请码的额外积分配置：通过该邀请码加入的被邀请人额外获得 BonusPoints 积分。
type DbRefBonus struct {
	Code        string `json:"code"`
	BonusPoints int64  `json:"bonusPoints"`
	UpdatedBy   string `json:"updatedBy"`
	UpdatedAt   string `json:"updatedAt"`
}

func loadRefBonusConfig() map[string]DbRefBonus {
	v, _ := dbGetJson[map[string]DbRefBonus](refBonusKey)
	if v == nil {
		return map[string]DbRefBonus{}
	}
	return *v
}

func saveRefBonusConfig(m map[string]DbRefBonus) error {
	return dbSetJson(refBonusKey, &m)
}

// GetReferralBonusConfigs 返回全部邀请码额外积分配置（按 code 排序）。
func (s *EarlyAccessStore) GetReferralBonusConfigs() []DbRefBonus {
	m := loadRefBonusConfig()
	out := make([]DbRefBonus, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// SetReferralBonus 设置/移除特定邀请码的额外积分。敏感操作：必须传入配置文件中的管理密码。
// bonusPoints <= 0 时移除该邀请码的配置。
func (s *EarlyAccessStore) SetReferralBonus(wallet, password, code string, bonusPoints int64) ([]DbRefBonus, string) {
	if !checkAdminPassword(password) {
		if loadAdminPassword() == "" {
			return s.GetReferralBonusConfigs(), "admin password not configured"
		}
		return s.GetReferralBonusConfigs(), "invalid password"
	}

	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return s.GetReferralBonusConfigs(), "code required"
	}
	if len(code) > 64 {
		return s.GetReferralBonusConfigs(), "code too long"
	}
	for _, r := range code {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return s.GetReferralBonusConfigs(), "code can only contain lowercase letters, digits, - and _"
		}
	}
	if bonusPoints < 0 {
		return s.GetReferralBonusConfigs(), "bonus points cannot be negative"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := loadRefBonusConfig()
	if bonusPoints == 0 {
		delete(cfg, code)
	} else {
		cfg[code] = DbRefBonus{
			Code: code, BonusPoints: bonusPoints,
			UpdatedBy: strings.ToLower(wallet), UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}
	if err := saveRefBonusConfig(cfg); err != nil {
		return s.GetReferralBonusConfigs(), err.Error()
	}
	return s.GetReferralBonusConfigs(), ""
}

// referralBonusFor 查询邀请码的额外积分（无配置返回 0）。
func referralBonusFor(code string) int64 {
	if v, ok := loadRefBonusConfig()[strings.ToLower(code)]; ok {
		return v.BonusPoints
	}
	return 0
}

// recordIp 记录用户的 IP（首次 + 最近 5 个）与 IP 反查索引。调用方需持有锁。
// IP 未变化时跳过写库，避免每次查询都产生 pebble 写入。
func (s *EarlyAccessStore) recordIp(wallet, ip string) {
	ip = normalizeIP(ip)
	if ip == "" || wallet == "" {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	rec, _ := dbGetJson[DbIpRecord]("ip:" + wallet)
	if rec == nil {
		// 新用户：首次 IP 直接落库
		rec = &DbIpRecord{FirstIP: ip, FirstSeen: now, LastSeen: now, LastIPs: []string{}}
		dbSetJson("ip:"+wallet, rec)
		s.indexIp(wallet, ip)
		return
	}
	last := ""
	if len(rec.LastIPs) > 0 {
		last = rec.LastIPs[len(rec.LastIPs)-1]
	}
	if last == ip || (len(rec.LastIPs) == 0 && rec.FirstIP == ip) {
		return // IP 未变化，无需写库
	}
	for _, old := range rec.LastIPs {
		if old == ip {
			return // 历史出现过，不重复追加
		}
	}
	rec.LastSeen = now
	rec.LastIPs = append(rec.LastIPs, ip)
	if len(rec.LastIPs) > 5 {
		rec.LastIPs = rec.LastIPs[len(rec.LastIPs)-5:]
	}
	dbSetJson("ip:"+wallet, rec)
	s.indexIp(wallet, ip)
}

// indexIp 维护 IP 反查索引（批量注册检测）。调用方需持有锁。
func (s *EarlyAccessStore) indexIp(wallet, ip string) {
	users, _ := dbGetJson[[]string]("ipidx:" + ip)
	if users == nil {
		users = &[]string{}
	}
	found := false
	for _, w := range *users {
		if w == wallet {
			found = true
			break
		}
	}
	if !found {
		*users = append(*users, wallet)
		if len(*users) > 100 {
			*users = (*users)[len(*users)-100:]
		}
		dbSetJson("ipidx:"+ip, users)
	}
}

// RecordClientIp 由 resolver 调用：用户刷新页面/查询时更新其 IP 记录，保持最近 IP 新鲜，
// 使同 IP 邀请检查始终基于邀请人当前实际使用的 IP。仅记录已注册用户；内部加锁，可安全调用。
func (s *EarlyAccessStore) RecordClientIp(wallet, ip string) {
	wallet = strings.ToLower(wallet)
	if wallet == "" || ip == "" {
		return
	}
	if u, _ := dbGetJson[DbUser]("user:" + wallet); u == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordIp(wallet, ip)
}

// normalizeIP 标准化 IP 表示（IPv6 小写、IPv4 去除前导零等），无法解析时原样返回。
func normalizeIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if p := net.ParseIP(ip); p != nil {
		return p.String()
	}
	return ip
}

// ipMatches 判断 ip 是否与该用户记录过的任一 IP 相同。
func (s *EarlyAccessStore) ipMatches(ip, wallet string) bool {
	ip = normalizeIP(ip)
	rec, _ := dbGetJson[DbIpRecord]("ip:" + wallet)
	if rec == nil {
		return false
	}
	if rec.FirstIP == ip {
		return true
	}
	for _, x := range rec.LastIPs {
		if x == ip {
			return true
		}
	}
	return false
}

func (s *EarlyAccessStore) loadPendingRef(wallet string) (*DbPendingRef, error) {
	return dbGetJson[DbPendingRef]("pendingref:" + wallet)
}

func (s *EarlyAccessStore) savePendingRef(wallet, inviter, code, ts string) error {
	return dbSetJson("pendingref:"+wallet, &DbPendingRef{Inviter: inviter, Code: code, CreatedAt: ts})
}

// settlePending 结算被邀请人的待发推荐奖励：邀请人 +75 分。
// 由签到/答题成功后触发。调用方需持有锁。
func (s *EarlyAccessStore) settlePending(wallet string) {
	p, _ := s.loadPendingRef(wallet)
	if p == nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)

	// 标记推荐关系已结算（邀请人改过别名时，推荐记录已迁移到其当前 code 下）
	codes := []string{p.Code}
	if inv := s.GetUser(p.Inviter); inv != nil && inv.ReferralCode != "" && inv.ReferralCode != p.Code {
		codes = append(codes, inv.ReferralCode)
	}
	settled := false // 记录已处于 Settled（无论本次标记还是之前已标记）
	granted := false // 本次实际发生 false→true 转换（真正发放了积分）
	for _, c := range codes {
		refs, _ := s.loadReferrals(c)
		if refs == nil {
			continue
		}
		for i := range refs {
			if refs[i].Wallet == wallet {
				if !refs[i].Settled {
					refs[i].Settled = true
					dbSetJson("referrals:"+c, &refs)
					granted = true
				}
				settled = true
				break
			}
		}
		if settled {
			break
		}
	}
	if !settled {
		// 推荐记录缺失（异常）：本次不发分，等待下次签到/答题再触发，避免重复发分
		return
	}

	// 发放邀请人积分（仅首次结算：崩溃/重放不会重复发放）
	if granted {
		s.addPoints(p.Inviter, 75)
		s.appendHistory(p.Inviter, "75", "referral_reward_inviter", now, &wallet)
		stats, _ := s.loadStats()
		stats.TotalPointsDistributed += 75
		s.saveStats(stats)
	}

	// 清理 pending 记录
	pebbleDB.Delete(eaKey("pendingref:"+wallet), pebble.Sync)
	fmt.Printf("[anti-sybil] settled pending referral: invitee=%s inviter=%s\n", wallet, p.Inviter)
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

// DbReferrals 兼容新旧格式的推荐列表：新版 []DbReferral，旧版 []string。
type DbReferrals []DbReferral

func (r *DbReferrals) UnmarshalJSON(b []byte) error {
	var refs []DbReferral
	if err := json.Unmarshal(b, &refs); err == nil {
		*r = refs
		return nil
	}
	var legacy []string
	if err := json.Unmarshal(b, &legacy); err != nil {
		return err
	}
	refs = make([]DbReferral, 0, len(legacy))
	for _, w := range legacy {
		refs = append(refs, DbReferral{Wallet: w, Settled: true})
	}
	*r = refs
	return nil
}

type ExportedData struct {
	Users       map[string]DbUser            `json:"users"`
	Points      map[string]int64             `json:"points"`
	History     map[string][]DbHistory       `json:"history"`
	Tasks       map[string]map[string]string `json:"tasks"`
	Codes       map[string]string            `json:"codes"`
	Emails      map[string]string            `json:"emails"`
	Referrals   map[string]DbReferrals       `json:"referrals"`
	PendingRefs map[string]DbPendingRef      `json:"pendingRefs,omitempty"`
	Stats       *DbStats                     `json:"stats"`
	Config      *DbConfig                    `json:"config,omitempty"`
	RefBonuses  map[string]DbRefBonus        `json:"refBonuses,omitempty"`
	IpRecords   map[string]DbIpRecord        `json:"ipRecords,omitempty"`
	IpIndex     map[string][]string          `json:"ipIndex,omitempty"`
}

func (s *EarlyAccessStore) Export() string {
	d := ExportedData{
		Users: make(map[string]DbUser), Points: make(map[string]int64),
		History: make(map[string][]DbHistory), Tasks: make(map[string]map[string]string),
		Codes: make(map[string]string), Emails: make(map[string]string),
		Referrals: make(map[string]DbReferrals), PendingRefs: make(map[string]DbPendingRef),
		IpRecords: make(map[string]DbIpRecord), IpIndex: make(map[string][]string),
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
			var refs DbReferrals
			if json.Unmarshal(val, &refs) == nil {
				d.Referrals[key[10:]] = refs
			}
		case strings.HasPrefix(key, "pendingref:"):
			var v DbPendingRef
			if json.Unmarshal(val, &v) == nil {
				d.PendingRefs[key[11:]] = v
			}
		case key == "stats":
			var v DbStats
			if json.Unmarshal(val, &v) == nil {
				d.Stats = &v
			}
		case key == "config":
			var v DbConfig
			if json.Unmarshal(val, &v) == nil {
				d.Config = &v
			}
		case key == refBonusKey:
			var v map[string]DbRefBonus
			if json.Unmarshal(val, &v) == nil {
				d.RefBonuses = v
			}
		case strings.HasPrefix(key, "ip:"):
			var v DbIpRecord
			if json.Unmarshal(val, &v) == nil {
				d.IpRecords[key[3:]] = v
			}
		case strings.HasPrefix(key, "ipidx:"):
			var v []string
			if json.Unmarshal(val, &v) == nil {
				d.IpIndex[key[6:]] = v
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
		for k, refs := range d.Referrals {
			if err := dbSetJson("referrals:"+k, &refs); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.PendingRefs {
			if err := dbSetJson("pendingref:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		if d.Config != nil {
			if err := dbSetJson("config", d.Config); err != nil {
				errResult = err
				return
			}
		}
		if d.RefBonuses != nil {
			if err := dbSetJson(refBonusKey, &d.RefBonuses); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.IpRecords {
			if err := dbSetJson("ip:"+k, &v); err != nil {
				errResult = err
				return
			}
		}
		for k, v := range d.IpIndex {
			if err := dbSetJson("ipidx:"+k, &v); err != nil {
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
