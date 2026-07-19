package jsl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserAgent_Headers_HasClientHints(t *testing.T) {
	u := uaPool[0]
	h := u.headers()
	assert.Contains(t, h["sec-ch-ua"], u.major)
	assert.Equal(t, "?0", h["sec-ch-ua-mobile"])
	assert.Contains(t, h["sec-ch-ua-platform"], u.platform)
	for _, k := range []string{"Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-User", "Sec-Fetch-Dest"} {
		_, ok := h[k]
		assert.True(t, ok, "应含 %s", k)
	}
	assert.Equal(t, u.ua, h["User-Agent"])
}

func TestUAPool_AllValid(t *testing.T) {
	assert.GreaterOrEqual(t, len(uaPool), 3)
	for _, u := range uaPool {
		assert.True(t, strings.HasPrefix(u.ua, "Mozilla/5.0"), "UA 应是浏览器格式")
		assert.NotEmpty(t, u.major)
		assert.NotEmpty(t, u.platform)
		assert.Contains(t, u.ua, "Chrome/"+u.major+".")
	}
}

func TestRandomUserAgent_InPool(t *testing.T) {
	u := randomUserAgent()
	found := false
	for _, p := range uaPool {
		if p.ua == u.ua {
			found = true
			break
		}
	}
	assert.True(t, found, "randomUserAgent 应返回池中元素")
}

func TestCaptchaHeaders_HasXHRMarkers(t *testing.T) {
	h := captchaHeaders("https://www.cnvd.org.cn/flaw/list")
	assert.Equal(t, "XMLHttpRequest", h["X-Requested-With"])
	assert.Equal(t, "https://www.cnvd.org.cn/flaw/list", h["Referer"])
	assert.Equal(t, "cors", h["Sec-Fetch-Mode"])
	assert.Equal(t, "empty", h["Sec-Fetch-Dest"])
}

func TestNavigationHeaders_EmptyRefererWhenNone(t *testing.T) {
	h := navigationHeaders("")
	_, ok := h["Referer"]
	assert.False(t, ok, "空 Referer 不应拼入")
	h2 := navigationHeaders("https://www.cnvd.org.cn/flaw/list")
	assert.Equal(t, "https://www.cnvd.org.cn/flaw/list", h2["Referer"])
}
