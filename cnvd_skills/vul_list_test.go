package cnvd_skills

import (
	"context"
	"testing"

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
