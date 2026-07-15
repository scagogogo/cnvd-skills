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

// jslClient 破解加速乐（JSL）三层加密的 HTTP 客户端。
// 复刻 github.com/JSREP/go-jsl-sdk 流程，修复其 first 层 cookie 正则不兼容
// CNVD 当前 `; Max-age`（大写带空格）格式的问题，并接入 context 与超时。
type jslClient struct {
	cookieMap map[string]string
	proxy     string
	timeout   time.Duration
	solver    CaptchaSolver
}

func newJslClient(proxy string, timeoutSeconds int, solver CaptchaSolver) *jslClient {
	timeout := time.Duration(0)
	if timeoutSeconds > 0 {
		timeout = time.Duration(timeoutSeconds) * time.Second
	}
	return &jslClient{
		cookieMap: make(map[string]string),
		proxy:     proxy,
		timeout:   timeout,
		solver:    solver,
	}
}

// Get 对被加速乐保护的目标 URL 发起 GET，自动完成三层解密，返回最终页面 HTML。
// 若第三层返回验证码挑战页且配置了 solver，则自动取图→识别→提交→刷新拿真实页。
func (x *jslClient) Get(ctx context.Context, targetUrl string) (string, error) {
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
func (x *jslClient) handlePossibleCaptcha(ctx context.Context, targetUrl, resp string) (string, error) {
	if !isCaptchaChallenge(resp) {
		return resp, nil
	}
	if x.solver == nil {
		return "", ErrCaptchaRequired
	}
	if err := x.processCaptcha(ctx, targetUrl); err != nil {
		return "", err
	}
	// 放行后重新请求目标页拿真实内容
	return x.plainRequest(ctx, targetUrl)
}

// processCaptcha 完整执行验证码挑战：取图→识别→提交，最多重试 6 次。
// 重试是因为验证码图为中文词组、ddddocr 识别有概率性，多次重试可显著提升通过率。
func (x *jslClient) processCaptcha(ctx context.Context, targetUrl string) error {
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
func (x *jslClient) fetchCaptchaImage(ctx context.Context, targetUrl string) (imageBase64, sec string, err error) {
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
func (x *jslClient) submitCaptchaAnswer(ctx context.Context, targetUrl, ans, sec string) error {
	body := "ans=" + url.QueryEscape(ans) + "&sec=" + url.QueryEscape(sec)
	_, err := x.captchaRequest(ctx, "https://www.cnvd.org.cn/cdn-cgi/captcha/v2/captcha/image", targetUrl, body)
	return err
}

// captchaRequest 对验证码端点发请求（GET 或 POST），共用 jsl 会话 cookie。
// postBody 非空时为 POST application/x-www-form-urlencoded。
// 端点返回非 200 视为失败。
func (x *jslClient) captchaRequest(ctx context.Context, reqURL, referer, postBody string) (string, error) {
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
func (x *jslClient) plainRequest(ctx context.Context, targetUrl string) (string, error) {
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
	// 收集响应 Set-Cookie
	for _, c := range resp.Cookies() {
		x.cookieMap[c.Name] = c.Value
	}
	if x.isBlockedByShield(resp.String()) {
		return "", fmt.Errorf("blocked by 创宇盾 (proxy may be banned): %s", targetUrl)
	}
	return resp.String(), nil
}

// processFirstLayer 从第一层加密响应解出初始 cookie。
// 用 goja 求值 document.cookie=XXX 的 JS 得到 name=value;Max-age=... 字符串，
// 再用兼容正则提取（修复库的正则不兼容 ; Max-age 大写带空格）。
func (x *jslClient) processFirstLayer(responseBody string) error {
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
	// 兼容正则：覆盖 ;max-age / ; Max-age / ; Max-Age 等大小写与空格组合
	submatch := regexp.MustCompile(`(.+?)=(.+?);\s*[Mm]ax-[Aa]ge`).FindStringSubmatch(setCookieStr)
	if len(submatch) != 3 {
		return fmt.Errorf("can not extract cookie value: %s", setCookieStr)
	}
	x.cookieMap[submatch[1]] = submatch[2]
	return nil
}

func (x *jslClient) isFirstLayer(body string) bool {
	return strings.HasPrefix(body, "<script>document.cookie=") &&
		strings.HasSuffix(body, ";location.href=location.pathname+location.search</script>")
}

// processSecondLayer 破解第二层 go({...}) 参数，算出真正的 __jsl_clearance_s cookie。
func (x *jslClient) processSecondLayer(ctx context.Context, responseBody string) error {
	submatch := regexp.MustCompile(`go\(({.+?})\)`).FindStringSubmatch(responseBody)
	if len(submatch) != 2 {
		return fmt.Errorf("can not find go(params) in second layer response")
	}
	var params secondLayerParams
	if err := json.Unmarshal([]byte(submatch[1]), &params); err != nil {
		return fmt.Errorf("unmarshal second layer params failed: %w", err)
	}
	cookie, cost := x.newCookie(&params)
	// 用解析出的 wt 做休眠（而非硬编码 1500），防 wt 变化
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

// isSecondLayer 宽松判断：含 __jsl_clearance + ct 字段且以 })</script> 结尾，
// 不再硬编码 wt 值，抵抗加速乐调整 wt/vt。
func (x *jslClient) isSecondLayer(body string) bool {
	return strings.HasSuffix(body, "})</script>") &&
		strings.Contains(body, `"tn":"__jsl_clearance`) &&
		strings.Contains(body, `"ct":"`)
}

// secondLayerParams 第二层 go({...}) 的参数（复刻 jsl_sdk SecondResponseParams）。
type secondLayerParams struct {
	Bts   []string `json:"bts"`
	Chars string   `json:"chars"`
	Ct    string   `json:"ct"`
	Ha    string   `json:"ha"`
	Tn    string   `json:"tn"`
	Vt    string   `json:"vt"`
	Wt    string   `json:"wt"`
}

// newCookie 复刻 jsl_sdk 的纯 Go 破解算法（md5/sha1/sha256）。
func (x *jslClient) newCookie(params *secondLayerParams) (string, int64) {
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

func (x *jslClient) cookieHeaderValue() string {
	var b strings.Builder
	for name, value := range x.cookieMap {
		b.WriteString(name)
		b.WriteString("=")
		b.WriteString(value)
		b.WriteString("; ")
	}
	return b.String()
}

func (x *jslClient) isBlockedByShield(body string) bool {
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
