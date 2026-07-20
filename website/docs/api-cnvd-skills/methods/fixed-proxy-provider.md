---
outline: deep
---

# FixedProxyProvider

返回一个始终使用固定 IP 的 `ProxyProvider`。

## 签名

```go
func FixedProxyProvider(proxy string) ProxyProvider
```

## 参数

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| proxy | `string` | 固定代理 URL，如 `http://127.0.0.1:8080`；`""` 等价直连 |

## 返回值

`ProxyProvider`（闭包），每次调用返回同一 `proxy`：

```go
return func() (string, error) {
    return proxy, nil
}
```

## 用途

- 本地调试：`FixedProxyProvider("")` 直连。
- 单一固定代理：`FixedProxyProvider("http://127.0.0.1:8080")`。
- 测试 fixture：返回确定性代理，便于单测。

```mermaid
flowchart LR
    F[FixedProxyProvider proxy] --> C[闭包]
    C --> |每次调用| R["(proxy, nil)"]
```

## 与 PinYiProxyProvider 对比

| 函数 | 返回值 | 是否变化 |
| --- | --- | --- |
| `FixedProxyProvider` | `ProxyProvider` | 始终固定 |
| [`PinYiProxyProvider`](./pinyi-proxy-provider) | `(string, error)` | 每次拉新 |

`FixedProxyProvider` 返回 `ProxyProvider` 类型，`PinYiProxyProvider` 直接是 `ProxyProvider` 实例（函数本身符合签名）。

## 示例

```go
x := cnvd_skills.NewCnvdSkills()

// 直连
_, _ = x.FetchVulDetail(ctx, "CNVD-2021-67823", cnvd_skills.FixedProxyProvider(""))

// 固定本地代理
_, _ = x.FetchVulDetail(ctx, "CNVD-2021-67823",
    cnvd_skills.FixedProxyProvider("http://127.0.0.1:8080"))
```
