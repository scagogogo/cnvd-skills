package cnvd_skills

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

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
