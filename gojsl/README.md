# go-jsl

破解[加速乐（JSL）](https://www.yunaq.com/)三层加密反爬的纯 Go 客户端，可访问任意被加速乐保护的站点。

## 能力

- **三层解密**：第一层 `document.cookie` 混淆 JS（goja 求值 + 兼容正则提取 cookie，修复 `; Max-age` 大小写格式）；第二层 `go({...})` 参数 + md5/sha1/sha256 暴力匹配算 `__jsl_clearance_s`；第三层带 cookie GET。
- **验证码挑战**：检测到验证码页后自动取图 → 调用 `CaptchaSolver` 识别 → POST 答案 → 放行刷新拿真实页，失败自动换图重试 6 次。
- **可插拔识别器**：`CaptchaSolver` 接口把"图→答案"留给调用方，内置 Noop/Static/Interactive/Command 四种实现。`CommandCaptchaSolver` 配合 `scripts/ddddocr_solver.py`（ddddocr）可全自动通过中文词组验证码。
- **context / 超时 / 代理**全支持。

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
