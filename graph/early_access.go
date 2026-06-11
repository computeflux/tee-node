package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/store"
)

const eaNamespace = "EARLY_POINTS"

// EarlyAccessStore 基于 Pebble 的持久化存储。
type EarlyAccessStore struct {
	mu sync.Mutex // 保护 join/claimTask 的读-检查-写
}

var earlyAccessStore = &EarlyAccessStore{}

// ── DB types (JSON serializable) ─────────────────────────

type dbEarlyAccessUser struct {
	Wallet       string `json:"wallet"`
	JoinedAt     string `json:"joinedAt"`
	Referrer     string `json:"referrer"`
	ReferralCode string `json:"referralCode"`
	Email        string `json:"email"`
}

type dbPointsHistoryRecord struct {
	Amount      string  `json:"amount"`
	Reason      string  `json:"reason"`
	Timestamp   string  `json:"timestamp"`
	RelatedUser *string `json:"relatedUser,omitempty"`
}

type dbEarlyAccessStats struct {
	UserCount              int   `json:"userCount"`
	TotalPointsDistributed int64 `json:"totalPointsDistributed"`
	TotalReferrals         int   `json:"totalReferrals"`
}

var taskPoints = map[string]int64{
	"JOIN_EARLY_ACCESS": 100,
	"VERIFY_EMAIL":      50,
	"CONNECT_WALLET":    50,
	"JOIN_DISCORD":      100,
	"JOIN_TELEGRAM":     100,
	"INVITE_FRIEND":     200,
}

var allTaskTypes = []string{
	"JOIN_EARLY_ACCESS",
	"VERIFY_EMAIL",
	"CONNECT_WALLET",
	"JOIN_DISCORD",
	"JOIN_TELEGRAM",
	"INVITE_FRIEND",
}

// 生成固定推荐码：ref_ + 地址后8位小写
func generateReferralCode(wallet string) string {
	addr := strings.ToLower(wallet)
	if strings.HasPrefix(addr, "0x") && len(addr) >= 10 {
		return "ref_" + addr[len(addr)-8:]
	}
	if len(addr) >= 8 {
		return "ref_" + addr[len(addr)-8:]
	}
	h := sha256.Sum256([]byte(wallet))
	return "ref_" + hex.EncodeToString(h[:])[:8]
}

func (s *EarlyAccessStore) join(wallet, email string, referralCode *string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet = strings.ToLower(wallet)

	// 检查用户是否已存在
	existing, err := store.GetJson[dbEarlyAccessUser](eaNamespace, "user:"+wallet)
	if err != nil {
		return false, err
	}
	if existing != nil {
		return false, nil
	}

	// 检查邮箱是否已被其他钱包使用
	email = strings.ToLower(email)
	if email != "" {
		existingWallet, err := s.loadEmailToWallet(email)
		if err != nil {
			return false, err
		}
		if existingWallet != "" && existingWallet != wallet {
			return false, fmt.Errorf("email already registered by another wallet")
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	code := generateReferralCode(wallet)

	user := &dbEarlyAccessUser{
		Wallet:       wallet,
		JoinedAt:     now,
		ReferralCode: code,
		Email:        strings.ToLower(email),
	}

	// 初始积分 100
	initialPoints := int64(100)
	if err := store.SetJson(eaNamespace, "points:"+wallet, &initialPoints); err != nil {
		return false, err
	}
	if err := s.appendHistory(wallet, "100", "join_early_access", now, nil); err != nil {
		return false, err
	}

	// 更新统计
	stats, err := s.loadStats()
	if err != nil {
		return false, err
	}
	stats.UserCount++
	stats.TotalPointsDistributed += initialPoints

	// 处理推荐关系
	if referralCode != nil && *referralCode != "" {
		refCode := strings.ToLower(*referralCode)
		referrerWallet, err := s.loadCodeToWallet(refCode)
		if err != nil {
			return false, err
		}
		if referrerWallet != "" && referrerWallet != wallet {
			user.Referrer = referrerWallet

			// 更新推荐人列表
			if err := s.appendReferral(refCode, wallet); err != nil {
				return false, err
			}

			// 被推荐人额外 50
			bonusInvitee := int64(50)
			if err := s.addPoints(wallet, bonusInvitee); err != nil {
				return false, err
			}
			if err := s.appendHistory(wallet, "50", "referral_reward_invitee", now, &referrerWallet); err != nil {
				return false, err
			}
			stats.TotalPointsDistributed += bonusInvitee

			// 邀请人 200
			bonusInviter := int64(200)
			if err := s.addPoints(referrerWallet, bonusInviter); err != nil {
				return false, err
			}
			if err := s.appendHistory(referrerWallet, "200", "referral_reward_inviter", now, &wallet); err != nil {
				return false, err
			}
			stats.TotalPointsDistributed += bonusInviter

			stats.TotalReferrals++
		}
	}

	// 写入用户、码映射、邮箱映射、统计
	if err := store.SetJson(eaNamespace, "user:"+wallet, user); err != nil {
		return false, err
	}
	if err := s.saveCodeToWallet(code, wallet); err != nil {
		return false, err
	}
	if email != "" {
		if err := s.saveEmailToWallet(email, wallet); err != nil {
			return false, err
		}
	}
	if err := s.saveStats(stats); err != nil {
		return false, err
	}

	// 自动标记 JOIN_EARLY_ACCESS 任务完成
	tasks := map[string]string{"JOIN_EARLY_ACCESS": now}
	if err := store.SetJson(eaNamespace, "tasks:"+wallet, &tasks); err != nil {
		return false, err
	}

	return true, nil
}

func (s *EarlyAccessStore) claimTask(wallet, task string) (bool, int64, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet = strings.ToLower(wallet)

	// 用户必须先加入 Early Access
	user, err := store.GetJson[dbEarlyAccessUser](eaNamespace, "user:"+wallet)
	if err != nil {
		return false, 0, 0, err
	}
	if user == nil {
		return false, 0, 0, fmt.Errorf("user not joined early access")
	}

	points, ok := taskPoints[task]
	if !ok {
		return false, 0, 0, fmt.Errorf("invalid task type: %s", task)
	}

	tasks, err := s.loadTasks(wallet)
	if err != nil {
		return false, 0, 0, err
	}
	if tasks == nil {
		tasks = make(map[string]string)
	}

	if _, completed := tasks[task]; completed {
		total, err := s.loadPoints(wallet)
		return false, 0, total, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tasks[task] = now

	if err := s.addPoints(wallet, points); err != nil {
		return false, 0, 0, err
	}
	if err := s.appendHistory(wallet, fmt.Sprintf("%d", points), strings.ToLower(task), now, nil); err != nil {
		return false, 0, 0, err
	}

	stats, err := s.loadStats()
	if err != nil {
		return false, 0, 0, err
	}
	stats.TotalPointsDistributed += points
	if err := s.saveStats(stats); err != nil {
		return false, 0, 0, err
	}

	if err := s.saveTasks(wallet, tasks); err != nil {
		return false, 0, 0, err
	}

	total, err := s.loadPoints(wallet)
	return true, points, total, err
}

func (s *EarlyAccessStore) updateReferralCode(wallet, referralCode string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet = strings.ToLower(wallet)
	refCode := strings.ToLower(referralCode)

	// 检查用户是否存在
	user, err := store.GetJson[dbEarlyAccessUser](eaNamespace, "user:"+wallet)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, fmt.Errorf("user not joined early access")
	}

	// 已存在推荐人则不允许修改
	if user.Referrer != "" {
		return false, fmt.Errorf("referral code already set")
	}

	// 不能填写自己的推荐码
	if user.ReferralCode == refCode {
		return false, fmt.Errorf("cannot use your own referral code")
	}

	// 查找推荐码对应的钱包
	referrerWallet, err := s.loadCodeToWallet(refCode)
	if err != nil {
		return false, err
	}
	if referrerWallet == "" {
		return false, fmt.Errorf("invalid referral code")
	}
	if referrerWallet == wallet {
		return false, fmt.Errorf("cannot refer yourself")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	user.Referrer = referrerWallet

	// 更新推荐人列表
	if err := s.appendReferral(refCode, wallet); err != nil {
		return false, err
	}

	// 被推荐人额外 50
	bonusInvitee := int64(50)
	if err := s.addPoints(wallet, bonusInvitee); err != nil {
		return false, err
	}
	if err := s.appendHistory(wallet, "50", "referral_reward_invitee", now, &referrerWallet); err != nil {
		return false, err
	}

	// 邀请人 200
	bonusInviter := int64(200)
	if err := s.addPoints(referrerWallet, bonusInviter); err != nil {
		return false, err
	}
	if err := s.appendHistory(referrerWallet, "200", "referral_reward_inviter", now, &wallet); err != nil {
		return false, err
	}

	// 更新统计
	stats, err := s.loadStats()
	if err != nil {
		return false, err
	}
	stats.TotalPointsDistributed += bonusInvitee + bonusInviter
	stats.TotalReferrals++

	// 保存用户和统计
	if err := store.SetJson(eaNamespace, "user:"+wallet, user); err != nil {
		return false, err
	}
	if err := s.saveStats(stats); err != nil {
		return false, err
	}

	return true, nil
}

func (s *EarlyAccessStore) getTasks(wallet string) []*taskStatusRecord {
	wallet = strings.ToLower(wallet)
	tasks, _ := s.loadTasks(wallet)

	var result []*taskStatusRecord
	for _, task := range allTaskTypes {
		status := &taskStatusRecord{
			task:      task,
			completed: false,
		}
		if tasks != nil {
			if claimedAt, ok := tasks[task]; ok {
				status.completed = true
				status.claimedAt = &claimedAt
			}
		}
		result = append(result, status)
	}
	return result
}

// ── Helpers ──────────────────────────────────────────────

func (s *EarlyAccessStore) loadPoints(wallet string) (int64, error) {
	v, err := store.GetJson[int64](eaNamespace, "points:"+wallet)
	if err != nil {
		return 0, err
	}
	if v == nil {
		return 0, nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) addPoints(wallet string, delta int64) error {
	cur, err := s.loadPoints(wallet)
	if err != nil {
		return err
	}
	next := cur + delta
	return store.SetJson(eaNamespace, "points:"+wallet, &next)
}

func (s *EarlyAccessStore) appendHistory(wallet, amount, reason, timestamp string, relatedUser *string) error {
	key := "history:" + wallet
	list, err := store.GetJson[[]dbPointsHistoryRecord](eaNamespace, key)
	if err != nil {
		return err
	}
	var records []dbPointsHistoryRecord
	if list != nil {
		records = *list
	}
	records = append(records, dbPointsHistoryRecord{
		Amount:      amount,
		Reason:      reason,
		Timestamp:   timestamp,
		RelatedUser: relatedUser,
	})
	return store.SetJson(eaNamespace, key, &records)
}

func (s *EarlyAccessStore) loadStats() (dbEarlyAccessStats, error) {
	v, err := store.GetJson[dbEarlyAccessStats](eaNamespace, "stats")
	if err != nil {
		return dbEarlyAccessStats{}, err
	}
	if v == nil {
		return dbEarlyAccessStats{}, nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) saveStats(stats dbEarlyAccessStats) error {
	return store.SetJson(eaNamespace, "stats", &stats)
}

func (s *EarlyAccessStore) loadCodeToWallet(code string) (string, error) {
	v, err := store.GetJson[string](eaNamespace, "code:"+code)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) saveCodeToWallet(code, wallet string) error {
	return store.SetJson(eaNamespace, "code:"+code, &wallet)
}

func (s *EarlyAccessStore) loadEmailToWallet(email string) (string, error) {
	v, err := store.GetJson[string](eaNamespace, "email:"+email)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) saveEmailToWallet(email, wallet string) error {
	return store.SetJson(eaNamespace, "email:"+email, &wallet)
}

func (s *EarlyAccessStore) loadReferrals(code string) ([]string, error) {
	v, err := store.GetJson[[]string](eaNamespace, "referrals:"+code)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) appendReferral(code, wallet string) error {
	list, err := s.loadReferrals(code)
	if err != nil {
		return err
	}
	list = append(list, wallet)
	return store.SetJson(eaNamespace, "referrals:"+code, &list)
}

func (s *EarlyAccessStore) loadTasks(wallet string) (map[string]string, error) {
	v, err := store.GetJson[map[string]string](eaNamespace, "tasks:"+wallet)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return *v, nil
}

func (s *EarlyAccessStore) saveTasks(wallet string, tasks map[string]string) error {
	return store.SetJson(eaNamespace, "tasks:"+wallet, &tasks)
}

// ── Read-only accessors ──────────────────────────────────

func (s *EarlyAccessStore) getUserByReferralCode(code string) *earlyAccessUserRecord {
	code = strings.ToLower(code)
	wallet, err := s.loadCodeToWallet(code)
	if err != nil || wallet == "" {
		return nil
	}
	return s.getUser(wallet)
}

func (s *EarlyAccessStore) getUserByEmail(email string) *earlyAccessUserRecord {
	email = strings.ToLower(email)
	wallet, err := s.loadEmailToWallet(email)
	if err != nil || wallet == "" {
		return nil
	}
	return s.getUser(wallet)
}

func (s *EarlyAccessStore) getUser(wallet string) *earlyAccessUserRecord {
	wallet = strings.ToLower(wallet)
	v, err := store.GetJson[dbEarlyAccessUser](eaNamespace, "user:"+wallet)
	if err != nil || v == nil {
		return nil
	}
	return &earlyAccessUserRecord{
		wallet:       v.Wallet,
		joinedAt:     v.JoinedAt,
		referrer:     v.Referrer,
		referralCode: v.ReferralCode,
		email:        v.Email,
	}
}

func (s *EarlyAccessStore) getPoints(wallet string) string {
	v, err := s.loadPoints(strings.ToLower(wallet))
	if err != nil {
		return "0"
	}
	return fmt.Sprintf("%d", v)
}

func (s *EarlyAccessStore) getReferralInfo(wallet string) (code, referrer string, earnings int64, referrals []string) {
	wallet = strings.ToLower(wallet)
	u, err := store.GetJson[dbEarlyAccessUser](eaNamespace, "user:"+wallet)
	if err == nil && u != nil {
		code = u.ReferralCode
		referrer = u.Referrer
	}
	refList, err := s.loadReferrals(code)
	if err == nil && len(refList) > 0 {
		referrals = make([]string, len(refList))
		copy(referrals, refList)
		for range refList {
			earnings += 200
		}
	}
	return
}

func (s *EarlyAccessStore) getPointsHistory(wallet string, offset, limit int) ([]*pointsHistoryRecord, int) {
	wallet = strings.ToLower(wallet)
	list, err := store.GetJson[[]dbPointsHistoryRecord](eaNamespace, "history:"+wallet)
	if err != nil || list == nil {
		return nil, 0
	}

	records := *list
	total := len(records)

	// 倒序
	items := make([]*pointsHistoryRecord, total)
	for i := 0; i < total; i++ {
		rec := records[total-1-i]
		item := &pointsHistoryRecord{
			amount:      rec.Amount,
			reason:      rec.Reason,
			timestamp:   rec.Timestamp,
			relatedUser: rec.RelatedUser,
		}
		if rec.RelatedUser != nil && *rec.RelatedUser != "" {
			if related := s.getUser(*rec.RelatedUser); related != nil && related.email != "" {
				item.relatedEmail = &related.email
			}
		}
		items[i] = item
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
	return items[offset:end], total
}

func (s *EarlyAccessStore) getStats() earlyAccessStatsRecord {
	stats, err := s.loadStats()
	if err != nil {
		return earlyAccessStatsRecord{}
	}
	return earlyAccessStatsRecord{
		userCount:              stats.UserCount,
		totalPointsDistributed: stats.TotalPointsDistributed,
		totalReferrals:         stats.TotalReferrals,
	}
}

// ── Internal types (kept for resolver compatibility) ─────

type earlyAccessUserRecord struct {
	wallet       string
	joinedAt     string
	referrer     string
	referralCode string
	email        string
}

type pointsHistoryRecord struct {
	amount       string
	reason       string
	timestamp    string
	relatedUser  *string
	relatedEmail *string
}

type earlyAccessStatsRecord struct {
	userCount              int
	totalPointsDistributed int64
	totalReferrals         int
}

type taskStatusRecord struct {
	task      string
	completed bool
	claimedAt *string
}
