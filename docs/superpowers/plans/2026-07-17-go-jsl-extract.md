# go-jsl 独立模块剥离 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: `superpowers:subagent-driven-development`
> Steps use checkbox (`- [ ]`) syntax.

**Goal:** 把加速乐（JSL）逆向能力从 cnvd-skills 剥离为独立 Go 模块 `github.com/scagogogo/go-jsl`（monorepo 子目录 `gojsl/`），含 JslClient + CaptchaSolver 体系 + ddddocr 脚本 + 独立文档，cnvd-skills 反向依赖它，使其成为可独立 `go get` 的全球库卖点。

**Architecture:** 数据流：新模块 `gojsl`（package `jsl`）提供 `JslClient`（三层解密+验证码挑战）+ `CaptchaSolver` 接口/错误/4 实现 + `scripts/ddddocr_solver.py`；cnvd-skills 通过 `require github.com/scagogogo/go-jsl`（本地 `replace ./gojsl`）反向依赖，`config.go` 的 `CaptchaSolver` 字段类型改为 `jsl.CaptchaSolver`，`vul_detail.go` 用 `jsl.NewJslClient` / `jsl.ErrCaptchaRequired`，`cnvd_skills.go` 持有 `*jsl.JslClient`。为什么这样做：JslClient 反向依赖 CaptchaSolver 接口与错误变量，三者必须同模块才能独立编译；monorepo 子目录+replace 既隔离 module 边界（可独立 go get/测试/文档）又无需用户手动建独立 GitHub 仓库，将来拆独立仓库只需 `git subtree split`；cnvd-skills 反向依赖新模块，业务与逆向彻底分层。

**Tech Stack:** Go 1.18, goja v0.0.0-20230427124612, go-resty/resty/v2 v2.7.0, testify 1.8.2。无新增依赖。

**Risks:**
- `CaptchaSolver` 迁走后 cnvd-skills `Config.CaptchaSolver` 字段类型变为 `jsl.CaptchaSolver`，外部调用方传 `CommandCaptchaSolver` 需改 `jsl.CommandCaptchaSolver` → 缓解：预期 API 变更，README 同步；T4 真实测试验证调用路径
- monorepo `replace ./gojsl` 是本地开发配置，发布独立仓库时需移除 → 缓解：README 注明
- 跨包 `errors.Is(err, jsl.ErrCaptchaRequired)` 需 import 新模块 → 缓解：T3 sed 批改 + 编译验证
- 测试 fixture 与脚本路径迁移后测试可能找不到文件 → 缓解：T2 同步迁移 testdata/scripts，测试用相对路径在 gojsl/ 内执行
- 真实集成测试依赖网络与 ddddocr → 缓解：T4 实跑，失败如实报告，离线测试先行

---

### Task 1: 创建 gojsl 模块骨架 + 迁移核心代码

**Depends on:** None
**Files:**
- Create: `gojsl/go.mod`
- Create: `gojsl/client.go`（从 cnvd_skills/jsl_client.go 迁移，package 改 jsl）
- Create: `gojsl/captcha.go`（从 cnvd_skills/captcha.go 迁移，package 改 jsl）

- [ ] **Step 1: 创建 gojsl/go.mod — 独立 module 定义**

```text
module github.com/scagogogo/go-jsl

go 1.18

require (
	github.com/dop251/goja v0.0.0-20230427124612-428fc442ff5f
	github.com/go-resty/resty/v2 v2.7.0
	github.com/stretchr/testify v1.8.2
)
```

- [ ] **Step 2: 创建 gojsl/client.go — 迁移 JslClient，package 改为 jsl**

把 `cnvd_skills/jsl_client.go` 全文复制到 `gojsl/client.go`，仅改第一行 `package cnvd_skills` → `package jsl`。其余代码（import、JslClient 类型、NewJslClient、Get、handlePossibleCaptcha、processCaptcha、fetchCaptchaImage、submitCaptchaAnswer、captchaRequest、isCaptchaChallenge、plainRequest、processFirstLayer、isFirstLayer、processSecondLayer、isSecondLayer、secondLayerParams、newCookie、cookieHeaderValue、isBlockedByShield、atoiSafe）原样保留——CaptchaSolver/ErrCaptchaRequired/ErrCaptchaSolveFailed 已在同模块 captcha.go 定义，无需改引用。

```go
package jsl

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

- [ ] **Step 3: 创建 gojsl/captcha.go — 迁移 CaptchaSolver 体系，package 改为 jsl**

把 `cnvd_skills/captcha.go` 全文复制到 `gojsl/captcha.go`，仅改第一行 `package cnvd_skills` → `package jsl`。其余（CaptchaSolver 接口、ErrCaptchaRequired/ErrCaptchaSolveFailed、NoopCaptchaSolver、InteractiveCaptchaSolver、StaticCaptchaSolver、CommandCaptchaSolver）原样保留。

- [ ] **Step 4: 验证 gojsl 模块独立编译**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills/gojsl && go build ./...`
Expected:
  - Exit code: 0
  - Output does NOT contain: `undefined`、`cannot find package`

- [ ] **Step 5: 提交**
Run: `git add gojsl/ && git commit -m "feat(jsl): extract go-jsl as independent module (JslClient + CaptchaSolver)"`

---

### Task 2: 迁移测试与 fixture 到 gojsl

**Depends on:** Task 1
**Files:**
- Create: `gojsl/client_test.go`（从 cnvd_skills/cnvd_jsl_client_test.go 迁移）
- Create: `gojsl/captcha_test.go`（从 cnvd_skills/captcha_test.go 迁移）
- Create: `gojsl/testdata/jsl_first_layer_sample.html`（从 cnvd_skills/testdata 迁移）
- Create: `gojsl/scripts/ddddocr_solver.py`（从 scripts/ 迁移）

- [ ] **Step 1: 迁移 client_test.go — package 改 jsl**

把 `cnvd_skills/cnvd_jsl_client_test.go` 复制到 `gojsl/client_test.go`，改 `package cnvd_skills` → `package jsl`。其余（4 个测试函数、NewJslClient 调用、cookieMap/secondLayerParams 同包访问）原样保留——同模块内私有字段可访问。

- [ ] **Step 2: 迁移 captcha_test.go — package 改 jsl**

把 `cnvd_skills/captcha_test.go` 复制到 `gojsl/captcha_test.go`，改 `package cnvd_skills` → `package jsl`。其余（5 个测试、errors.Is(ErrCaptchaRequired)、StaticCaptchaSolver）原样保留。

- [ ] **Step 3: 迁移 testdata fixture**

Run: `mkdir -p /home/cc11001100/github/scagogogo/cnvd-skills/gojsl/testdata && cp cnvd_skills/testdata/jsl_first_layer_sample.html gojsl/testdata/`

- [ ] **Step 4: 迁移 ddddocr 脚本**

Run: `mkdir -p /home/cc11001100/github/scagogogo/cnvd-skills/gojsl/scripts && cp scripts/ddddocr_solver.py gojsl/scripts/`

- [ ] **Step 5: 验证 gojsl 模块独立测试通过**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills/gojsl && go test ./... -short -count=1 -v 2>&1 | tail -20`
Expected:
  - Exit code: 0
  - client_test 与 captcha_test 全 PASS
  - Output contains: `PASS`、`ok`

- [ ] **Step 6: 提交**
Run: `git add gojsl/ && git commit -m "test(jsl): migrate jsl client/captcha tests, fixture and ddddocr script to gojsl module"`

---

### Task 3: cnvd-skills 反向依赖改造

**Depends on:** Task 1, Task 2
**Files:**
- Modify: `go.mod`（加 require + replace）
- Delete: `cnvd_skills/captcha.go`、`cnvd_skills/captcha_test.go`、`cnvd_skills/jsl_client.go`、`cnvd_skills/cnvd_jsl_client_test.go`、`cnvd_skills/testdata/jsl_first_layer_sample.html`、`scripts/ddddocr_solver.py`
- Modify: `cnvd_skills/config.go`（import jsl，CaptchaSolver 字段类型）
- Modify: `cnvd_skills/vul_detail.go`（import jsl，NewJslClient/ErrCaptchaRequired/solver 类型）
- Modify: `cnvd_skills/cnvd_skills.go`（import jsl，*JslClient/NewJslClient）
- Modify: `cnvd_skills/vul_detail_test.go`、`cnvd_skills/vul_list_test.go`（CommandCaptchaSolver → jsl.CommandCaptchaSolver）

- [ ] **Step 1: 修改 go.mod — 加 go-jsl 依赖与本地 replace**

文件: `go.mod`（在现有 require 块中增加 go-jsl，并在文件末尾加 replace）

在第一个 `require (...)` 块中增加一行 `github.com/scagogogo/go-jsl v0.0.0-00010101000000-000000000000`，并在 go.mod 末尾追加：

```text
replace github.com/scagogogo/go-jsl => ./gojsl
```

- [ ] **Step 2: 修改 cnvd_skills/config.go — import jsl，CaptchaSolver 字段类型改 jsl.CaptchaSolver**

文件: `cnvd_skills/config.go:1-34`

在 import 块增加 `"github.com/scagogogo/go-jsl"`，并把 `CaptchaSolver CaptchaSolver` 改为 `CaptchaSolver jsl.CaptchaSolver`：

```go
package cnvd_skills

import "github.com/scagogogo/go-jsl"

// Config 抓取配置，控制输出路径、分页大小、请求节奏、重试与去重。
type Config struct {

	// 抓取结果输出文件路径，默认 data/test.jsonl
	OutputPath string

	// 每页漏洞条目数，默认 10（CNVD 列表页固定为 10）
	NumPerPage int

	// 列表翻页之间的休眠时长（秒），默认 3
	ListPageIntervalSeconds int

	// 详情页请求之间的休眠时长（秒），默认 3
	DetailIntervalSeconds int

	// 代理失效后重试前的休眠时长（秒），默认 3
	ProxyRetryIntervalSeconds int

	// 单次请求最大重试次数（0=不重试，直接返回错误），默认 3
	MaxRetry int

	// 单次请求超时（秒，0=不设超时），默认 30
	RequestTimeoutSeconds int

	// 是否对输出文件按 CNVD-ID 去重，默认 true
	EnableDedup bool

	// 验证码识别器。CNVD 触发图片验证码挑战时用于自动通过：
	// 配置后库自动取图→识别→提交→放行刷新；不配置则遇验证码返回 jsl.ErrCaptchaRequired。
	// 内置实现见 go-jsl 包（jsl.CommandCaptchaSolver 等）。
	CaptchaSolver jsl.CaptchaSolver
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		OutputPath:                "data/test.jsonl",
		NumPerPage:                10,
		ListPageIntervalSeconds:   3,
		DetailIntervalSeconds:     3,
		ProxyRetryIntervalSeconds: 3,
		MaxRetry:                  3,
		RequestTimeoutSeconds:     30,
		EnableDedup:               true,
	}
}
```

- [ ] **Step 3: 修改 cnvd_skills/vul_detail.go — import jsl，改 NewJslClient/ErrCaptchaRequired/solver 类型**

文件: `cnvd_skills/vul_detail.go:1-164`

import 块增加 `"github.com/scagogogo/go-jsl"`。requestWithRetry 内：
- `var solver CaptchaSolver` → `var solver jsl.CaptchaSolver`
- `solver = config.CaptchaSolver` 不变（字段已是 jsl.CaptchaSolver 类型）
- `client := NewJslClient(proxy, timeoutSec, solver)` → `client := jsl.NewJslClient(proxy, timeoutSec, solver)`
- `if errors.Is(getErr, ErrCaptchaRequired)` → `if errors.Is(getErr, jsl.ErrCaptchaRequired)`

完整替换后的 requestWithRetry 函数：

```go
func (x *CnvdSkills) requestWithRetry(ctx context.Context, proxyProvider ProxyProvider, config *Config, targetUrl string) (string, error) {
	var lastErr error
	proxy, err := proxyProvider()
	if err != nil {
		return "", err
	}
	maxRetry := 0
	timeoutSec := 0
	var solver jsl.CaptchaSolver
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

		client := jsl.NewJslClient(proxy, timeoutSec, solver)
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
		if errors.Is(getErr, jsl.ErrCaptchaRequired) {
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

- [ ] **Step 4: 修改 cnvd_skills/cnvd_skills.go — import jsl，改 *JslClient/NewJslClient**

文件: `cnvd_skills/cnvd_skills.go:1-21`

```go
package cnvd_skills

import "github.com/scagogogo/go-jsl"

// CnvdSkills 是 CNVD 网站抓取的入口。
// 持有一个默认的加速乐客户端实例 jslClient，用于无 config 的简单请求场景；
// 带 config 的请求会在 requestWithRetry 内按请求派生独立客户端（并发安全）。
type CnvdSkills struct {
	jslClient *jsl.JslClient
}

// NewCnvdSkills 构造一个 CnvdSkills，默认直连、不限时、不配验证码识别器。
func NewCnvdSkills() *CnvdSkills {
	return &CnvdSkills{
		jslClient: jsl.NewJslClient("", 0, nil),
	}
}

// JslClient 返回 CnvdSkills 持有的默认加速乐客户端实例（只读引用）。
// 外部可用它直接访问任意被加速乐保护的 URL。
func (x *CnvdSkills) JslClient() *jsl.JslClient {
	return x.jslClient
}
```

- [ ] **Step 5: 修改测试文件 — CommandCaptchaSolver 改 jsl.CommandCaptchaSolver**

文件: `cnvd_skills/vul_detail_test.go`、`cnvd_skills/vul_list_test.go`

两个文件都：
- import 块增加 `"github.com/scagogogo/go-jsl"`
- `CommandCaptchaSolver{...}` → `jsl.CommandCaptchaSolver{...}`

Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && sed -i 's|"github.com/scagogogo/cnvd-skills/cnvd_skills"|"github.com/scagogogo/cnvd-skills/cnvd_skills"\n\t"github.com/scagogogo/go-jsl"|; s/CommandCaptchaSolver{/jsl.CommandCaptchaSolver{/g' cnvd_skills/vul_detail_test.go cnvd_skills/vul_list_test.go`

注：sed 后人工检查 import 块不重复、对齐。

- [ ] **Step 6: 删除 cnvd_skills 内已迁移的文件**

Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && rm cnvd_skills/captcha.go cnvd_skills/captcha_test.go cnvd_skills/jsl_client.go cnvd_skills/cnvd_jsl_client_test.go cnvd_skills/testdata/jsl_first_layer_sample.html scripts/ddddocr_solver.py`

- [ ] **Step 7: 验证 cnvd-skills 编译 + go mod tidy**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go mod tidy && go build ./... && go vet ./...`
Expected:
  - Exit code: 0
  - Output does NOT contain: `undefined`、`cannot find package`、`not enough arguments`

- [ ] **Step 8: 验证离线测试通过**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go test ./cnvd_skills/ -short -count=1`
Expected:
  - Exit code: 0
  - Output contains: `ok`

- [ ] **Step 9: 提交**
Run: `git add -A && git commit -m "refactor(cnvd-skills): depend on go-jsl module, remove migrated jsl/captcha code"`

---

### Task 4: 全量验证 + 真实集成测试

**Depends on:** Task 1, Task 2, Task 3
**Files:** 无（仅验证）

- [ ] **Step 1: 两模块各自独立验证**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills/gojsl && go vet ./... && go build ./... && go test ./... -short -count=1 2>&1 | tail -5 && cd .. && go vet ./... && go build ./... && go test ./cnvd_skills/ -short -count=1 2>&1 | tail -5`
Expected:
  - Exit code: 0
  - gojsl 与 cnvd_skills 离线测试均 `ok`
  - NO_PANIC（grep 验证）

- [ ] **Step 2: 真实集成测试复跑 — 确认剥离未破坏抓取能力**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go test ./cnvd_skills/ -run "_Real" -v -count=1 -timeout 400s`
Expected:
  - Exit code: 0
  - 三个 _Real 测试全 PASS（详情/单条/列表），证明 jsl.NewJslClient + jsl.CommandCaptchaSolver 链路正常

- [ ] **Step 3: 验证 gojsl 可被独立 go get（replace 路径生效）**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go list -m all | grep go-jsl`
Expected:
  - Output contains: `github.com/scagogogo/go-jsl v0.0.0-... => ./gojsl`
  - Exit code: 0

---

### Task 5: 文档更新

**Depends on:** Task 1, Task 2, Task 3, Task 4
**Files:**
- Create: `gojsl/README.md`
- Modify: `README.md`

- [ ] **Step 1: 创建 gojsl/README.md — 独立卖点文档**

```markdown
# go-jsl

破解[加速乐（JSL）](https://www.yunaq.com/)三层加密反爬的纯 Go 客户端，可访问任意被加速乐保护的站点。

## 能力

- **三层解密**：第一层 `document.cookie` 混淆 JS（goja 求值 + 兼容正则提取 cookie，修复 `; Max-age` 大小写格式）；第二层 `go({...})` 参数 + md5/sha1/sha256 暴力匹配算 `__jsl_clearance_s`；第三层带 cookie GET。
- **验证码挑战**：检测到验证码页后自动取图 → 调用 `CaptchaSolver` 识别 → POST 答案 → 放行刷新拿真实页，失败自动换图重试 6 次。
- **可插拔识别器**：`CaptchaSolver` 接口把"图→答案"留给调用方，内置 Noop/Static/Interactive/Command 四种实现。`CommandCaptchaSolver` 配合 `scripts/ddddocr_solver.py`（ddddocr）可全自动通过中文词组验证码。
- **context / 超时 / 代理**全支持。

## 安装

\`\`\`bash
go get github.com/scagogogo/go-jsl
\`\`\`

## 用法

\`\`\`go
client := jsl.NewJslClient("", 30, jsl.CommandCaptchaSolver{
    Command: "python3",
    Args:    []string{"scripts/ddddocr_solver.py"},
})
html, err := client.Get(context.Background(), "https://www.cnvd.org.cn/flaw/show/CNVD-2021-67823")
\`\`\`

## 错误处理

未配识别器遇验证码返回 `ErrCaptchaRequired`，多次识别失败返回 `ErrCaptchaSolveFailed`，均可用 `errors.Is` 判断。

## 依赖

goja（JS 引擎）、go-resty（HTTP）。无私有依赖。
```

- [ ] **Step 2: 更新根 README — 补 go-jsl 独立模块说明**

文件: `README.md`

在「独立使用加速乐客户端」小节更新引用为 `jsl.NewJslClient`，并在「设计要点」补充「加速乐逆向已剥离为独立模块 [go-jsl](./gojsl)（`github.com/scagogogo/go-jsl`），本库通过 go.mod require + 本地 replace 依赖；该模块可独立 go get，作为通用加速乐绕过库」。

- [ ] **Step 3: 提交**
Run: `git add gojsl/README.md README.md && git commit -m "docs: add go-jsl module README and reference it from root README"`

---

## 跨 Task 一致性说明

- **模块路径**：`github.com/scagogogo/go-jsl`，T1 go.mod 定义、根 go.mod require+replace、T3/T5 引用均用此路径
- **package 名**：`jsl`（非 `gojsl`），T1 client.go/captcha.go、T2 测试、T3 import 均用 `package jsl`
- **JslClient/NewJslClient**：T1 定义于 gojsl/client.go，T3 的 cnvd_skills.go/vul_detail.go 引用 `jsl.JslClient`/`jsl.NewJslClient`，T5 文档示例一致
- **CaptchaSolver/ErrCaptchaRequired/ErrCaptchaSolveFailed/CommandCaptchaSolver**：T1 定义于 gojsl/captcha.go，T3 的 config.go/vul_detail.go/测试引用 `jsl.CaptchaSolver`/`jsl.ErrCaptchaRequired`/`jsl.CommandCaptchaSolver`，T5 文档一致
- **scripts 路径**：T2 迁移到 `gojsl/scripts/ddddocr_solver.py`，T5 文档示例用 `scripts/ddddocr_solver.py`（相对 gojsl 目录）
