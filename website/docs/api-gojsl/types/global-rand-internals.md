---
outline: deep
---

# globalRand 内部

`globalRand` 是 go-jsl 的全局随机源，用于 UA 池选择与人类节奏抖动。未导出但文档说明。源码：[`gojsl/headers.go`](https://github.com/scagogogo/cnvd-skills/blob/main/gojsl/headers.go) 与 [`gojsl/client.go`](https://github.com/scagogogo/cnvd-skills/blob/main/gojsl/client.go)。

## 定义

```go
var globalRand = rand.New(rand.NewSource(time.Now().UnixNano()))
```

go-jsl 是工具库，无需调用方注入随机源，故用包级全局实例。

## 使用点

| 位置 | 用途 |
|------|------|
| `randomUserAgent` | `globalRand.Intn(len(uaPool))` 从 UA 池随机选 |
| `processCaptcha` | `globalRand.Intn(1000)` 生成 500~1500ms 人类看图反应延迟 |

## Go 1.18 兼容

Go 1.20 之前 `math/rand` 全局源需手动播种（`rand.Seed`），否则每次进程启动序列固定。go-jsl 用 `rand.New(rand.NewSource(time.Now().UnixNano()))` 自带播种，不依赖全局 `rand.Seed`，在 Go 1.18 工具链下也能保证启动随机性。详见 [FAQ - Go 1.18 兼容](/faq/go-1.18-compat)。

```mermaid
flowchart LR
    T[time.Now().UnixNano] --> S[rand.NewSource]
    S --> R[rand.New]
    R --> G[globalRand]
    G -->|Intn| U[UA 池选择]
    G -->|Intn 1000| D[500~1500ms 反应延迟]
```

## 并发安全

`*rand.Rand` 的并发调用不安全，但 go-jsl 推荐每请求独立 `JslClient`（见 [FAQ - 并发安全](/faq/concurrent-safe)），单实例内顺序使用 `globalRand`。若需高并发共享，调用方应自行加锁或每实例独立随机源。

## 相关

- [UA 池内部](/api-gojsl/types/ua-pool-internals)
- [processCaptcha 内部](/api-gojsl/methods/process-captcha-internals)
- [FAQ - Go 1.18 兼容](/faq/go-1.18-compat)
