package jsl

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

// HttpClient 是 gojsl 内部统一收发 HTTP 请求的客户端。
//
// 相比每次请求 resty.New()，它持有一个长生命周期的 *resty.Client：
//   - 启用 cookie jar，Set-Cookie 自动管理，无需手动拼 Cookie 头
//   - 复用底层 TCP/TLS 连接（keep-alive），减少握手与 TLS 指纹抖动
//   - 集中配置 UA / Header 策略（见 headers.go）
//
// 一个 HttpClient 实例对应一个"浏览器会话"，非并发安全（cookie jar 会累积）。
// JslClient 内部持有一个 HttpClient，三层解密每一跳与验证码流程都经它收发。
type HttpClient struct {
	client *resty.Client
	mu     sync.Mutex
	ua     userAgent
}

// NewHttpClient 构造一个统一 HTTP 客户端。
// proxy 为空串表示直连；timeoutSeconds 为 0 表示不限时。
// 启用 cookie jar，设置浏览器级默认 Header（见 applyBrowserHeaders）。
func NewHttpClient(proxy string, timeoutSeconds int) *HttpClient {
	jar, _ := cookiejar.New(nil)
	client := resty.New().
		SetCookieJar(jar).
		SetHeaderVerbatim("Accept-Language", "zh-CN,zh;q=0.9").
		SetRedirectPolicy(resty.FlexibleRedirectPolicy(10))
	if proxy != "" {
		client.SetProxy(proxy)
	}
	if timeoutSeconds > 0 {
		client.SetTimeout(time.Duration(timeoutSeconds) * time.Second)
	}
	hc := &HttpClient{client: client}
	hc.ua = hc.pickUserAgent()
	hc.applyBrowserHeaders()
	return hc
}

// Client 返回底层 resty client（供需要直接操作的场景，如写入解密中间 cookie）。
func (h *HttpClient) Client() *resty.Client { return h.client }

// SetCookie 往 cookie jar 写入一个 cookie（用于把解密算出的 __jsl_clearance_s 等同步进会话）。
func (h *HttpClient) SetCookie(targetURL, name, value string) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return
	}
	h.client.GetClient().Jar.SetCookies(u, []*http.Cookie{
		{Name: name, Value: value, Path: "/", Domain: u.Hostname()},
	})
}

// Cookies 返回 jar 中某 URL 的所有 cookie（供调试与兼容旧 cookieMap 读取）。
func (h *HttpClient) Cookies(targetURL string) []*http.Cookie {
	u, err := url.Parse(targetURL)
	if err != nil {
		return nil
	}
	return h.client.GetClient().Jar.Cookies(u)
}

// Do 发起一次 GET，返回响应体字符串。
// extraHeaders 用于按场景附加/覆盖 Header（如 captcha 请求加 X-Requested-With）。
func (h *HttpClient) Do(ctx context.Context, targetURL string, extraHeaders map[string]string) (string, error) {
	req := h.client.R().SetContext(ctx)
	for k, v := range extraHeaders {
		req.SetHeader(k, v)
	}
	resp, err := req.Get(targetURL)
	if err != nil {
		return "", err
	}
	return resp.String(), nil
}

// DoPost 发起一次 POST（application/x-www-form-urlencoded），返回响应体字符串。
func (h *HttpClient) DoPost(ctx context.Context, targetURL, body string, extraHeaders map[string]string) (string, error) {
	req := h.client.R().SetContext(ctx).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetBody(body)
	for k, v := range extraHeaders {
		req.SetHeader(k, v)
	}
	resp, err := req.Post(targetURL)
	if err != nil {
		return "", err
	}
	return resp.String(), nil
}

func (h *HttpClient) pickUserAgent() userAgent {
	return randomUserAgent()
}

// applyBrowserHeaders 把当前 UA 对应的浏览器级 Header 全套设到 client 默认头。
func (h *HttpClient) applyBrowserHeaders() {
	h.client.SetHeaders(h.ua.headers())
}

// RefreshUserAgent 轮换到另一个 UA（供长会话定期换装，降低指纹固定风险）。
func (h *HttpClient) RefreshUserAgent() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ua = h.pickUserAgent()
	h.applyBrowserHeaders()
}
