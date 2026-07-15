package cnvd_skills

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCnvdSkills_RequestVulDetail 是集成测试，依赖真实网络与代理。
// 离线解析测试见 vul_detail_parse_test.go。
// 无代理或网络受限时会失败，可用 -short 跳过： go test -short
func TestCnvdSkills_RequestVulDetail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent integration test in short mode")
	}
	proxyProvider := FixedProxyProvider("http://121.206.45.124:64257")
	detail, err := NewCnvdSkills().RequestVulDetailByID(context.Background(), "CNVD-2021-67823", proxyProvider)
	assert.Nil(t, err)
	marshal, err := json.Marshal(detail)
	assert.Nil(t, err)
	fmt.Println("抓取结果： " + string(marshal))
}

// TestRequestVulDetail_Real 真实集成测试：从 CNVD 抓取 CNVD-2021-67823 并校验数据
// 格式与有效性。CNVD 触发加速乐图片验证码挑战时用 CommandCaptchaSolver 调
// scripts/ddddocr_solver.py（ddddocr）自动识别。-short 跳过。
func TestRequestVulDetail_Real(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cfg := &Config{
		MaxRetry:              3,
		RequestTimeoutSeconds: 30,
		CaptchaSolver: CommandCaptchaSolver{
			Command: "python3",
			Args:    []string{"../scripts/ddddocr_solver.py"},
		},
	}
	detail, err := NewCnvdSkills().RequestVulDetailByIDWithConfig(ctx, "CNVD-2021-67823", FixedProxyProvider(""), cfg)
	if err != nil {
		t.Fatalf("真实抓取失败（检查网络/CNVD/ddddocr）: %v", err)
	}
	assert.NotNil(t, detail)

	// CNVD-ID 格式校验
	assert.Equal(t, "CNVD-2021-67823", detail.CNVD)
	assert.Regexp(t, `^CVE-\d{4}-\d+$`, detail.CVE, "CVE 应为标准格式")

	// 危害级别非空
	assert.NotNil(t, detail.HazardLevel)
	assert.NotEmpty(t, detail.HazardLevel.Level)

	// 时间字段可解析（Str 非空 → Time 应解析成功）
	if detail.PublishTimeStr != "" {
		assert.NotNil(t, detail.PublishTime, "PublishTimeStr 非空时 PublishTime 应解析成功")
	}

	// URL 回填
	assert.Equal(t, "https://www.cnvd.org.cn/flaw/show/CNVD-2021-67823", detail.URL)
}

// TestFetchVulDetail_Real 单条 API 真实抓取（带验证码识别器）
func TestFetchVulDetail_Real(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cfg := &Config{
		MaxRetry:              3,
		RequestTimeoutSeconds: 30,
		CaptchaSolver: CommandCaptchaSolver{
			Command: "python3",
			Args:    []string{"../scripts/ddddocr_solver.py"},
		},
	}
	detail, err := NewCnvdSkills().FetchVulDetailWithConfig(ctx, "CNVD-2021-67823", FixedProxyProvider(""), cfg)
	if err != nil {
		t.Fatalf("FetchVulDetail 真实抓取失败: %v", err)
	}
	assert.Equal(t, "CNVD-2021-67823", detail.CNVD)
}
