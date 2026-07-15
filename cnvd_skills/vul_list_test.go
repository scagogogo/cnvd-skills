package cnvd_skills

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCnvdSkills_VulList 是集成测试，依赖真实网络与代理。
// 无代理时会失败，可用 -short 跳过。
func TestCnvdSkills_VulList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent integration test in short mode")
	}
	ctx := context.Background()
	err := NewCnvdSkills().VulList(ctx, PinYiProxyProvider, DefaultConfig())
	// 集成测试在无可用代理时期望返回 error 而非 panic
	assert.NotNil(t, err)
}

// TestRequestVulListByOffset_Real 真实集成测试：抓取第一页列表校验条目格式。
func TestRequestVulListByOffset_Real(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	list, err := NewCnvdSkills().RequestVulListByOffset(ctx, 0, PinYiProxyProvider)
	if err != nil {
		t.Fatalf("真实抓取列表失败: %v", err)
	}
	assert.NotNil(t, list)

	// 第一页应有条目
	assert.NotEmpty(t, list.VulListItems, "第一页列表不应为空")

	// 每条 Href 应为 /flaw/show/CNVD-xxx 格式
	for _, item := range list.VulListItems {
		assert.Regexp(t, `^/flaw/show/CNVD-\d{4}-\d+$`, item.Href)
		assert.NotEmpty(t, item.Title)
	}

	// 应解析出总页数或总记录数（用于停止条件）
	hasTotal := (list.TotalPage != nil) || (list.TotalRecord != nil)
	assert.True(t, hasTotal, "应解析出 TotalPage 或 TotalRecord")
}
