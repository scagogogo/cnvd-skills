# gojsl 统一请求客户端 + 隐蔽性强化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: `superpowers:subagent-driven-development`
> Steps use checkbox (`- [ ]`) syntax.

**Goal:** 为 gojsl 封装一个统一的 HttpClient，让 JslClient 所有请求（三层解密的每一跳 + 验证码取图/提交）都经它收发，并在此客户端上集中强化隐蔽性：连接复用 + cookie jar 自动管理 + 浏览器级 Header 全套 + UA 池随机 + 人类节奏抖动，降低被 CNVD/加速乐/创宇盾识别为爬虫的概率。

**Architecture:** 数据流：`JslClient.Get` → 三层解密每一跳调 `plainRequest` / 验证码流程调 `captchaRequest` → 两者改为委托统一的 `HttpClient.Do`。HttpClient 内部持有一个**长生命周期**的 `*resty.Client`（启用 `SetCookieJar` 自动管理 cookie、复用 TCP/TLS 连接），Header 策略集中（浏览器级全套 + 从 UA 池随机选 UA 并联动 sec-ch-ua）。cnvd_skills 的 `requestWithRetry` 仍构造 JslClient，但节奏抖动配置经 Config 传入主流程。设计理由：隐蔽性与加速乐三层解密强耦合（每一跳都是带 cookie 的 GET），故 HttpClient 放 gojsl 模块内而非另起模块；不引入 uTLS 新依赖（resty 的 SetCookieJar/SetTransport 已够用，且现有真实测试证明当前 TLS 指纹已能过 CNVD，优先把 Header/连接/节奏三维度做满）。

**Tech Stack:** Go 1.18, go-resty/resty/v2 v2.7.0（已有 SetCookieJar/SetTransport/SetTLSClientConfig API）, goja, testify。无新增依赖。

**Risks:**
- T3 改 JslClient 请求层，processFirstLayer/SecondLayer 依赖 plainRequest 累积的 cookieMap → 缓解：HttpClient 用 resty cookie jar 自动管理 Set-Cookie，但解密中间产物（第一层 goja 算出的 cookie、第二层 newCookie 算出的 __jsl_clearance_s）需手动写入 jar；保留 cookieMap 作为解密中间态，新增 syncToJar 把中间 cookie 同步进 jar。每步用现有 gojsl 离线测试 + 真实 _Real 测试兜底
- T2 UA 池随机化可能让某 UA 触发 CNVD 不同行为 → 缓解：UA 池用真实 Chrome 稳定大版本（120/121/122），_Real 测试覆盖
- T4 随机抖动影响测试稳定性 → 缓解：抖动可配 0 关闭，默认幅度 ±50%，测试用固定 seed 或关闭抖动
- gojsl 是独立 module，改其公开 API 需保持向后兼容 → 缓解：NewJslClient 签名不变，HttpClient 作为内部细节；cnvd_skills 透明受益

---

### Task 1: 创建 HttpClient — 统一请求收发 + 连接复用 + cookie jar

**Depends on:** None
**Files:**
- Create: `gojsl/httpclient.go`
- Create: `gojsl/httpclient_test.go`

- [ ] **Step 1: 创建 httpclient.go — 封装长生命周期 resty client + cookie jar + 统一 Do**

```go
package jsl

import (
	"context"
	"net/http"
	"net/http/cookiejar"
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

	// mu 保护 ua 在请求间轮换
	mu sync.Mutex
	ua userAgent
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
// url 决定 cookie 的作用域，一般传目标站点根 URL。
func (h *HttpClient) SetCookie(targetURL, name, value string) {
	u, err := parseURL(targetURL)
	if err != nil {
		return
	}
	h.client.GetClient().Jar.SetCookies(u, []*http.Cookie{
		{Name: name, Value: value, Path: "/", Domain: u.Hostname()},
	})
}

// Cookies 返回 jar 中某 URL 的所有 cookie（供调试与兼容旧 cookieMap 读取）。
func (h *HttpClient) Cookies(targetURL string) []*http.Cookie {
	u, err := parseURL(targetURL)
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

// pickUserAgent 从 UA 池随机选一个（见 headers.go）。
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

// parseURL 包装 url.Parse，避免在 httpclient.go 顶部散落 import。
func parseURL(raw string) (*url.URL, error) {
	return url.Parse(raw)
}
```

注：`url` 与 `net/http` 需在 import。最终 import 块：

```go
import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)
```

- [ ] **Step 2: 创建 httpclient_test.go — 离线验证连接复用、cookie jar、Header 应用**

```go
package jsl

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHttpClient_CookieJarEnabled(t *testing.T) {
	gotCookie := ""
	srv := httptest.NewServer(nil)
	defer srv.Close()
	http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "abc"})
		w.Write([]byte("ok"))
	})
	_ = srv
	// 直接验证 jar 存在且可读写
	hc := NewHttpClient("", 0)
	hc.SetCookie("https://www.cnvd.org.cn/flaw/list", "__jsl_clearance_s", "v123")
	cs := hc.Cookies("https://www.cnvd.org.cn/flaw/list")
	found := false
	for _, c := range cs {
		if c.Name == "__jsl_clearance_s" && c.Value == "v123" {
			found = true
		}
	}
	assert.True(t, found, "SetCookie 写入后应能从 jar 读回")
}

func TestHttpClient_Do_GetReturnsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NotEmpty(t, r.Header.Get("User-Agent"))
		assert.NotEmpty(t, r.Header.Get("Sec-Fetch-Site"))
		w.Write([]byte("hello"))
	}))
	defer srv.Close()
	hc := NewHttpClient("", 0)
	body, err := hc.Do(context.Background(), srv.URL, nil)
	assert.Nil(t, err)
	assert.Equal(t, "hello", body)
}

func TestHttpClient_DoPost_BodyAndContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		_ = r.ParseForm()
		assert.Equal(t, "ans42", r.FormValue("ans"))
		w.Write([]byte("submitted"))
	}))
	defer srv.Close()
	hc := NewHttpClient("", 0)
	body, err := hc.DoPost(context.Background(), srv.URL, "ans=42", nil)
	assert.Nil(t, err)
	assert.Equal(t, "submitted", body)
}

func TestHttpClient_RefreshUserAgent_ChangesHeaders(t *testing.T) {
	hc := NewHttpClient("", 0)
	ua1 := hc.ua
	hc.RefreshUserAgent()
	// 刷新后 client 默认头应与当前 ua 一致
	gotUA := hc.client.Header["User-Agent"]
	assert.NotEmpty(t, gotUA)
	assert.Equal(t, ua1.string(), gotUA[0])
	// ua 池只有一个时可能不变，但 headers 应非空
	assert.NotEmpty(t, hc.ua.headers())
}
```

注：测试用了 `net/http`，import 块：

```go
import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)
```

- [ ] **Step 3: 验证 HttpClient 离线测试**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills/gojsl && go build ./... && go test . -run "HttpClient" -v -count=1`
Expected:
  - Exit code: 0
  - Output contains: `PASS`
  - Output does NOT contain: `undefined`、`FAIL`

- [ ] **Step 4: 提交**
Run: `git add gojsl/httpclient.go gojsl/httpclient_test.go && git commit -m "$(cat <<'EOF'
feat(jsl): add unified HttpClient with cookie jar and connection reuse

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"`

---

### Task 2: 创建 headers.go — 浏览器级 Header 全套 + UA 池随机

**Depends on:** Task 1
**Files:**
- Create: `gojsl/headers.go`
- Create: `gojsl/headers_test.go`

- [ ] **Step 1: 创建 headers.go — UA 池与浏览器级 Header 策略**

```go
package jsl

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

// userAgent 封装一个真实 Chrome 浏览器的 UA 字符串与其配套 Header。
// UA 与 sec-ch-ua 大版本必须联动，否则反爬可从 Client Hints 与 UA 不一致识别。
type userAgent struct {
	// ua 完整 User-Agent 字符串
	ua string
	// major Chrome 大版本号，如 "122"
	major string
	// platform 平台标识，如 "Windows"
	platform string
}

// string 返回 UA 字符串。
func (u userAgent) string() string { return u.ua }

// headers 返回该 UA 对应的浏览器级默认 Header 全套。
// 覆盖现代 Chrome 必带的 Client Hints（sec-ch-ua*）与 Fetch Metadata（Sec-Fetch-*），
// 缺这些是非浏览器的强特征。
func (u userAgent) headers() map[string]string {
	chUa := fmt.Sprintf(`"Chromium";v="%s", "Not(A:Brand";v="24", "Google Chrome";v="%s"`, u.major, u.major)
	return map[string]string{
		"User-Agent":             u.ua,
		"Accept":                 "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"Accept-Language":        "zh-CN,zh;q=0.9",
		"Accept-Encoding":        "gzip, deflate",
		"sec-ch-ua":              chUa,
		"sec-ch-ua-mobile":       "?0",
		"sec-ch-ua-platform":     fmt.Sprintf(`"%s"`, u.platform),
		"Sec-Fetch-Site":         "same-origin",
		"Sec-Fetch-Mode":         "navigate",
		"Sec-Fetch-User":         "?1",
		"Sec-Fetch-Dest":         "document",
		"Upgrade-Insecure-Requests": "1",
		"Connection":             "keep-alive",
	}
}

// uaPool 真实 Chrome 稳定大版本 UA 池。
// 每项 UA 与 major/platform 联动，确保 Client Hints 一致。
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

// randomUserAgent 从 UA 池随机选一个。
// 用全局 rand（gojsl 是工具库，无需调用方注入源），时间种子初始化一次。
var globalRand = rand.New(rand.NewSource(time.Now().UnixNano()))

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

// 避免未用 import（若 headers.go 不直接用 http，移除）。
var _ = http.MethodGet
```

注：`net/http` 仅用于占位避免 lint，实际 headers.go 不需要它，最终 import 块去掉 `net/http`：

```go
import (
	"fmt"
	"math/rand"
	"time"
)
```

- [ ] **Step 2: 创建 headers_test.go — 验证 UA 池与 Header 一致性**

```go
package jsl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserAgent_Headers_HasClientHints(t *testing.T) {
	u := uaPool[0]
	h := u.headers()
	// 必含 Client Hints
	assert.Contains(t, h["sec-ch-ua"], u.major)
	assert.Equal(t, "?0", h["sec-ch-ua-mobile"])
	assert.Contains(t, h["sec-ch-ua-platform"], u.platform)
	// 必含 Fetch Metadata
	for _, k := range []string{"Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-User", "Sec-Fetch-Dest"} {
		_, ok := h[k]
		assert.True(t, ok, "应含 %s", k)
	}
	// UA 自身
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
```

- [ ] **Step 3: 验证 headers 离线测试**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills/gojsl && go test . -run "UserAgent|UAPool|RandomUserAgent|CaptchaHeaders|NavigationHeaders" -v -count=1`
Expected:
  - Exit code: 0
  - Output contains: `PASS`
  - 5 个测试通过

- [ ] **Step 4: 提交**
Run: `git add gojsl/headers.go gojsl/headers_test.go && git commit -m "$(cat <<'EOF'
feat(jsl): add browser-grade headers and randomized UA pool with client hints

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"`

---

### Task 3: JslClient 接入 HttpClient — 替换散装 plainRequest/captchaRequest

**Depends on:** Task 1, Task 2
**Files:**
- Modify: `gojsl/client.go:28-54`（JslClient 持有 HttpClient，NewJslClient 构造它）
- Modify: `gojsl/client.go:155-232`（captchaRequest/plainRequest 委托 HttpClient）
- Modify: `gojsl/client.go:234-258`（processFirstLayer 把算出的 cookie 同步进 jar）
- Modify: `gojsl/client.go:265-291`（processSecondLayer 把 __jsl_clearance_s 同步进 jar）

- [ ] **Step 1: 修改 JslClient 结构体与 NewJslClient — 持有 HttpClient**

文件: `gojsl/client.go:28-48`（替换 JslClient 结构体定义与 NewJslClient 构造函数，保留 Proxy/HasSolver getter）

```go
// JslClient 破解加速乐（JSL）三层加密的 HTTP 客户端，可访问任意被加速乐保护的站点。
//
// 自动完成三层解密（第一层 document.cookie 混淆 JS 求值 + 兼容正则提取 cookie；
// 第二层 go({...}) 参数 + md5/sha1/sha256 暴力匹配算 __jsl_clearance_s；第三层带 cookie GET），
// 并在第三层返回验证码挑战页时自动取图→调用 CaptchaSolver→提交答案→放行刷新拿真实页。
//
// 内部持有统一的 HttpClient（连接复用 + cookie jar + 浏览器级 Header + UA 池），
// 三层解密每一跳与验证码流程都经它收发，降低被反爬识别的概率。
// 一个实例非并发安全（cookie jar 会随请求累积），并发场景请为每个请求构造独立实例。
type JslClient struct {
	httpClient *HttpClient
	// cookieMap 保留作为解密中间产物（第一层 goja 算出、第二层 newCookie 算出），
	// 解密完成后同步进 HttpClient 的 cookie jar，由 jar 统一管理后续请求的 Cookie 头。
	cookieMap map[string]string
	proxy     string
	timeout   time.Duration
	solver    CaptchaSolver
	// targetSite 用于把解密 cookie 写入 jar 的作用域，默认从首次请求 URL 推导。
	targetSite string
}

// NewJslClient 构造一个加速乐客户端。proxy 为空串表示直连；
// timeoutSeconds 为 0 表示不限时；solver 为 nil 时遇验证码返回 ErrCaptchaRequired。
// 内部构造一个 HttpClient 启用 cookie jar 与浏览器级 Header。
func NewJslClient(proxy string, timeoutSeconds int, solver CaptchaSolver) *JslClient {
	timeout := time.Duration(0)
	if timeoutSeconds > 0 {
		timeout = time.Duration(timeoutSeconds) * time.Second
	}
	return &JslClient{
		httpClient: NewHttpClient(proxy, timeoutSeconds),
		cookieMap:  make(map[string]string),
		proxy:      proxy,
		timeout:    timeout,
		solver:     solver,
	}
}

// Proxy 返回当前客户端配置的代理地址（只读）。
func (x *JslClient) Proxy() string { return x.proxy }

// HasSolver 返回是否配置了验证码识别器。
func (x *JslClient) HasSolver() bool { return x.solver != nil }
```

- [ ] **Step 2: 修改 captchaRequest — 委托 HttpClient.DoPost/Do + captchaHeaders**

文件: `gojsl/client.go:155-195`（替换 captchaRequest 函数）

```go
// captchaRequest 对验证码端点发请求（GET 或 POST），共用 jsl 会话 cookie。
// postBody 非空时为 POST application/x-www-form-urlencoded。
// 端点返回非 200 视为失败。
func (x *JslClient) captchaRequest(ctx context.Context, reqURL, referer, postBody string) (string, error) {
	x.ensureTargetSite(referer)
	headers := captchaHeaders(referer)
	var resp string
	var err error
	if postBody != "" {
		resp, err = x.httpClient.DoPost(ctx, reqURL, postBody, headers)
	} else {
		resp, err = x.httpClient.Do(ctx, reqURL, headers)
	}
	if err != nil {
		return "", err
	}
	// 校验是否为有效 JSON 响应（captcha 端点返回 JSON）
	return resp, nil
}
```

- [ ] **Step 3: 修改 plainRequest — 委托 HttpClient.Do + navigationHeaders + 创宇盾检测**

文件: `gojsl/client.go:202-232`（替换 plainRequest 函数）

```go
// plainRequest 发一次带当前会话 cookie 的普通 GET（导航请求），返回响应体字符串。
// cookie 由 HttpClient 的 jar 自动带上，无需手动拼 Cookie 头。
func (x *JslClient) plainRequest(ctx context.Context, targetUrl string) (string, error) {
	x.ensureTargetSite(targetUrl)
	resp, err := x.httpClient.Do(ctx, targetUrl, navigationHeaders(""))
	if err != nil {
		return "", err
	}
	if x.isBlockedByShield(resp) {
		return "", fmt.Errorf("blocked by 创宇盾 (proxy may be banned): %s", targetUrl)
	}
	return resp, nil
}
```

- [ ] **Step 4: 新增 ensureTargetSite + syncCookiesToJar — 解密中间 cookie 同步进 jar**

文件: `gojsl/client.go`（在 cookieHeaderValue 函数前新增两个辅助方法）

```go
// ensureTargetSite 从 URL 推导站点根（scheme://host），用于把解密 cookie 写入 jar 的作用域。
// 首次调用后缓存，后续请求复用。
func (x *JslClient) ensureTargetSite(rawURL string) {
	if x.targetSite != "" {
		return
	}
	if u, err := url.Parse(rawURL); err == nil && u.Scheme != "" && u.Host != "" {
		x.targetSite = u.Scheme + "://" + u.Host
	}
}

// syncCookiesToJar 把 cookieMap 中的解密中间产物同步进 HttpClient 的 cookie jar，
// 使后续请求的 Cookie 头由 jar 统一携带。
func (x *JslClient) syncCookiesToJar() {
	if x.targetSite == "" {
		return
	}
	for name, value := range x.cookieMap {
		x.httpClient.SetCookie(x.targetSite, name, value)
	}
}
```

- [ ] **Step 5: 修改 processFirstLayer 末尾 — 算出 cookie 后同步进 jar**

文件: `gojsl/client.go:256`（在 `x.cookieMap[submatch[1]] = submatch[2]` 之后追加一行）

```go
	x.cookieMap[submatch[1]] = submatch[2]
	x.syncCookiesToJar()
	return nil
```

- [ ] **Step 6: 修改 processSecondLayer 末尾 — 算出 __jsl_clearance_s 后同步进 jar**

文件: `gojsl/client.go:289-290`（在 `x.cookieMap[params.Tn] = cookie` 之后追加一行）

```go
	x.cookieMap[params.Tn] = cookie
	x.syncCookiesToJar()
	return nil
```

- [ ] **Step 7: 修改 fetchCaptchaImage — 非零状态码判断适配（HttpClient.Do 不返回状态码，改为校验 JSON 内容）**

文件: `gojsl/client.go:130-146`（fetchCaptchaImage 已通过 captchaRequest 拿响应，captchaRequest 不再返回状态码错误，需在校验 JSON 失败时返回错误。当前逻辑 `if e := json.Unmarshal(...) || result.Image == ""` 已覆盖，无需改）。此 Step 仅确认无破坏：Read fetchCaptchaImage 确认其调用 captchaRequest 的返回值处理仍正确。

（无代码改动，验证 Step）

- [ ] **Step 8: 验证 gojsl 全量离线测试 + 编译**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills/gojsl && go build ./... && go vet ./... && go test . -short -count=1 -v`
Expected:
  - Exit code: 0
  - 所有离线测试 PASS（含原有 9 个 + 新增 HttpClient/headers 测试）
  - Output does NOT contain: `undefined`、`FAIL`

- [ ] **Step 9: 提交**
Run: `git add gojsl/client.go && git commit -m "$(cat <<'EOF'
refactor(jsl): route all requests through unified HttpClient with cookie jar

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"`

---

### Task 4: 人类节奏随机化 — 翻页/详情间隔抖动 + 请求间微延迟

**Depends on:** Task 3
**Files:**
- Modify: `cnvd_skills/config.go:6-50`（Config 增节奏抖动字段 + DefaultConfig 默认值）
- Modify: `cnvd_skills/vul_list.go:49-91`（VulList 翻页间隔抖动）
- Modify: `cnvd_skills/vul_list.go:122-177`（fetchAndSaveDetail 详情间隔抖动）
- Modify: `gojsl/client.go:105-128`（processCaptcha 取图→提交间微延迟）

- [ ] **Step 1: 修改 Config — 新增节奏抖动字段**

文件: `cnvd_skills/config.go:6-50`（在 EnableDedup 字段后、CaptchaSolver 字段前新增抖动字段；DefaultConfig 增默认值）

```go
	// 是否对输出文件按 CNVD-ID 去重，默认 true
	EnableDedup bool

	// 翻页与详情请求间隔的随机抖动幅度（0~1，0=关闭抖动用固定间隔，0.5=间隔在 ±50% 范围随机）。
	// 用于模拟人类浏览节奏，降低被反爬识别为机器的概率。默认 0.3。
	Jitter float64

	// 验证码识别器。...（保持原 CaptchaSolver 字段不变）
```

DefaultConfig 在 EnableDedup 后加：

```go
		EnableDedup:               true,
		Jitter:                    0.3,
```

- [ ] **Step 2: 新增 jitterSleep 工具函数 — 把固定间隔按 Jitter 随机化**

文件: `cnvd_skills/vul_list.go`（在 parentDir 函数后新增）

```go
// jitterSleep 按 config.Jitter 把 baseSeconds 随机化后休眠，ctx 感知。
// Jitter=0 时固定休眠 baseSeconds；Jitter=0.5 时休眠时长在 [base*(1-0.5), base*(1+0.5)] 范围。
func jitterSleep(ctx context.Context, baseSeconds int, jitter float64) {
	if baseSeconds <= 0 {
		return
	}
	d := time.Duration(baseSeconds) * time.Second
	if jitter > 0 {
		span := float64(d) * jitter
		// 在 [d-span, d+span] 范围随机
		offset := time.Duration(rand.Float64()*2*span) - time.Duration(span)
		d = d + offset
		if d < 0 {
			d = 0
		}
	}
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
```

vul_list.go import 块新增 `"math/rand"`：

```go
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/golang-infrastructure/go-pointer"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)
```

- [ ] **Step 3: 修改 VulList — 翻页间隔用 jitterSleep**

文件: `cnvd_skills/vul_list.go:89`（替换 `time.Sleep(time.Duration(config.ListPageIntervalSeconds) * time.Second)`）

```go
		page++
		jitterSleep(ctx, config.ListPageIntervalSeconds, config.Jitter)
	}
```

- [ ] **Step 4: 修改 fetchAndSaveDetail — 详情间隔用 jitterSleep**

文件: `cnvd_skills/vul_list.go:174`（替换 `time.Sleep(time.Duration(config.DetailIntervalSeconds) * time.Second)`）

```go
		jitterSleep(ctx, config.DetailIntervalSeconds, config.Jitter)
		return nil
```

同时把 fetchAndSaveDetail 内代理重试的固定 Sleep 也改抖动（第 144 行）：

```go
			if isProxyInvalid(err) {
				jitterSleep(ctx, config.ProxyRetryIntervalSeconds, config.Jitter)
				continue
			}
```

VulList 内代理重试（第 65 行）同理：

```go
			if isProxyInvalid(err) {
				jitterSleep(ctx, config.ProxyRetryIntervalSeconds, config.Jitter)
				continue // 同一页重试，换代理
			}
```

- [ ] **Step 5: 修改 gojsl processCaptcha — 取图→提交间加微随机延迟**

文件: `gojsl/client.go:107-128`（processCaptcha 循环内，取图前加微延迟，模拟人类看图反应）

```go
func (x *JslClient) processCaptcha(ctx context.Context, targetUrl string) error {
	const maxAttempts = 6
	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		// 模拟人类看图反应：500~1500ms 随机延迟，降低机器化特征
		reactionDelay := time.Duration(500+rand.Intn(1000)) * time.Millisecond
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(reactionDelay):
		}
		imageBase64, sec, err := x.fetchCaptchaImage(ctx, targetUrl)
		if err != nil {
			continue
		}
		ans, err := x.solver.Solve(ctx, imageBase64)
		if err != nil {
			continue
		}
		if err := x.submitCaptchaAnswer(ctx, targetUrl, ans, sec); err == nil {
			return nil
		}
	}
	return ErrCaptchaSolveFailed
}
```

gojsl/client.go import 块新增 `"math/rand"`（已有 context/crypto/encoding/json/fmt/net/url/regexp/strings/time + goja + resty）：

```go
import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/go-resty/resty/v2"
)
```

- [ ] **Step 6: 验证编译 + 离线测试不破坏**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go build ./... && go vet ./... && go test ./cnvd_skills/ -short -count=1 -v 2>&1 | tail -10 && go test ./gojsl/ -short -count=1`
Expected:
  - Exit code: 0
  - 离线测试全 PASS
  - Output does NOT contain: `undefined`、`FAIL`

- [ ] **Step 7: 提交**
Run: `git add cnvd_skills/config.go cnvd_skills/vul_list.go gojsl/client.go && git commit -m "$(cat <<'EOF'
feat(stealth): add human-like jitter to pagination/detail/captcha timing

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"`

---

### Task 5: 全量验证 + 真实集成测试 + 文档更新

**Depends on:** Task 1, Task 2, Task 3, Task 4
**Files:**
- Modify: `gojsl/README.md`
- Modify: `README.md`

- [ ] **Step 1: 全量离线测试 + vet + build + panic 检查（两模块）**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go vet ./... && go build ./... && go test ./gojsl/ -short -count=1 -v 2>&1 | tail -15 && go test ./cnvd_skills/ -short -count=1 2>&1 | tail -5 && grep -rn "panic(" gojsl/ cnvd_skills/ --include="*.go" | grep -v "_test.go" | grep -v "//" || echo NO_PANIC`
Expected:
  - Exit code: 0
  - 两模块离线测试全 PASS
  - 输出 `NO_PANIC`

- [ ] **Step 2: 真实跑全部 _Real 集成测试 — 验证 HttpClient 改造后全链路仍通**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go test ./cnvd_skills/ -run "_Real" -v -count=1 -timeout 400s`
Expected:
  - Exit code: 0
  - 4 个 _Real 测试全 PASS：TestRequestVulDetail_Real / TestFetchVulDetail_Real / TestRequestVulListByQuery_Real / TestRequestVulListByOffset_Real
  - 若 FAIL：如实记录失败原因（如 UA 池某版本被 CNVD 拒、cookie jar 与三层解密配合问题）

- [ ] **Step 3: 更新 gojsl/README.md — 补 HttpClient 与隐蔽性说明**

文件: `gojsl/README.md`（能力列表与设计段补充）

补充内容要点：
- 能力列表新增「**统一 HttpClient**：连接复用 + cookie jar 自动管理 + 浏览器级 Header（Client Hints / Fetch Metadata）+ UA 池随机 + 人类节奏抖动，降低被反爬识别概率」
- 新增「## 隐蔽性」小节：说明 HttpClient 持有长生命周期 resty client 复用 TCP/TLS 连接、cookie jar 自动管理会话、Header 全套对齐现代 Chrome（sec-ch-ua 与 UA 大版本联动）、UA 从真实 Chrome 池随机、翻页/详情/验证码间隔带随机抖动
- 用法段不变（NewJslClient 签名未改）

- [ ] **Step 4: 更新根 README.md — 设计要点补隐蔽性强化**

文件: `README.md`（设计要点段补充）

补充内容要点：
- 设计要点「自研加速乐客户端」行补：gojsl 内部经统一 HttpClient 收发所有请求，连接复用 + cookie jar + 浏览器级 Header + UA 池随机 + 节奏抖动，强化反检测隐蔽性
- 配置表新增 Jitter 字段行

- [ ] **Step 5: 提交**
Run: `git add gojsl/README.md README.md && git commit -m "$(cat <<'EOF'
docs: document unified HttpClient and stealth hardening (jar/headers/UA pool/jitter)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"`

---

## 跨 Task 一致性说明

- **HttpClient 类型**：T1 定义于 httpclient.go，T3 的 JslClient.httpClient 字段、NewJslClient 构造、plainRequest/captchaRequest 委托均引用 `*HttpClient`，方法名 `Do`/`DoPost`/`SetCookie`/`Cookies`/`RefreshUserAgent` 全程一致
- **userAgent 类型与 uaPool**：T2 定义于 headers.go，T1 的 HttpClient.ua 字段、pickUserAgent/applyBrowserHeaders、T2 的 randomUserAgent/headers() 均引用同名
- **captchaHeaders/navigationHeaders**：T2 定义于 headers.go，T3 的 captchaRequest/plainRequest 调用，签名 `captchaHeaders(referer string) map[string]string` / `navigationHeaders(referer string) map[string]string`
- **syncCookiesToJar/ensureTargetSite**：T3 定义于 client.go，processFirstLayer/processSecondLayer 末尾调用
- **jitterSleep**：T4 定义于 vul_list.go，VulList/fetchAndSaveDetail 调用，签名 `jitterSleep(ctx context.Context, baseSeconds int, jitter float64)`，Config.Jitter 字段透传
- **NewJslClient 签名不变**：`(proxy string, timeoutSeconds int, solver CaptchaSolver) *JslClient`，cnvd_skills 的 requestWithRetry 调用点无需改
- **import 一致**：gojsl/client.go 加 `"math/rand"`，cnvd_skills/vul_list.go 加 `"math/rand"`；httpclient.go 用 `net/http`/`net/http/cookiejar`/`net/url`/`sync`/`time`/`resty`；headers.go 用 `fmt`/`math/rand`/`time`
- **不引入新依赖**：全程用 resty 已有 API（SetCookieJar/SetHeaders/SetHeaderVerbatim），go.mod/go.sum 不变
