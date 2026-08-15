package actives

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *EarlyAccessStore {
	dir := t.TempDir()
	db, err := pebble.Open(dir, &pebble.Options{})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	InitDB(db)

	// 把密码文件重定向到临时目录，避免测试读写到工作目录下真实的 flux_config.json（含私钥）
	old := adminPasswordFile
	adminPasswordFile = filepath.Join(dir, "flux_config.json")
	t.Cleanup(func() { adminPasswordFile = old })

	return &EarlyAccessStore{}
}

func joinWithCode(t *testing.T, s *EarlyAccessStore, wallet, ip string, code *string) (warning string, awarded bool) {
	ok, warning, awarded, err := s.Join(wallet, "", ip, code)
	require.NoError(t, err)
	require.True(t, ok)
	return warning, awarded
}

// ── IP 防女巫 ────────────────────────────────────────────

func TestJoin_SameIP_BlocksReferralReward(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	_, _ = joinWithCode(t, s, "0xbbbb", "1.2.3.4", nil)

	// 获取 A 的推荐码
	a := s.GetUser("0xaaaa")
	require.NotNil(t, a)

	ok, warning, awarded, err := s.Join("0xcccc", "", "1.2.3.4", &a.ReferralCode)
	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, awarded)
	require.Contains(t, warning, "Not eligible for referral reward")

	// A 不应收到任何推荐积分，B 也不应有被推荐记录
	assert.Equal(t, "100", s.GetPoints("0xaaaa"))
	assert.Equal(t, "100", s.GetPoints("0xcccc"))
	_, _, _, earnings, pending, referrals := s.GetReferralInfo("0xaaaa")
	assert.Equal(t, int64(0), earnings)
	assert.Equal(t, 0, pending)
	assert.Len(t, referrals, 0)
}

func TestJoin_SameIP_CheckDisabled_Allows(t *testing.T) {
	s := newTestStore(t)
	writeAdminPassword(t, "testpass")
	cfg, msg := s.UpdateAntiSybilConfig("0xadmin", "testpass", false, false)
	require.Empty(t, msg)
	assert.False(t, cfg.IpCheckEnabled)

	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")

	_, awarded := joinWithCode(t, s, "0xbbbb", "1.2.3.4", &a.ReferralCode)
	assert.True(t, awarded)
	assert.Equal(t, "175", s.GetPoints("0xaaaa"))
}

// ── 加入前预检查 ──────────────────────────────────────────

func TestCheckReferralEligibility_SameIP_Blocked(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")
	require.NotNil(t, a)

	// 同 IP → 不可获得奖励
	eligible, reason := s.CheckReferralEligibility("0xbbbb", "1.2.3.4", a.ReferralCode)
	assert.False(t, eligible)
	assert.Contains(t, reason, "Not eligible for referral reward")

	// 不同 IP → 可获得奖励
	eligible, reason = s.CheckReferralEligibility("0xbbbb", "9.9.9.9", a.ReferralCode)
	assert.True(t, eligible)
	assert.Empty(t, reason)
}

func TestCheckReferralEligibility_CheckDisabled_AllowsSameIP(t *testing.T) {
	s := newTestStore(t)
	writeAdminPassword(t, "testpass")
	_, msg := s.UpdateAntiSybilConfig("0xadmin", "testpass", false, false)
	require.Empty(t, msg)

	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")
	require.NotNil(t, a)

	// IP 检查关闭 → 同 IP 也可获得奖励
	eligible, reason := s.CheckReferralEligibility("0xbbbb", "1.2.3.4", a.ReferralCode)
	assert.True(t, eligible)
	assert.Empty(t, reason)
}

func TestCheckReferralEligibility_InvalidCodeAndAlreadyJoined(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")
	require.NotNil(t, a)

	// 无效邀请码
	eligible, reason := s.CheckReferralEligibility("0xbbbb", "9.9.9.9", "no_such_code")
	assert.False(t, eligible)
	assert.Equal(t, "invalid code", reason)

	// 自己的邀请码
	eligible, reason = s.CheckReferralEligibility("0xbbbb", "9.9.9.9", a.ReferralCode)
	assert.True(t, eligible)

	// 已加入用户 → 不可再加入
	eligible, reason = s.CheckReferralEligibility("0xaaaa", "9.9.9.9", a.ReferralCode)
	assert.False(t, eligible)
	assert.Equal(t, "already joined", reason)
}

func TestCheckReferralEligibility_EmptyWallet_OnlyIPCheck(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")
	require.NotNil(t, a)

	// wallet 为空：跳过已加入/自邀检查，只做同 IP 判断
	// 已加入用户 + 空 wallet → 不再报 already joined
	eligible, reason := s.CheckReferralEligibility("", "9.9.9.9", a.ReferralCode)
	assert.True(t, eligible)
	assert.Empty(t, reason)

	// 同 IP 仍被拦截
	eligible, reason = s.CheckReferralEligibility("", "1.2.3.4", a.ReferralCode)
	assert.False(t, eligible)
	assert.Contains(t, reason, "Not eligible for referral reward")

	// 无效邀请码仍被拦截
	eligible, reason = s.CheckReferralEligibility("", "9.9.9.9", "no_such_code")
	assert.False(t, eligible)
	assert.Equal(t, "invalid code", reason)
}

func TestCheckReferralEligibility_ReferralCodeUpdatesFreshIP(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")
	require.NotNil(t, a)

	// 邀请人刷新页面换了 IP，新 IP 也要能拦截同 IP 加入
	s.RecordClientIp("0xaaaa", "5.6.7.8")
	eligible, reason := s.CheckReferralEligibility("0xbbbb", "5.6.7.8", a.ReferralCode)
	assert.False(t, eligible)
	assert.Contains(t, reason, "Not eligible for referral reward")
}

func TestJoin_DifferentIP_Awards(t *testing.T) {
	s := newTestStore(t)
	writeAdminPassword(t, "testpass")
	_, msg := s.UpdateAntiSybilConfig("0xadmin", "testpass", true, false)
	require.Empty(t, msg)

	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")

	_, awarded := joinWithCode(t, s, "0xbbbb", "5.6.7.8", &a.ReferralCode)
	assert.True(t, awarded)
	assert.Equal(t, "175", s.GetPoints("0xaaaa"))
	assert.Equal(t, "150", s.GetPoints("0xbbbb"))
}

// ── 延迟结算 ─────────────────────────────────────────────

func TestJoin_DeferredSettlement_InviterPaidAfterActivity(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")

	_, awarded := joinWithCode(t, s, "0xbbbb", "5.6.7.8", &a.ReferralCode)
	require.True(t, awarded)

	// 被邀请人拿到 50（立即可得），邀请人的 75 处于 pending
	assert.Equal(t, "100", s.GetPoints("0xaaaa"))
	assert.Equal(t, "150", s.GetPoints("0xbbbb"))
	_, _, _, earnings, pending, referrals := s.GetReferralInfo("0xaaaa")
	assert.Equal(t, int64(0), earnings)
	assert.Equal(t, 1, pending)
	assert.Len(t, referrals, 1)

	// 被邀请人完成一次签到 → 结算
	_, _, _, _, err := s.DailyCheckin("0xbbbb", "")
	require.NoError(t, err)
	assert.Equal(t, "175", s.GetPoints("0xaaaa"))
	_, _, _, earnings, pending, _ = s.GetReferralInfo("0xaaaa")
	assert.Equal(t, int64(75), earnings)
	assert.Equal(t, 0, pending)

	// 二次签到不重复结算
	_, _, _, _, err = s.DailyCheckin("0xbbbb", "")
	require.NoError(t, err)
	assert.Equal(t, "175", s.GetPoints("0xaaaa"))
}

func TestJoin_DeferredDisabled_ImmediateSettlement(t *testing.T) {
	s := newTestStore(t)
	writeAdminPassword(t, "testpass")
	_, msg := s.UpdateAntiSybilConfig("0xadmin", "testpass", true, false)
	require.Empty(t, msg)

	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")

	_, awarded := joinWithCode(t, s, "0xbbbb", "5.6.7.8", &a.ReferralCode)
	require.True(t, awarded)
	assert.Equal(t, "175", s.GetPoints("0xaaaa"))
	_, _, _, earnings, pending, _ := s.GetReferralInfo("0xaaaa")
	assert.Equal(t, int64(75), earnings)
	assert.Equal(t, 0, pending)
}

// ── 别名系统 ─────────────────────────────────────────────

func TestAlias_SetAndLookup(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")
	require.NotNil(t, a)

	ok, alias, err := s.UpdateReferralAlias("0xaaaa", "satoshi")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "satoshi", alias)

	// 通过别名可查回用户
	u := s.GetUserByReferralCode("satoshi")
	require.NotNil(t, u)
	assert.Equal(t, "0xaaaa", u.Wallet)

	// 旧自动码仍有效
	oldCode := a.ReferralCode
	assert.True(t, strings.HasPrefix(oldCode, "ref_"))
	u2 := s.GetUserByReferralCode(oldCode)
	require.NotNil(t, u2)
	assert.Equal(t, "0xaaaa", u2.Wallet)

	// GetReferralInfo 返回别名
	_, gotAlias, _, _, _, _ := s.GetReferralInfo("0xaaaa")
	assert.Equal(t, "satoshi", gotAlias)
}

func TestAlias_Validation(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)

	cases := []string{
		"abc",          // 太短
		"ref_12345678", // 保留前缀
		"AbC",          // 大写
		"has space",    // 非法字符
		"admin",        // 保留词
		"this_alias_is_way_too_long_for_32_chars_limit", // 太长
	}
	for _, c := range cases {
		ok, _, err := s.UpdateReferralAlias("0xaaaa", c)
		assert.False(t, ok, "alias %q should be rejected", c)
		assert.Error(t, err)
	}

	// 非法别名不改变当前 code
	u := s.GetUser("0xaaaa")
	assert.True(t, strings.HasPrefix(u.ReferralCode, "ref_"))
}

func TestAlias_Collision(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	joinWithCode(t, s, "0xbbbb", "5.6.7.8", nil)

	_, _, err := s.UpdateReferralAlias("0xaaaa", "takeme")
	require.NoError(t, err)

	ok, _, err := s.UpdateReferralAlias("0xbbbb", "takeme")
	assert.False(t, ok)
	assert.Error(t, err)
}

func TestAlias_ReferralsMigrateToNewCode(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")

	// B 用 A 的自动码加入
	joinWithCode(t, s, "0xbbbb", "5.6.7.8", &a.ReferralCode)

	// A 改别名后，推荐记录应迁移到新别名下
	_, _, err := s.UpdateReferralAlias("0xaaaa", "newalias")
	require.NoError(t, err)

	_, _, _, _, pending, referrals := s.GetReferralInfo("0xaaaa")
	assert.Equal(t, 1, pending)
	assert.Len(t, referrals, 1)

	// 通过新别名还能拿到积分（settle 路径）
	_, _, _, _, err = s.DailyCheckin("0xbbbb", "")
	require.NoError(t, err)
	_, _, _, earnings, pending, _ := s.GetReferralInfo("0xaaaa")
	assert.Equal(t, int64(75), earnings)
	assert.Equal(t, 0, pending)
}

func TestAlias_OldLinkStillUsable_AfterAliasChange(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")
	oldCode := a.ReferralCode
	require.True(t, strings.HasPrefix(oldCode, "ref_"))

	// A 设置自定义别名
	_, _, err := s.UpdateReferralAlias("0xaaaa", "satoshi")
	require.NoError(t, err)

	// 旧链接（自动码）仍能解析到 A
	u := s.GetUserByReferralCode(oldCode)
	require.NotNil(t, u)
	assert.Equal(t, "0xaaaa", u.Wallet)

	// C 通过老链接加入：推荐应记录在 A 当前 code（别名）下，列表可见
	_, awarded := joinWithCode(t, s, "0xcccc", "5.6.7.8", &oldCode)
	require.True(t, awarded)
	_, _, _, _, pending, referrals := s.GetReferralInfo("0xaaaa")
	assert.Equal(t, 1, pending)
	assert.Len(t, referrals, 1)
	assert.Equal(t, "0xcccc", referrals[0])

	// 结算正常
	_, _, _, _, err = s.DailyCheckin("0xcccc", "")
	require.NoError(t, err)
	_, _, _, earnings, pending, _ := s.GetReferralInfo("0xaaaa")
	assert.Equal(t, int64(75), earnings)
	assert.Equal(t, 0, pending)
}

func TestUpdateReferralCode_SameIP_Blocked(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	joinWithCode(t, s, "0xbbbb", "1.2.3.4", nil) // B 先加入（同 IP）
	a := s.GetUser("0xaaaa")

	// B 用 updateReferralCode 绑定 A 的码：同 IP → 拦截
	ok, warning, awarded, err := s.UpdateReferralCode("0xbbbb", "1.2.3.4", a.ReferralCode)
	require.NoError(t, err)
	require.True(t, ok)
	assert.False(t, awarded)
	assert.Contains(t, warning, "Not eligible for referral reward")

	// B 未被绑定，可稍后从其他 IP 重试
	u := s.GetUser("0xbbbb")
	assert.Equal(t, "", u.Referrer)

	// 不同 IP 重试 → 成功
	ok, warning, awarded, err = s.UpdateReferralCode("0xbbbb", "5.6.7.8", a.ReferralCode)
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, awarded)
	assert.Empty(t, warning)
	u = s.GetUser("0xbbbb")
	assert.Equal(t, "0xaaaa", u.Referrer)
}

func TestQuiz_CorrectAnswer_SettlesPendingReferral(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")
	_, awarded := joinWithCode(t, s, "0xbbbb", "5.6.7.8", &a.ReferralCode)
	require.True(t, awarded)

	// 先确认 A 的积分在 pending
	assert.Equal(t, "100", s.GetPoints("0xaaaa"))

	// 解锁 day3 答题需要连续签到 3 天
	today := time.Now().UTC().Truncate(24 * time.Hour)
	d1 := today.AddDate(0, 0, -2).Format("2006-01-02")
	d2 := today.AddDate(0, 0, -1).Format("2006-01-02")
	d3 := today.Format("2006-01-02")
	_, _, _, _, err := s.DailyCheckin("0xbbbb", d1)
	require.NoError(t, err)
	_, _, _, _, err = s.DailyCheckin("0xbbbb", d2)
	require.NoError(t, err)
	_, _, unlocked3, _, err := s.DailyCheckin("0xbbbb", d3)
	require.NoError(t, err)
	require.True(t, unlocked3)

	// 取一道 day3 题目并答对
	var qid string
	var q *QuizQuestionRecord
	for id, qq := range Day3QuestionsData {
		qid, q = id, qq
		break
	}
	correct, pts, _, err := s.SubmitQuizAnswer("0xbbbb", qid, q.CorrectAnswer)
	require.NoError(t, err)
	require.True(t, correct)
	assert.Equal(t, int64(day3QuizPoints), pts)

	// 答题正确 → 结算：邀请人拿到 75
	assert.Equal(t, "175", s.GetPoints("0xaaaa"))
	_, _, _, earnings, pending, _ := s.GetReferralInfo("0xaaaa")
	assert.Equal(t, int64(75), earnings)
	assert.Equal(t, 0, pending)
}

// ── 密码 / 配置 ──────────────────────────────────────────

func writeAdminPassword(t *testing.T, pw string) {
	t.Helper()
	require.NoError(t, os.WriteFile(adminPasswordFile, []byte(fmt.Sprintf(`{"admin_password":"%s"}`, pw)), 0644))
}

func TestConfig_PasswordRequired_ToChange(t *testing.T) {
	s := newTestStore(t)
	writeAdminPassword(t, "secret123")

	// 无密码 → 拒绝
	_, msg := s.UpdateAntiSybilConfig("0xuser1", "", false, true)
	assert.Equal(t, "invalid password", msg)

	// 密码错误 → 拒绝
	_, msg = s.UpdateAntiSybilConfig("0xuser1", "wrongpass", false, true)
	assert.Equal(t, "invalid password", msg)

	// 正确密码 → 成功
	cfg, msg := s.UpdateAntiSybilConfig("0xuser1", "secret123", false, true)
	require.Empty(t, msg)
	assert.False(t, cfg.IpCheckEnabled)
	assert.True(t, cfg.DeferredSettlementEnabled)
	assert.Equal(t, "0xuser1", cfg.UpdatedBy)
}

func TestConfig_NoPasswordFile_Rejected(t *testing.T) {
	s := newTestStore(t)

	// 未配置密码文件 → 敏感操作拒绝
	_, msg := s.UpdateAntiSybilConfig("0xuser1", "anything", false, true)
	assert.Equal(t, "admin password not configured", msg)
}

func TestConfig_PasswordChangeRequiresCurrent(t *testing.T) {
	s := newTestStore(t)
	writeAdminPassword(t, "old-pass")

	// 先用旧密码修改成功
	cfg, msg := s.UpdateAntiSybilConfig("0xuser1", "old-pass", true, false)
	require.Empty(t, msg)
	assert.True(t, cfg.IpCheckEnabled)
	assert.False(t, cfg.DeferredSettlementEnabled)

	// 运维直接改配置文件密码后，新密码生效（每次读文件，无需重启）
	writeAdminPassword(t, "new-pass")
	_, msg = s.UpdateAntiSybilConfig("0xuser1", "old-pass", true, true)
	assert.Equal(t, "invalid password", msg)
	cfg, msg = s.UpdateAntiSybilConfig("0xuser1", "new-pass", true, true)
	require.Empty(t, msg)
	assert.True(t, cfg.DeferredSettlementEnabled)
}

// ── IP 新鲜度（刷新页面更新 IP）────────────────────────────

func TestRecordClientIp_RefreshKeepsIpFresh(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")

	// A 换了网络（刷新页面触发）→ IP 记录更新
	s.RecordClientIp("0xaaaa", "9.9.9.9")

	// 女巫 B 从 A 当前 IP 加入 → 应被拦截
	ok, warning, awarded, err := s.Join("0xbbbb", "", "9.9.9.9", &a.ReferralCode)
	require.NoError(t, err)
	require.True(t, ok)
	assert.False(t, awarded)
	assert.Contains(t, warning, "Not eligible for referral reward")

	// 老 IP 仍在记录内 → 从老 IP 加入同样被拦截
	_, _, awarded2, err := s.Join("0xcccc", "", "1.2.3.4", &a.ReferralCode)
	require.NoError(t, err)
	require.False(t, awarded2)
}

func TestRecordClientIp_SameIp_NoDuplicateWrite(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)

	// 多次刷新同一 IP → 不追加重复
	s.RecordClientIp("0xaaaa", "1.2.3.4")
	s.RecordClientIp("0xaaaa", "1.2.3.4")
	rec, _ := dbGetJson[DbIpRecord]("ip:0xaaaa")
	require.NotNil(t, rec)
	assert.Equal(t, "1.2.3.4", rec.FirstIP)
	assert.Len(t, rec.LastIPs, 0)

	// 连续换 6 个 IP → 只保留最近 5 个
	for i := 0; i < 6; i++ {
		s.RecordClientIp("0xaaaa", fmt.Sprintf("10.0.0.%d", i))
	}
	rec, _ = dbGetJson[DbIpRecord]("ip:0xaaaa")
	require.NotNil(t, rec)
	assert.Equal(t, "1.2.3.4", rec.FirstIP)
	assert.Len(t, rec.LastIPs, 5)
	assert.Equal(t, "10.0.0.5", rec.LastIPs[len(rec.LastIPs)-1])
}

func TestRecordClientIp_UnregisteredWallet_Ignored(t *testing.T) {
	s := newTestStore(t)
	s.RecordClientIp("0xnobody", "1.2.3.4")
	rec, _ := dbGetJson[DbIpRecord]("ip:0xnobody")
	assert.Nil(t, rec)
}

// ── 旧数据迁移 ───────────────────────────────────────────

func TestLegacyReferrals_MigrateToSettled(t *testing.T) {
	s := newTestStore(t)

	// 模拟旧格式 []string 数据，迁移后全部视为已结算
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	u := s.GetUser("0xaaaa")
	legacy := []string{"0xold1", "0xold2"}
	require.NoError(t, dbSetJson("referrals:"+u.ReferralCode, &legacy))

	_, _, _, earnings, pending, referrals := s.GetReferralInfo("0xaaaa")
	assert.Equal(t, int64(150), earnings)
	assert.Equal(t, 0, pending)
	assert.Len(t, referrals, 2)

	// 数据已迁移为新格式（写回后 Settled=true）
	refs, err := s.loadReferrals(u.ReferralCode)
	require.NoError(t, err)
	assert.Len(t, refs, 2)
	assert.True(t, refs[0].Settled)
	assert.True(t, refs[1].Settled)
}

// ── 邀请码额外积分（推荐码加成）────────────────────────────

func TestReferralBonus_ConfigSetRemove(t *testing.T) {
	// 未配置密码文件 → 拒绝（先于设置密码的 store，避免全局文件重定向互相影响）
	s0 := newTestStore(t)
	_, msg := s0.SetReferralBonus("0xadmin", "anything", "vipcode", 300)
	assert.Equal(t, "admin password not configured", msg)

	s := newTestStore(t)
	writeAdminPassword(t, "secret123")

	// 密码错误/为空 → 拒绝
	_, msg = s.SetReferralBonus("0xadmin", "wrong", "vipcode", 300)
	assert.Equal(t, "invalid password", msg)
	_, msg = s.SetReferralBonus("0xadmin", "", "vipcode", 300)
	assert.Equal(t, "invalid password", msg)

	// 正确密码 → 设置成功
	list, msg := s.SetReferralBonus("0xadmin", "secret123", "VIPCODE", 300)
	require.Empty(t, msg)
	require.Len(t, list, 1)
	assert.Equal(t, "vipcode", list[0].Code) // 小写归一
	assert.Equal(t, int64(300), list[0].BonusPoints)
	assert.Equal(t, "0xadmin", list[0].UpdatedBy)

	// 非法 code → 拒绝
	_, msg = s.SetReferralBonus("0xadmin", "secret123", "BAD CODE!", 100)
	assert.NotEmpty(t, msg)

	// 负积分 → 拒绝
	_, msg = s.SetReferralBonus("0xadmin", "secret123", "vipcode", -5)
	assert.NotEmpty(t, msg)

	// 移除（bonus=0）→ 配置清空
	list, msg = s.SetReferralBonus("0xadmin", "secret123", "vipcode", 0)
	require.Empty(t, msg)
	assert.Empty(t, list)
}

func TestJoin_ReferralBonusApplied(t *testing.T) {
	s := newTestStore(t)
	writeAdminPassword(t, "testpass")
	_, msg := s.SetReferralBonus("0xadmin", "testpass", "vipcode", 300)
	require.Empty(t, msg)

	// A 注册并设置别名 vipcode
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	_, _, err := s.UpdateReferralAlias("0xaaaa", "vipcode")
	require.NoError(t, err)
	a := s.GetUser("0xaaaa")
	require.Equal(t, "vipcode", a.ReferralCode)

	// B 通过 vip 码从不同 IP 加入 → 获得 300 额外积分
	_, awarded := joinWithCode(t, s, "0xbbbb", "5.6.7.8", &a.ReferralCode)
	require.True(t, awarded)

	// B: 100 注册 + 50 被邀请 + 300 加成 = 450
	assert.Equal(t, "450", s.GetPoints("0xbbbb"))
	// A: 100 注册（75 邀请奖励延迟结算）
	assert.Equal(t, "100", s.GetPoints("0xaaaa"))

	// 历史记录包含加成条目
	items, _ := s.GetPointsHistory("0xbbbb", 0, 10)
	found := false
	for _, it := range items {
		if it.Reason == "referral_bonus" && it.Amount == "300" {
			found = true
		}
	}
	assert.True(t, found, "history should contain referral_bonus +300")
}

func TestJoin_ReferralBonus_BlockedBySameIP(t *testing.T) {
	s := newTestStore(t)
	writeAdminPassword(t, "testpass")
	_, msg := s.SetReferralBonus("0xadmin", "testpass", "vipcode", 300)
	require.Empty(t, msg)

	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	_, _, err := s.UpdateReferralAlias("0xaaaa", "vipcode")
	require.NoError(t, err)

	// 同 IP → 推荐关系被拦截，加成也不发放
	_, awarded := joinWithCode(t, s, "0xbbbb", "1.2.3.4", strPtr("vipcode"))
	require.False(t, awarded)
	assert.Equal(t, "100", s.GetPoints("0xbbbb"))
}

func TestUpdateReferralCode_ReferralBonusApplied(t *testing.T) {
	s := newTestStore(t)
	writeAdminPassword(t, "testpass")
	_, msg := s.SetReferralBonus("0xadmin", "testpass", "vipcode", 200)
	require.Empty(t, msg)

	// A 设置别名 vipcode
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	_, _, err := s.UpdateReferralAlias("0xaaaa", "vipcode")
	require.NoError(t, err)

	// B 先无码注册，后补填 vipcode 码
	joinWithCode(t, s, "0xbbbb", "5.6.7.8", nil)
	ok, _, awarded, err := s.UpdateReferralCode("0xbbbb", "5.6.7.8", "vipcode")
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, awarded)

	// B: 100 注册 + 50 被邀请 + 200 加成 = 350
	assert.Equal(t, "350", s.GetPoints("0xbbbb"))
}

// ── settlePending 幂等（防崩溃重放重复发分）────────────────

func TestSettlePending_Idempotent_NoDoubleAward(t *testing.T) {
	s := newTestStore(t)
	joinWithCode(t, s, "0xaaaa", "1.2.3.4", nil)
	a := s.GetUser("0xaaaa")

	// B 从不同 IP 加入（延迟结算默认开启）→ 邀请人的 75 分 pending
	_, awarded := joinWithCode(t, s, "0xbbbb", "5.6.7.8", &a.ReferralCode)
	require.True(t, awarded)
	assert.Equal(t, "100", s.GetPoints("0xaaaa"))

	// B 签到 → 触发结算，A 拿到 75
	_, _, _, _, err := s.DailyCheckin("0xbbbb", "")
	require.NoError(t, err)
	assert.Equal(t, "175", s.GetPoints("0xaaaa"))

	// 模拟崩溃重放：结算已成功但 pendingref 未删除
	require.NoError(t, s.savePendingRef("0xbbbb", a.Wallet, a.ReferralCode, time.Now().UTC().Format(time.RFC3339)))
	s.settlePending("0xbbbb")

	// 不应重复发放 → A 仍为 175
	assert.Equal(t, "175", s.GetPoints("0xaaaa"))

	// 历史记录里邀请人的 75 奖励只有一条
	items, _ := s.GetPointsHistory("0xaaaa", 0, 20)
	count := 0
	for _, it := range items {
		if it.Reason == "referral_reward_inviter" && it.Amount == "75" {
			count++
		}
	}
	assert.Equal(t, 1, count, "inviter 75 reward should be awarded exactly once")
}

// ── updateReferralAlias 签名验证（EIP-191）────────────────

const (
	testKeyA = "0x1111111111111111111111111111111111111111111111111111111111111111"
	testKeyB = "0x2222222222222222222222222222222222222222222222222222222222222222"
)

// signEIP191 用私钥对 message 做 personal_sign 风格的 EIP-191 签名，返回 hex。
func signEIP191(t *testing.T, privHex, message string) string {
	key, err := ethcrypto.HexToECDSA(strings.TrimPrefix(privHex, "0x"))
	require.NoError(t, err)
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := ethcrypto.Keccak256Hash([]byte(msg))
	sig, err := ethcrypto.Sign(hash.Bytes(), key)
	require.NoError(t, err)
	return hex.EncodeToString(sig)
}

func TestUpdateReferralAlias_RequiresOwnerSignature(t *testing.T) {
	s := newTestStore(t)
	keyA, err := ethcrypto.HexToECDSA(strings.TrimPrefix(testKeyA, "0x"))
	require.NoError(t, err)
	addrA := strings.ToLower(ethcrypto.PubkeyToAddress(keyA.PublicKey).Hex())

	joinWithCode(t, s, addrA, "1.2.3.4", nil)

	mr := &mutationResolver{&Resolver{}}
	ctx := context.Background()

	// 1. 无签名 → 拒绝
	res, err := mr.UpdateReferralAlias(ctx, addrA, "myalias", "")
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Equal(t, "invalid signature", *res.TxHash)

	// 2. 他人（B）的签名 → 拒绝
	sigB := signEIP191(t, testKeyB, "myalias")
	res, err = mr.UpdateReferralAlias(ctx, addrA, "myalias", sigB)
	require.NoError(t, err)
	assert.False(t, res.Success)

	// 3. 本人正确签名 → 成功
	sigA := signEIP191(t, testKeyA, "myalias")
	res, err = mr.UpdateReferralAlias(ctx, addrA, "myalias", sigA)
	require.NoError(t, err)
	assert.True(t, res.Success)
	u := s.GetUser(addrA)
	require.NotNil(t, u)
	assert.Equal(t, "myalias", u.ReferralCode)

	// 4. 签名绑定 alias：对别的 alias 的签名不能修改 myalias
	sigOther := signEIP191(t, testKeyA, "otheralias")
	res, err = mr.UpdateReferralAlias(ctx, addrA, "myalias", sigOther)
	require.NoError(t, err)
	assert.False(t, res.Success)

	// 5. 未注册用户 → 拒绝（即使签名正确）
	keyB, err := ethcrypto.HexToECDSA(strings.TrimPrefix(testKeyB, "0x"))
	require.NoError(t, err)
	addrB := strings.ToLower(ethcrypto.PubkeyToAddress(keyB.PublicKey).Hex())
	res, err = mr.UpdateReferralAlias(ctx, addrB, "myalias", signEIP191(t, testKeyB, "myalias"))
	require.NoError(t, err)
	assert.False(t, res.Success)
}

// ── 自动邀请码去重（碰撞时生成唯一码）───────────────────────

func TestJoin_ReferralCodeUnique_OnCollision(t *testing.T) {
	s := newTestStore(t)

	// 两个不同 wallet 后 8 位相同（"00000001"）→ 自动码碰撞
	joinWithCode(t, s, "aaa00000001", "1.2.3.4", nil)
	u1 := s.GetUser("aaa00000001")
	require.NotNil(t, u1)
	assert.Equal(t, "ref_00000001", u1.ReferralCode)

	joinWithCode(t, s, "bbb00000001", "5.6.7.8", nil)
	u2 := s.GetUser("bbb00000001")
	require.NotNil(t, u2)
	assert.NotEqual(t, u1.ReferralCode, u2.ReferralCode, "colliding codes must be deduplicated")
	assert.NotEqual(t, "ref_00000001", u2.ReferralCode)

	// 两个码都能正确反查回各自的钱包（老用户映射未被覆盖）
	w1, _ := s.loadCodeToWallet(u1.ReferralCode)
	w2, _ := s.loadCodeToWallet(u2.ReferralCode)
	assert.Equal(t, "aaa00000001", w1)
	assert.Equal(t, "bbb00000001", w2)

	// 用 u2 的码邀请不会算到 u1 头上
	joinWithCode(t, s, "0xcccc", "9.9.9.9", &u2.ReferralCode)
	refs1, err := s.loadReferrals(u1.ReferralCode)
	require.NoError(t, err)
	assert.Len(t, refs1, 0)
	refs2, err := s.loadReferrals(u2.ReferralCode)
	require.NoError(t, err)
	assert.Len(t, refs2, 1)
}
