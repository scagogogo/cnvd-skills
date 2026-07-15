package cnvd_skills

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/dop251/goja"
	"github.com/go-resty/resty/v2"
	"regexp"
	"strings"
	"time"
)

// jslClient 破解加速乐（JSL）三层加密的 HTTP 客户端。
// 复刻 github.com/JSREP/go-jsl-sdk 流程，修复其 first 层 cookie 正则不兼容
// CNVD 当前 `; Max-age`（大写带空格）格式的问题，并接入 context 与超时。
type jslClient struct {
	cookieMap map[string]string
	proxy     string
	timeout   time.Duration
}

func newJslClient(proxy string, timeoutSeconds int) *jslClient {
	timeout := time.Duration(0)
	if timeoutSeconds > 0 {
		timeout = time.Duration(timeoutSeconds) * time.Second
	}
	return &jslClient{
		cookieMap: make(map[string]string),
		proxy:     proxy,
		timeout:   timeout,
	}
}

// Get 对被加速乐保护的目标 URL 发起 GET，自动完成三层解密，返回最终页面 HTML。
func (x *jslClient) Get(ctx context.Context, targetUrl string) (string, error) {
	// 第一层：解出初始 cookie
	resp, err := x.plainRequest(ctx, targetUrl)
	if err != nil {
		return "", err
	}
	if !x.isFirstLayer(resp) {
		// 未加密，直接返回
		return resp, nil
	}
	if err := x.processFirstLayer(resp); err != nil {
		return "", err
	}

	// 第二层：破解 go({...}) 参数算出新 cookie
	resp, err = x.plainRequest(ctx, targetUrl)
	if err != nil {
		return "", err
	}
	if !x.isSecondLayer(resp) {
		return resp, nil
	}
	if err := x.processSecondLayer(ctx, resp); err != nil {
		return "", err
	}

	// 第三层：带 cookie 取真实页
	resp, err = x.plainRequest(ctx, targetUrl)
	if err != nil {
		return "", err
	}
	return resp, nil
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
