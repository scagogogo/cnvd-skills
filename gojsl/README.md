# go-jsl

破解[加速乐（JSL）](https://www.yunaq.com/)三层加密反爬的纯 Go 客户端，可访问任意被加速乐保护的站点。

## 能力

- **三层解密**：第一层 `document.cookie` 混淆 JS（goja 求值 + 兼容正则提取 cookie，修复 `; Max-age` 大小写格式）；第二层 `go({...})` 参数 + md5/sha1/sha256 暴力匹配算 `__jsl_clearance_s`；第三层带 cookie GET。
- **验证码挑战**：检测到验证码页后自动取图 → 调用 `CaptchaSolver` 识别 → POST 答案 → 放行刷新拿真实页，失败自动换图重试 6 次。
- **可插拔识别器**：`CaptchaSolver` 接口把"图→答案"留给调用方，内置 Noop/Static/Interactive/Command 四种实现。`CommandCaptchaSolver` 配合 `scripts/ddddocr_solver.py`（ddddocr）可全自动通过中文词组验证码。
- **统一 HttpClient**：持有长生命周期 resty client，复用 TCP/TLS 连接、cookie jar 自动管理会话、浏览器级 Header（Client Hints / Fetch Metadata 与 UA 大版本联动）、UA 从真实 Chrome 121/122 池随机、翻页/详情/验证码间隔带随机抖动，降低被反爬识别概率。
- **context / 超时 / 代理**全支持。

## 隐蔽性

`JslClient` 内部经统一 `HttpClient` 收发所有请求（三层解密每一跳、验证码取图/提交、放行刷新），而非每次 `resty.New()`：

- **连接复用**：长生命周期的 `*resty.Client` 复用底层 TCP/TLS 连接，减少握手与 TLS 指纹抖动，贴近真实浏览器单会话行为。
- **cookie jar 自动管理**：`net/http/cookiejar` 自动收发 `Set-Cookie`，三层解密算出的 `__jsl_clearance_s` 同步进 jar 后由 jar 统一携带，无需手动拼 `Cookie` 头。
- **浏览器级 Header 全套**：`Accept`、`Accept-Language`、`sec-ch-ua` / `sec-ch-ua-mobile` / `sec-ch-ua-platform`（Client Hints，与 UA 大版本联动）、`Sec-Fetch-Site/Mode/User/Dest`（Fetch Metadata）、`Upgrade-Insecure-Requests`、`Connection: keep-alive`，对齐现代 Chrome 导航请求。
- **UA 池随机**：UA 从真实 Chrome 121/122（Win/Mac/Linux）池随机选取，Client Hints 头随之联动，避免单一固定 UA 的指纹特征。长会话可调 `RefreshUserAgent()` 轮换。
- **人类节奏抖动**：翻页与详情请求间隔按可配置的 `Jitter`（默认 0.3，范围 ±30%）随机化，验证码取图前加 500~1500ms 人类看图反应延迟，降低机器化节奏特征。

## 安装

```bash
go get github.com/scagogogo/go-jsl
```

## 用法

```go
import "github.com/scagogogo/go-jsl"

client := jsl.NewJslClient("", 30, jsl.CommandCaptchaSolver{
    Command: "python3",
    Args:    []string{"scripts/ddddocr_solver.py"},
})
html, err := client.Get(context.Background(), "https://www.cnvd.org.cn/flaw/show/CNVD-2021-67823")
```

## 错误处理

未配识别器遇验证码返回 `ErrCaptchaRequired`，多次识别失败返回 `ErrCaptchaSolveFailed`，均可用 `errors.Is` 判断。

```go
if errors.Is(err, jsl.ErrCaptchaRequired) {
    // 需配置识别器
}
```

## 依赖

goja（JS 引擎）、go-resty（HTTP）。无私有依赖。

## 测试

```bash
# 离线测试
go test ./... -short -v

# 真实集成测试（需可访问外网 + ddddocr）
go test ./... -run "_Real" -v -timeout 400s
```
