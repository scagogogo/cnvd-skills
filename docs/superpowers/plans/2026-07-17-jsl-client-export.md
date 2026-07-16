# JslClient 导出与 CnvdSkills 客户端持有 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: `superpowers:subagent-driven-development`
> Steps use checkbox (`- [ ]`) syntax.

**Goal:** 把加速乐逆向逻辑封装为导出的公开模块 `JslClient`（可独立访问任意被加速乐保护的站点），让 `CnvdSkills` 持有一个 `*JslClient` 实例，三处 CNVD 请求统一走客户端入口，请求层与业务层彻底分层。

**Architecture:** 数据流：调用方 `RequestVulDetailByID(...)` → `CnvdSkills.requestWithRetry(ctx, proxyProvider, config, url)`（方法，负责重试/换代理/超时/取 solver）→ 内部按请求 `NewJslClient(proxy, timeout, solver)` 构造独立客户端（并发安全，不污染共享实例）→ `client.Get(ctx, url)` 完成三层解密 + 验证码挑战 → 返回 HTML → `ParseXxx`。关键组件：`JslClient`（导出类型，字段私有，仅暴露 `NewJslClient` 构造 + `Get` 方法 + 配置 getter）、`CnvdSkills`（持有 `jslClient *JslClient` 默认实例 + `requestWithRetry` 方法）。为什么这样做：导出后外部可直接复用逆向能力访问任意加速乐站点（不止 CNVD）；CNVD 业务层不再直接碰 HTTP 细节，只调统一请求入口，职责分层清晰。

**Tech Stack:** Go 1.18, goja v0.0.0-20230427124612, go-resty/resty/v2 v2.7.0, goquery 1.8.1, testify 1.8.2。无新增依赖。

**Risks:**
- `requestWithRetry` 当前是包级私有函数被三处调用，改成 `CnvdSkills` 方法后调用点 `requestWithRetry(...)` → `x.requestWithRetry(...)` → 缓解：T3 统一改三处调用点，编译会立即暴露遗漏
- 导出 `JslClient` 后若字段也导出会破坏会话状态（cookieMap 被外部篡改）→ 缓解：字段全私有，只导出类型名 + 构造 + Get + 只读 getter
- 客户端配置（proxy/timeout/solver）按请求变化，不能让 CnvdSkills 持有一个固定配置客户端覆盖所有请求 → 缓解：持有字段作为默认/简单场景引擎，requestWithRetry 在有 config 时按请求 `NewJslClient` 派生独立实例（不修改共享字段）
- 真实集成测试依赖网络与 ddddocr → 缓解：T4 实跑，失败如实报告，离线测试先行保证编译与逻辑

---

### Task 1: 导出 JslClient 公开模块

**Depends on:** None
**Files:**
- Create: `cnvd_skills/jsl_client.go`
- Delete: `cnvd_skills/cnvd_jsl_client.go`（内容迁移到 jsl_client.go，符号导出）
- Modify: `cnvd_skills/cnvd_jsl_client_test.go`（引用新符号）

- [ ] **Step 1: 创建 jsl_client.go — 从 cnvd_jsl_client.go 迁移并导出符号**

把 `cnvd_jsl_client.go` 全文内容复制到新文件 `jsl_client.go`，做以下符号改名导出：
- `type jslClient struct` → `type JslClient struct`，字段保持私有（`cookieMap`/`proxy`/`timeout`/`solver` 不导出）
- `func newJslClient(...)` → `func NewJslClient(proxy string, timeoutSeconds int, solver CaptchaSolver) *JslClient`
- 所有方法接收者 `(x *jslClient)` → `(x *JslClient)`
- `secondLayerParams` 保持包内私有（不导出，实现细节）
- 新增导出的只读 getter 供外部查询配置：

```go
package cnvd_skills

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/go-resty/resty/v2"
)

// JslClient 破解加速乐（JSL）三层加密的 HTTP 客户端，可访问任意被加速乐保护的站点。
//
// 自动完成三层解密（第一层 document.cookie 混淆 JS 求值 + 兼容正则提取 cookie；
// 第二层 go({...}) 参数 + md5/sha1/sha256 暴力匹配算 __jsl_clearance_s；第三层带 cookie GET），
// 并在第三层返回验证码挑战页时自动取图→调用 CaptchaSolver→提交答案→放行刷新拿真实页。
//
// 字段私有，外部通过 NewJslClient 构造、Get 发起请求。一个实例非并发安全
// （cookieMap 会随请求累积），并发场景请为每个请求构造独立实例。
type JslClient struct {
	cookieMap map[string]string
	proxy     string
	timeout   time.Duration
	solver    CaptchaSolver
}

// NewJslClient 构造一个加速乐客户端。proxy 为空串表示直连；
// timeoutSeconds 为 0 表示不限时；solver 为 nil 时遇验证码返回 ErrCaptchaRequired。
func NewJslClient(proxy string, timeoutSeconds int, solver CaptchaSolver) *JslClient {
	timeout := time.Duration(0)
	if timeoutSeconds > 0 {
		timeout = time.Duration(timeoutSeconds) * time.Second
	}
	return &JslClient{
		cookieMap: make(map[string]string),
		proxy:     proxy,
		timeout:   timeout,
		solver:    solver,
	}
}

// Proxy 返回当前客户端配置的代理地址（只读）。
func (x *JslClient) Proxy() string { return x.proxy }

// HasSolver 返回是否配置了验证码识别器。
func (x *JslClient) HasSolver() bool { return x.solver != nil }

// Get 对被加速乐保护的目标 URL 发起 GET，自动完成三层解密，返回最终页面 HTML。
// 若第三层返回验证码挑战页且配置了 solver，则自动取图→识别→提交→刷新拿真实页。
func (x *JslClient) Get(ctx context.Context, targetUrl string) (string, error) {
	resp, err := x.plainRequest(ctx, targetUrl)
	if err != nil {
		return "", err
	}
	if !x.isFirstLayer(resp) {
		return x.handlePossibleCaptcha(ctx, targetUrl, resp)
	}
	if err := x.processFirstLayer(resp); err != nil {
		return "", err
	}

	resp, err = x.plainRequest(ctx, targetUrl)
	if err != nil {
		return "", err
	}
	if !x.isSecondLayer(resp) {
		return x.handlePossibleCaptcha(ctx, targetUrl, resp)
	}
	if err := x.processSecondLayer(ctx, resp); err != nil {
		return "", err
	}

	resp, err = x.plainRequest(ctx, targetUrl)
	if err != nil {
		return "", err
	}
	return x.handlePossibleCaptcha(ctx, targetUrl, resp)
}

// handlePossibleCaptcha 检测响应是否为验证码挑战页：
// 若是且配置了 solver，走完整验证码流程后重新 GET 目标页返回真实内容；
// 若是但未配置 solver，返回 ErrCaptchaRequired；
// 若不是验证码页，直接返回原响应。
func (x *JslClient) handlePossibleCaptcha(ctx context.Context, targetUrl, resp string) (string, error) {
	if !isCaptchaChallenge(resp) {
		return resp, nil
	}
	if x.solver == nil {
		return "", ErrCaptchaRequired
	}
	if err := x.processCaptcha(ctx, targetUrl); err != nil {
		return "", err
	}
	return x.plainRequest(ctx, targetUrl)
}

// processCaptcha 完整执行验证码挑战：取图→识别→提交，最多重试 6 次。
func (x *JslClient) processCaptcha(ctx context.Context, targetUrl string) error {
	const maxAttempts = 6
	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
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

// fetchCaptchaImage GET 验证码图端点，返回 base64 图片与 sec token。
func (x *JslClient) fetchCaptchaImage(ctx context.Context, targetUrl string) (imageBase64, sec string, err error) {
	capURL := "https://www.cnvd.org.cn/cdn-cgi/captcha/v2/captcha/image?c=1&s=cnvdskills"
	resp, err := x.captchaRequest(ctx, capURL, targetUrl, "")
	if err != nil {
		return "", "", err
	}
	var result struct {
		Image string `json:"image"`
		Sec   string `json:"sec"`
		Msg   string `json:"msg"`
	}
	if e := json.Unmarshal([]byte(resp), &result); e != nil || result.Image == "" {
		return "", "", fmt.Errorf("parse captcha image failed: %s", resp)
	}
	return result.Image, result.Sec, nil
}

// submitCaptchaAnswer POST 答案，成功（HTTP 200）返回 nil。
func (x *JslClient) submitCaptchaAnswer(ctx context.Context, targetUrl, ans, sec string) error {
	body := "ans=" + url.QueryEscape(ans) + "&sec=" + url.QueryEscape(sec)
	_, err := x.captchaRequest(ctx, "https://www.cnvd.org.cn/cdn-cgi/captcha/v2/captcha/image", targetUrl, body)
	return err
}

// captchaRequest 对验证码端点发请求（GET 或 POST），共用 jsl 会话 cookie。
func (x *JslClient) captchaRequest(ctx context.Context, reqURL, referer, postBody string) (string, error) {
	client := resty.New()
	if x.proxy != "" {
		client.SetProxy(x.proxy)
	}
	if x.timeout > 0 {
		client.SetTimeout(x.timeout)
	}
	req := client.R().
		SetContext(ctx).
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36").
		SetHeader("Accept", "application/json, text/javascript, */*; q=0.01").
		SetHeader("Accept-Language", "zh-CN,zh;q=0.9").
		SetHeader("Referer", referer).
		SetHeader("X-Requested-With", "XMLHttpRequest")
	if cv := x.cookieHeaderValue(); cv != "" {
		req.SetHeader("Cookie", cv)
	}
	var resp *resty.Response
	var err error
	if postBody != "" {
		req.SetHeader("Content-Type", "application/x-www-form-urlencoded")
		req.SetBody(postBody)
		resp, err = req.Post(reqURL)
	} else {
		resp, err = req.Get(reqURL)
	}
	if err != nil {
		return "", err
	}
	for _, c := range resp.Cookies() {
		x.cookieMap[c.Name] = c.Value
	}
	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("captcha endpoint returned %d", resp.StatusCode())
	}
	return resp.String(), nil
}

// isCaptchaChallenge 判断响应是否为加速乐验证码挑战页。
func isCaptchaChallenge(body string) bool {
	return strings.Contains(body, "本站开启了验证码保护") || strings.Contains(body, "/cdn-cgi/js/captcha.js")
}

// plainRequest 发一次带当前 cookie 的普通 GET，返回响应体字符串。
func (x *JslClient) plainRequest(ctx context.Context, targetUrl string) (string, error) {
	client := resty.New()
	if x.proxy != "" {
		client.SetProxy(x.proxy)
	}
	if x.timeout > 0 {
		client.SetTimeout(x.timeout)
	}
	req := client.R().
		SetContext(ctx).
		SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8").
		SetHeader("Accept-Encoding", "gzip, deflate, br").
		SetHeader("Accept-Language", "zh-CN,zh;q=0.9").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36")
	if cv := x.cookieHeaderValue(); cv != "" {
		req.SetHeader("Cookie", cv)
	}
	resp, err := req.Get(targetUrl)
	if err != nil {
		return "", err
	}
	for _, c := range resp.Cookies() {
		x.cookieMap[c.Name] = c.Value
	}
	if x.isBlockedByShield(resp.String()) {
		return "", fmt.Errorf("blocked by 创宇盾 (proxy may be banned): %s", targetUrl)
	}
	return resp.String(), nil
}

// processFirstLayer 从第一层加密响应解出初始 cookie。
func (x *JslClient) processFirstLayer(responseBody string) error {
	find := regexp.MustCompile(`document\.cookie=([\s\S]+?)location\.href=`).FindStringSubmatch(responseBody)
	if len(find) != 2 {
		return fmt.Errorf("can not parse first layer cookie from response")
	}
	vm := goja.New()
	v, err := vm.RunString(find[1])
	if err != nil {
		return fmt.Errorf("goja eval first layer cookie failed: %w", err)
	}
	setCookieStr, ok := v.Export().(string)
	if !ok {
		return fmt.Errorf("first layer cookie is not string: %v", v.Export())
	}
	submatch := regexp.MustCompile(`(.+?)=(.+?);\s*[Mm]ax-[Aa]ge`).FindStringSubmatch(setCookieStr)
	if len(submatch) != 3 {
		return fmt.Errorf("can not extract cookie value: %s", setCookieStr)
	}
	x.cookieMap[submatch[1]] = submatch[2]
	return nil
}

func (x *JslClient) isFirstLayer(body string) bool {
	return strings.HasPrefix(body, "<script>document.cookie=") &&
		strings.HasSuffix(body, ";location.href=location.pathname+location.search</script>")
}

// processSecondLayer 破解第二层 go({...}) 参数，算出真正的 __jsl_clearance_s cookie。
func (x *JslClient) processSecondLayer(ctx context.Context, responseBody string) error {
	submatch := regexp.MustCompile(`go\(({.+?})\)`).FindStringSubmatch(responseBody)
	if len(submatch) != 2 {
		return fmt.Errorf("can not find go(params) in second layer response")
	}
	var params secondLayerParams
	if err := json.Unmarshal([]byte(submatch[1]), &params); err != nil {
		return fmt.Errorf("unmarshal second layer params failed: %w", err)
	}
	cookie, cost := x.newCookie(&params)
	wtMs := 1500
	if n, err := atoiSafe(params.Wt); err == nil && n > 0 {
		wtMs = n
	}
	remain := wtMs - int(cost)
	if remain > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(remain) * time.Millisecond):
		}
	}
	x.cookieMap[params.Tn] = cookie
	return nil
}

func (x *JslClient) isSecondLayer(body string) bool {
	return strings.HasSuffix(body, "})</script>") &&
		strings.Contains(body, `"tn":"__jsl_clearance`) &&
		strings.Contains(body, `"ct":"`)
}

// secondLayerParams 第二层 go({...}) 的参数。
type secondLayerParams struct {
	Bts   []string `json:"bts"`
	Chars string   `json:"chars"`
	Ct    string   `json:"ct"`
	Ha    string   `json:"ha"`
	Tn    string   `json:"tn"`
	Vt    string   `json:"vt"`
	Wt    string   `json:"wt"`
}

// newCookie 复刻加速乐的纯 Go 破解算法（md5/sha1/sha256）。
func (x *JslClient) newCookie(params *secondLayerParams) (string, int64) {
	begin := time.Now()
	for _, c1 := range params.Chars {
		for _, c2 := range params.Chars {
			v := params.Bts[0] + string(c1) + string(c2) + params.Bts[1]
			var result string
			switch params.Ha {
			case "md5":
				h := md5.New()
				h.Write([]byte(v))
				result = hex.EncodeToString(h.Sum(nil))
			case "sha1":
				h := sha1.New()
				h.Write([]byte(v))
				result = fmt.Sprintf("%x", h.Sum(nil))
			case "sha256":
				h := sha256.New()
				h.Write([]byte(v))
				result = fmt.Sprintf("%x", h.Sum(nil))
			}
			if result == params.Ct {
				return v, time.Since(begin).Milliseconds()
			}
		}
	}
	return "", 0
}

func (x *JslClient) cookieHeaderValue() string {
	var b strings.Builder
	for name, value := range x.cookieMap {
		b.WriteString(name)
		b.WriteString("=")
		b.WriteString(value)
		b.WriteString("; ")
	}
	return b.String()
}

func (x *JslClient) isBlockedByShield(body string) bool {
	return strings.Contains(body, `当前访问疑似黑客攻击，已被创宇盾拦截。`)
}

func atoiSafe(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a digit")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
```

- [ ] **Step 2: 删除旧 cnvd_jsl_client.go — 内容已迁移到 jsl_client.go**

Run: `rm cnvd_skills/cnvd_jsl_client.go`

- [ ] **Step 3: 修改 cnvd_jsl_client_test.go — 引用导出符号**

文件: `cnvd_skills/cnvd_jsl_client_test.go`

把测试中所有 `newJslClient(...)` 调用改为 `NewJslClient(...)`。当前测试里是 `newJslClient("",0,nil)` 形式（4 处），全部替换为 `NewJslClient("", 0, nil)`。同时保留测试文件名不变（仅改符号引用）。

Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && sed -i 's/newJslClient/NewJslClient/g' cnvd_skills/cnvd_jsl_client_test.go`

- [ ] **Step 4: 验证编译 + 离线测试**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go build ./... && go test ./cnvd_skills/ -short -count=1`
Expected:
  - Exit code: 0
  - Output contains: `ok`
  - Output does NOT contain: `undefined`、`not enough arguments`、`FAIL`

- [ ] **Step 5: 提交**
Run: `git add cnvd_skills/jsl_client.go cnvd_skills/cnvd_jsl_client_test.go && git rm cnvd_skills/cnvd_jsl_client.go && git commit -m "refactor(jsl): export JslClient as public module, rename jslClient->JslClient, newJslClient->NewJslClient"`

---

### Task 2: CnvdSkills 持有客户端 + requestWithRetry 转为方法

**Depends on:** Task 1
**Files:**
- Modify: `cnvd_skills/cnvd_skills.go:1-8`
- Modify: `cnvd_skills/vul_detail.go:105-161`

- [ ] **Step 1: 修改 cnvd_skills.go — CnvdSkills 持有默认 JslClient 实例**

文件: `cnvd_skills/cnvd_skills.go:1-8`（替换整个文件）

```go
package cnvd_skills

// CnvdSkills 是 CNVD 网站抓取的入口。
// 持有一个默认的加速乐客户端实例 jslClient，用于无 config 的简单请求场景；
// 带 config 的请求会在 requestWithRetry 内按请求派生独立客户端（并发安全）。
type CnvdSkills struct {
	jslClient *JslClient
}

// NewCnvdSkills 构造一个 CnvdSkills，默认直连、不限时、不配验证码识别器。
func NewCnvdSkills() *CnvdSkills {
	return &CnvdSkills{
		jslClient: NewJslClient("", 0, nil),
	}
}

// JslClient 返回 CnvdSkills 持有的默认加速乐客户端实例（只读引用）。
// 外部可用它直接访问任意被加速乐保护的 URL。
func (x *CnvdSkills) JslClient() *JslClient {
	return x.jslClient
}
```

- [ ] **Step 2: 修改 requestWithRetry — 转为 CnvdSkills 方法，内部按请求派生客户端**

文件: `cnvd_skills/vul_detail.go:101-161`（替换 requestWithRetry 整个函数）

```go
// requestWithRetry 对单个 URL 执行带加速乐解密的 GET，失败时按 config 重试。
// 代理类错误（isProxyInvalid）会重新向 proxyProvider 取新 IP 重试；
// 非代理错误在 MaxRetry 次内重试，超出返回最后一次错误。
// config 为 nil 时退化为不重试的单次请求。全程响应 ctx 取消（含飞行中 HTTP）。
//
// 每次尝试按当前 config 派生一个独立的 JslClient（不修改 CnvdSkills 持有的共享实例），
// 保证并发安全。
func (x *CnvdSkills) requestWithRetry(ctx context.Context, proxyProvider ProxyProvider, config *Config, targetUrl string) (string, error) {
	var lastErr error
	proxy, err := proxyProvider()
	if err != nil {
		return "", err
	}
	maxRetry := 0
	timeoutSec := 0
	var solver CaptchaSolver
	if config != nil {
		maxRetry = config.MaxRetry
		timeoutSec = config.RequestTimeoutSeconds
		solver = config.CaptchaSolver
	}
	for attempt := 0; attempt <= maxRetry; attempt++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		client := NewJslClient(proxy, timeoutSec, solver)
		body, getErr := client.Get(ctx, targetUrl)
		if getErr == nil {
			return body, nil
		}
		lastErr = getErr

		if isProxyInvalid(getErr) {
			if config != nil && config.ProxyRetryIntervalSeconds > 0 {
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(time.Duration(config.ProxyRetryIntervalSeconds) * time.Second):
				}
			}
			if newProxy, pErr := proxyProvider(); pErr == nil {
				proxy = newProxy
			}
			continue
		}

		// 验证码类错误不上抛重试（需调用方配识别器），直接返回
		if errors.Is(getErr, ErrCaptchaRequired) {
			return "", getErr
		}

		if config != nil && config.ProxyRetryIntervalSeconds > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(config.ProxyRetryIntervalSeconds) * time.Second):
			}
		}
	}
	return "", lastErr
}
```

- [ ] **Step 3: 验证编译**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go build ./...`
Expected:
  - Exit code: 0
  - Output does NOT contain: `undefined`、`cannot refer to unexported`

- [ ] **Step 4: 提交**
Run: `git add cnvd_skills/cnvd_skills.go cnvd_skills/vul_detail.go && git commit -m "refactor(client): CnvdSkills holds default JslClient, requestWithRetry becomes method deriving per-request clients"`

---

### Task 3: 三处调用点切换 + 测试同步

**Depends on:** Task 2
**Files:**
- Modify: `cnvd_skills/vul_detail.go:186`（requestWithRetry → x.requestWithRetry）
- Modify: `cnvd_skills/vul_list.go:214`（同上）
- Modify: `cnvd_skills/vul_patch.go:43`（同上）

- [ ] **Step 1: 修改 vul_detail.go 调用点 — requestWithRetry 改为方法调用**

文件: `cnvd_skills/vul_detail.go:186`

把 `body, err := requestWithRetry(ctx, proxyProvider, config, detailPageURL)` 改为 `body, err := x.requestWithRetry(ctx, proxyProvider, config, detailPageURL)`。

Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && sed -i 's/body, err := requestWithRetry(ctx, proxyProvider, config, detailPageURL)/body, err := x.requestWithRetry(ctx, proxyProvider, config, detailPageURL)/' cnvd_skills/vul_detail.go`

- [ ] **Step 2: 修改 vul_list.go 调用点 — requestWithRetry 改为方法调用**

文件: `cnvd_skills/vul_list.go:214`

把 `body, err := requestWithRetry(ctx, proxyProvider, config, targetUrl)` 改为 `body, err := x.requestWithRetry(ctx, proxyProvider, config, targetUrl)`。

Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && sed -i 's/body, err := requestWithRetry(ctx, proxyProvider, config, targetUrl)/body, err := x.requestWithRetry(ctx, proxyProvider, config, targetUrl)/' cnvd_skills/vul_list.go`

- [ ] **Step 3: 修改 vul_patch.go 调用点 — requestWithRetry 改为方法调用**

文件: `cnvd_skills/vul_patch.go:43`

把 `body, err := requestWithRetry(ctx, proxyProvider, nil, patchPageURL)` 改为 `body, err := x.requestWithRetry(ctx, proxyProvider, nil, patchPageURL)`。

Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && sed -i 's/body, err := requestWithRetry(ctx, proxyProvider, nil, patchPageURL)/body, err := x.requestWithRetry(ctx, proxyProvider, nil, patchPageURL)/' cnvd_skills/vul_patch.go`

- [ ] **Step 4: 确认无遗留的包级 requestWithRetry 调用**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && grep -rn "requestWithRetry" cnvd_skills/ --include="*.go" | grep -v "x.requestWithRetry" | grep -v "func (x \*CnvdSkills) requestWithRetry" | grep -v "//"`
Expected:
  - Exit code: 0 且无输出（所有调用点都已改为方法调用）

- [ ] **Step 5: 验证编译 + 全量离线测试**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go build ./... && go vet ./... && go test ./cnvd_skills/ -short -count=1`
Expected:
  - Exit code: 0
  - Output contains: `ok`
  - Output does NOT contain: `FAIL`、`cannot use`

- [ ] **Step 6: 提交**
Run: `git add cnvd_skills/vul_detail.go cnvd_skills/vul_list.go cnvd_skills/vul_patch.go && git commit -m "refactor(request): switch all Request* call sites to x.requestWithRetry method"`

---

### Task 4: 全量验证 + 真实集成测试 + 文档

**Depends on:** Task 1, Task 2, Task 3
**Files:**
- Modify: `README.md`

- [ ] **Step 1: 全量离线测试 + vet + build + panic 检查**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go vet ./... && go build ./... && go test ./cnvd_skills/ -short -v -count=1 2>&1 | tail -25 && grep -rn "panic(" cnvd_skills/ --include="*.go" | grep -v "_test.go" | grep -v "//" || echo NO_PANIC`
Expected:
  - Exit code: 0
  - 离线测试全 PASS、集成测试 SKIP
  - 输出 `NO_PANIC`

- [ ] **Step 2: 真实集成测试复跑 — 确认重构未破坏抓取能力**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go test ./cnvd_skills/ -run "_Real" -v -count=1 -timeout 400s`
Expected:
  - Exit code: 0
  - 三个 _Real 测试全 PASS（详情/单条/列表）

- [ ] **Step 3: 更新 README — 补 JslClient 独立用法**

文件: `README.md`（在「加速乐与验证码挑战」小节后新增「加速乐客户端独立使用」子节，并更新「设计要点」）

在「加速乐与验证码挑战」小节末尾追加：

```markdown
### 独立使用加速乐客户端

`JslClient` 是导出的公开模块，可脱离 CNVD 业务直接访问任意被加速乐保护的站点：

\`\`\`go
client := cnvd_skills.NewJslClient("", 30, cnvd_skills.CommandCaptchaSolver{
    Command: "python3",
    Args:    []string{"scripts/ddddocr_solver.py"},
})
html, err := client.Get(context.Background(), "https://www.cnvd.org.cn/flaw/show/CNVD-2021-67823")
\`\`\`

`CnvdSkills` 也持有一个默认 `JslClient` 实例，可通过 `JslClient()` 获取。
```

在「设计要点」中「自研加速乐客户端」一条补充：「已导出为公开模块 `JslClient`，`CnvdSkills` 持有默认实例，三处请求统一走 `requestWithRetry` 方法派生的独立客户端（并发安全）。」

- [ ] **Step 4: 提交**
Run: `git add README.md && git commit -m "docs: document exported JslClient standalone usage and CnvdSkills ownership"`

---

## 跨 Task 一致性说明

- **JslClient 类型名**：T1 定义导出类型 `JslClient`，T2 的 `CnvdSkills.jslClient` 字段类型、`JslClient()` getter、`requestWithRetry` 内 `NewJslClient(...)` 调用均引用此类型名
- **NewJslClient 构造签名**：T1 定义 `(proxy string, timeoutSeconds int, solver CaptchaSolver) *JslClient`，T2 requestWithRetry 调用 `NewJslClient(proxy, timeoutSec, solver)` 签名匹配
- **requestWithRetry 方法**：T2 定义为 `(x *CnvdSkills) requestWithRetry(ctx, proxyProvider, config, targetUrl)`，T3 三处调用点统一为 `x.requestWithRetry(ctx, proxyProvider, config, url)`
- **CaptchaSolver 接口**：T1 的 `JslClient.solver` 字段、T2 的 `config.CaptchaSolver` 取值均引用 captcha.go 已定义的导出接口
- **ErrCaptchaRequired/ErrCaptchaSolveFailed**：T1 的 handlePossibleCaptcha/processCaptcha 引用 captcha.go 已定义的错误变量
