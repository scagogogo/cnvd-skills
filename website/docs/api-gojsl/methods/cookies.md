---
outline: deep
---

# Cookies 方法

`Cookies` 返回 jar 中某 URL 的所有 cookie。源码：[`gojsl/httpclient.go`](https://github.com/scagogogo/cnvd-skills/blob/main/gojsl/httpclient.go)。

## 签名

```go
func (h *HttpClient) Cookies(targetURL string) []*http.Cookie
```

## 参数与返回

| 参数 | 类型 | 语义 |
|------|------|------|
| `targetURL` | `string` | 用于匹配 cookie 作用域 |

返回 `[]*http.Cookie`：jar 中匹配该 URL 的所有 cookie。`url.Parse` 失败时返回 nil。

## 用途

调试与兼容旧 `cookieMap` 读取：查看当前会话已积累的 cookie（含解密算出的 `__jsl_clearance_s`、`__jsluid_s` 等）。

```mermaid
flowchart LR
    JAR[HttpClient jar] -->|Cookies url| R[[]*http.Cookie]
    R --> D[调试输出]
```

## 示例

```go
package main

import (
    "fmt"

    "github.com/scagogogo/go-jsl"
)

func main() {
    hc := jsl.NewHttpClient("", 30)
    hc.SetCookie("https://www.cnvd.org.cn/", "foo", "bar")
    for _, c := range hc.Cookies("https://www.cnvd.org.cn/") {
        fmt.Printf("%s=%s domain=%s path=%s\n", c.Name, c.Value, c.Domain, c.Path)
    }
}
```

## 相关

- [SetCookie 方法](/api-gojsl/methods/set-cookie)
- [HttpClient 结构](/api-gojsl/types/http-client-struct)
- [架构 - cookie 生命周期](/architecture/cookie-lifecycle)
