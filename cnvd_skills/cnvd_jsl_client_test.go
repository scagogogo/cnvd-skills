package cnvd_skills

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestJslClient_ProcessFirstLayer_ExtractsCookie 验证修复后的正则能从
// 真实 first 层响应（含 ; Max-age=3600; Path=/; SameSite=None; Secure）提取 cookie。
func TestJslClient_ProcessFirstLayer_ExtractsCookie(t *testing.T) {
	body, err := os.ReadFile("testdata/jsl_first_layer_sample.html")
	assert.Nil(t, err)

	c := newJslClient("", 0, nil)
	err = c.processFirstLayer(string(body))
	assert.Nil(t, err)

	// cookie 名应为 __jsl_clearance_s
	v, ok := c.cookieMap["__jsl_clearance_s"]
	assert.True(t, ok, "应提取到 __jsl_clearance_s cookie")
	assert.NotEmpty(t, v)
	// 值应包含 |- 分隔的时间戳与签名结构
	assert.Contains(t, v, "|")
	assert.Contains(t, v, "%3D")
}

// TestJslClient_IsFirstLayer 判断函数对真实 fixture 返回 true
func TestJslClient_IsFirstLayer(t *testing.T) {
	body, _ := os.ReadFile("testdata/jsl_first_layer_sample.html")
	c := newJslClient("", 0, nil)
	assert.True(t, c.isFirstLayer(string(body)))
	assert.False(t, c.isFirstLayer("<html>normal page</html>"))
}

// TestJslClient_IsSecondLayer 宽松判断不依赖固定 wt 值
func TestJslClient_IsSecondLayer(t *testing.T) {
	c := newJslClient("", 0, nil)
	// wt=3000（非 1500）也应被识别
	body := `...go({"bts":["x","y"],"chars":"ab","ct":"deadbeef","ha":"sha256","tn":"__jsl_clearance_s","vt":"3600","wt":"3000"})</script>`
	assert.True(t, c.isSecondLayer(body))
	assert.False(t, c.isSecondLayer("<html>normal</html>"))
}

// TestJslClient_NewCookie 复刻的破解算法不 panic 且命中时返回正确结构。
// 构造一个 ct 已知的参数确保命中。
func TestJslClient_NewCookie(t *testing.T) {
	c := newJslClient("", 0, nil)
	// 构造确定命中的场景：chars="ab"，bts=["pre-","-post"]，
	// 预算 "pre-aa-post" 的 sha256 作为 ct，确保 newCookie 能命中并返回 "pre-aa-post"。
	v := "pre-aa-post"
	params := &secondLayerParams{
		Bts:   []string{"pre-", "-post"},
		Chars: "ab",
		Ha:    "sha256",
	}
	// 用与 newCookie 相同算法预算 ct
	h := sha256.Sum256([]byte(v))
	params.Ct = hex.EncodeToString(h[:])

	cookie, _ := c.newCookie(params)
	assert.NotEmpty(t, cookie, "ct 已知匹配时应返回非空 cookie")
	assert.Equal(t, "pre-aa-post", cookie)
}
