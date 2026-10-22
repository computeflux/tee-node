package gateway

import (
	"sort"
	"testing"

	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/stretchr/testify/assert"
)

// TestDiffModelSets 验证上架/下架集合差异计算。
func TestDiffModelSets(t *testing.T) {
	prev := map[string]bool{"gpt-4": true, "claude-3": true}
	curr := map[string]bool{"claude-3": true, "gemini-2": true}

	added, removed := diffModelSets(prev, curr)
	sort.Strings(added)
	sort.Strings(removed)
	assert.Equal(t, []string{"gemini-2"}, added)
	assert.Equal(t, []string{"gpt-4"}, removed)
}

// TestDiffModelSets_NoChange 集合不变时无差异。
func TestDiffModelSets_NoChange(t *testing.T) {
	s := map[string]bool{"gpt-4": true}
	added, removed := diffModelSets(s, s)
	assert.Empty(t, added)
	assert.Empty(t, removed)
}

// TestActiveModelNames 从 nameMap 提取活跃模型名。
func TestActiveModelNames(t *testing.T) {
	nameMap := map[string]cgateway.ModelInfoV2{
		"gpt-4":    {Name: "gpt-4"},
		"claude-3": {Name: "claude-3"},
	}
	names := activeModelNames(nameMap)
	assert.True(t, names["gpt-4"])
	assert.True(t, names["claude-3"])
	assert.Len(t, names, 2)

	// nil 输入返回 nil
	assert.Nil(t, activeModelNames(nil))
}
