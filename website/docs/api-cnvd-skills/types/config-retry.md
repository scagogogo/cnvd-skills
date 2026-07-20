---
outline: deep
---

# 重试与超时字段

```go
MaxRetry              int
RequestTimeoutSeconds int
```

## 字段表

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| MaxRetry | `3` | 单次请求最大重试次数（0=不重试，直接返回错误） |
| RequestTimeoutSeconds | `30` | 单次请求超时（秒，0=不限） |

## 作用域

仅在 `requestWithRetry` 中，`config != nil` 时读取：

```go
if config != nil {
    maxRetry = config.MaxRetry
    timeoutSec = config.RequestTimeoutSeconds
    solver = config.CaptchaSolver
}
for attempt := 0; attempt <= maxRetry; attempt++ {
    client := jsl.NewJslClient(proxy, timeoutSec, solver)
    ...
}
```

## 重试流程

```mermaid
flowchart TD
    A[requestWithRetry] --> B{config?}
    B -- nil --> C[attempt 0..0  单次]
    B -- 非 nil --> D[attempt 0..MaxRetry]
    D --> E{Get 成功?}
    E -- 是 --> F[返回 body]
    E -- 否 --> G{isProxyInvalid?}
    G -- 是 --> H[换 IP 重试]
    G -- 否 --> I{ErrCaptchaRequired?}
    I -- 是 --> J[直接返回不重试]
    I -- 否 --> K[等待后重试]
```

验证码错误（`jsl.ErrCaptchaRequired`）不重试，直接返回，需调用方配置 [`CaptchaSolver`](./config-captcha-solver)。

## 示例

```go
cfg := cnvd_skills.DefaultConfig()
cfg.MaxRetry = 5
cfg.RequestTimeoutSeconds = 60
```
