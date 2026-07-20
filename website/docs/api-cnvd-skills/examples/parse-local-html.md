---
outline: deep
---

# 离线解析本地 HTML

`ParseVulDetail` / `ParseVulList` / `ParseVulPatch` 不依赖网络，可直接解析本地保存的 HTML，便于测试与回放。

## 流程

```mermaid
flowchart LR
    F[本地 HTML 文件] --> R[os.ReadFile]
    R --> P[ParseVul*]
    P --> S[结构化对象]
    S --> U[断言/落盘]
```

## 完整代码

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "os"

    "github.com/scagogogo/cnvd-skills/cnvd_skills"
)

func main() {
    x := cnvd_skills.NewCnvdSkills()

    // 解析详情
    detailHTML, err := os.ReadFile("fixtures/cnvd-2021-67823.html")
    if err != nil {
        log.Fatal(err)
    }
    d, err := x.ParseVulDetail(string(detailHTML))
    if err != nil {
        log.Fatal(err)
    }
    b, _ := json.MarshalIndent(d, "", "  ")
    fmt.Println(string(b))

    // 解析列表
    listHTML, err := os.ReadFile("fixtures/list-page-1.html")
    if err != nil {
        log.Fatal(err)
    }
    list, err := x.ParseVulList(string(listHTML))
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("page=%v total=%v items=%d\n", list.Page, list.TotalPage, len(list.VulListItems))

    // 解析补丁
    patchHTML, err := os.ReadFile("fixtures/patch-289241.html")
    if err != nil {
        log.Fatal(err)
    }
    p, err := x.ParseVulPatch(string(patchHTML))
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("patch=%s vendor=%s\n", p.Name, p.Vendor)
}
```

## 用途

- 单元测试：保存真实 HTML fixture，断言解析结果。
- 离线回放：先抓原始 HTML 存盘，之后离线解析，避免反复请求 CNVD。
- 解析历史存档：对已抓取的原始 HTML 重新解析提取新字段。

## 注意

`ParseVulDetail` 返回的 `VulDetail.URL` 为空（URL 由调用方在 `RequestVulDetailByURLWithConfig` 中回填，离线解析不涉及）。如需保留 URL，自行赋值：

```go
d.URL = "https://www.cnvd.org.cn/flaw/show/CNVD-2021-67823"
```

## 相关

- 方法：[ParseVulDetail](../methods/parse-vul-detail)、[ParseVulList](../methods/parse-vul-list)、[ParseVulPatch](../methods/parse-vul-patch)
- 在线版：[单条详情](./single-detail)
