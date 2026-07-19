package cnvd_skills

import (
	"context"
	"testing"
	"time"

	"github.com/scagogogo/go-jsl"
	"github.com/stretchr/testify/assert"
)

// TestRequestVulListByQuery_Real 真实集成测试：按关键词 "XStream" 检索列表，
// 验证查询参数拼装 + jsl 三层 + 验证码全链路。CNVD 触发验证码时用
// CommandCaptchaSolver 调 gojsl/scripts/ddddocr_solver.py 自动识别。-short 跳过。
func TestRequestVulListByQuery_Real(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cfg := &Config{
		MaxRetry:              3,
		RequestTimeoutSeconds: 30,
		CaptchaSolver: jsl.CommandCaptchaSolver{
			Command: "python3",
			Args:    []string{"../gojsl/scripts/ddddocr_solver.py"},
		},
	}
	q := VulListQuery{Keyword: "XStream"}
	list, err := NewCnvdSkills().RequestVulListByQueryWithConfig(ctx, q, 0, FixedProxyProvider(""), cfg)
	if err != nil {
		t.Fatalf("真实检索失败（检查网络/CNVD/ddddocr）: %v", err)
	}
	assert.NotNil(t, list)
	assert.NotEmpty(t, list.VulListItems, "XStream 关键词应至少返回一条")
	for _, item := range list.VulListItems {
		assert.Regexp(t, `^/flaw/show/CNVD-\d{4}-\d+$`, item.Href)
	}
}
