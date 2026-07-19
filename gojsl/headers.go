package jsl

import (
	"fmt"
	"math/rand"
	"time"
)

// userAgent 封装一个真实 Chrome 浏览器的 UA 字符串与其配套 Header。
// UA 与 sec-ch-ua 大版本必须联动，否则反爬可从 Client Hints 与 UA 不一致识别。
type userAgent struct {
	ua       string
	major    string
	platform string
}

func (u userAgent) string() string { return u.ua }

// headers 返回该 UA 对应的浏览器级默认 Header 全套。
// 覆盖现代 Chrome 必带的 Client Hints（sec-ch-ua*）与 Fetch Metadata（Sec-Fetch-*），
// 缺这些是非浏览器的强特征。
func (u userAgent) headers() map[string]string {
	chUa := fmt.Sprintf(`"Chromium";v="%s", "Not(A:Brand";v="24", "Google Chrome";v="%s"`, u.major, u.major)
	return map[string]string{
		"User-Agent":                u.ua,
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"Accept-Language":           "zh-CN,zh;q=0.9",
		"Accept-Encoding":           "gzip, deflate",
		"sec-ch-ua":                 chUa,
		"sec-ch-ua-mobile":          "?0",
		"sec-ch-ua-platform":        fmt.Sprintf(`"%s"`, u.platform),
		"Sec-Fetch-Site":            "same-origin",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-User":            "?1",
		"Sec-Fetch-Dest":            "document",
		"Upgrade-Insecure-Requests": "1",
		"Connection":                "keep-alive",
	}
}

// uaPool 真实 Chrome 稳定大版本 UA 池。每项 UA 与 major/platform 联动。
var uaPool = []userAgent{
	{
		ua:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		major:    "122",
		platform: "Windows",
	},
	{
		ua:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		major:    "121",
		platform: "Windows",
	},
	{
		ua:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		major:    "122",
		platform: "macOS",
	},
	{
		ua:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		major:    "121",
		platform: "Linux",
	},
}

// globalRand 全局随机源（gojsl 是工具库，无需调用方注入源）。
var globalRand = rand.New(rand.NewSource(time.Now().UnixNano()))

// randomUserAgent 从 UA 池随机选一个。
func randomUserAgent() userAgent {
	return uaPool[globalRand.Intn(len(uaPool))]
}

// captchaHeaders 返回验证码端点（XHR）专用的额外 Header。
// 现代浏览器 fetch 不发 X-Requested-With，但 CNVD 的 captcha.js 仍检查它，故保留。
// Referer 由调用方按目标 URL 传入。
func captchaHeaders(referer string) map[string]string {
	return map[string]string{
		"Accept":           "application/json, text/javascript, */*; q=0.01",
		"X-Requested-With": "XMLHttpRequest",
		"Referer":          referer,
		"Sec-Fetch-Site":   "same-origin",
		"Sec-Fetch-Mode":   "cors",
		"Sec-Fetch-Dest":   "empty",
	}
}

// navigationHeaders 返回普通页面导航请求的额外 Header（Referer 由调用方传入）。
func navigationHeaders(referer string) map[string]string {
	h := map[string]string{}
	if referer != "" {
		h["Referer"] = referer
	}
	return h
}
