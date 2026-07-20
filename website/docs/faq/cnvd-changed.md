---
outline: deep
---

# CNVD 改版如何应对

CNVD 站点改版可能导致 go-jsl 三层解密或验证码流程失效。本页给出识别与应对步骤。

## 改版迹象

| 迹象 | 可能原因 |
|------|----------|
| `can not parse first layer cookie` | 第一层 JS 结构变化 |
| `can not find go(params)` | 第二层 `go({...})` 结构变化 |
| `parse captcha image failed` | 验证码端点返回格式变化 |
| `captcha endpoint returned %d` | 验证码端点路径或行为变化 |
| 响应非 HTML 但无错误 | 页面结构变化导致解析空 |

## 排查流程

```mermaid
flowchart TD
    E[改版错误] --> S[抓取原始响应体]
    S --> L1{第一层正则匹配}
    L1 -- 否 --> F1[更新 isFirstLayer/processFirstLayer]
    L1 -- 是 --> L2{第二层 go(params)}
    L2 -- 否 --> F2[更新 isSecondLayer/processSecondLayer]
    L2 -- 是 --> L3{验证码端点}
    L3 -- 异常 --> F3[更新 captcha 端点/格式]
    L3 -- 正常 --> O[其他 改页面解析]
```

## 抓取原始响应体定位

构造一个 `HttpClient` 直接 GET，打印响应体定位变化：

```go
package main

import (
    "context"
    "fmt"

    "github.com/scagogogo/go-jsl"
)

func main() {
    hc := jsl.NewHttpClient("", 30)
    body, err := hc.Do(context.Background(), "https://www.cnvd.org.cn/", nil)
    if err != nil {
        fmt.Println("err:", err)
        return
    }
    fmt.Println("body length:", len(body))
    fmt.Println("first 500:", body[:min(500, len(body))])
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
```

## 常见改版点与应对

### 第一层

正则 `document\.cookie=([\s\S]+?)location\.href=` 提取 JS。若 CNVD 改用其他写法（如 `location.replace`），需更新正则。兼容正则 `(.+?)=(.+?);\s*[Mm]ax-[Aa]ge` 覆盖大小写空格组合。

### 第二层

正则 `go\(({.+?})\)` 提取参数。`secondLayerParams` 字段名（`bts/chars/ct/ha/tn/vt/wt`）若变化需同步结构体 tag。`newCookie` 的哈希算法（md5/sha1/sha256）与候选构造 `v = bts[0] + c1 + c2 + bts[1]` 一般稳定。

### 验证码端点

`processCaptcha` 端点固定 `https://www.cnvd.org.cn/cdn-cgi/captcha/v2/captcha/image`。若 CNVD 改路径或参数（`c=1&s=cnvdskills`），需更新 `fetchCaptchaImage` / `submitCaptchaAnswer`。`isCaptchaChallenge` 检测关键字 `本站开启了验证码保护` / `/cdn-cgi/js/captcha.js`，若改文案需同步。

## 上报与更新

发现改版请到 [GitHub Issues](https://github.com/scagogogo/cnvd-skills/issues) 上报，附错误信息与原始响应体片段。维护者会更新 go-jsl 适配。

## 相关

- [三层解密深度解析](/api-gojsl/three-layers-deep-dive)
- [secondLayerParams](/api-gojsl/types/second-layer-params)
- [processCaptcha 内部](/api-gojsl/methods/process-captcha-internals)
- [错误处理示例](/api-gojsl/examples/error-handling)
