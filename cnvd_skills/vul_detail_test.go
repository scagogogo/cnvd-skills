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
// 格式与有效性。依赖外网与 CNVD 可达。-short 跳过。
func TestRequestVulDetail_Real(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	detail, err := NewCnvdSkills().RequestVulDetailByID(ctx, "CNVD-2021-67823", PinYiProxyProvider)
	if err != nil {
		t.Fatalf("真实抓取失败（检查网络/代理/CNVD 可用性）: %v", err)
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

// TestFetchVulDetail_Real 单条 API 真实抓取
func TestFetchVulDetail_Real(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	detail, err := NewCnvdSkills().FetchVulDetail(ctx, "CNVD-2021-67823", PinYiProxyProvider)
	if err != nil {
		t.Fatalf("FetchVulDetail 真实抓取失败: %v", err)
	}
	assert.Equal(t, "CNVD-2021-67823", detail.CNVD)
}
