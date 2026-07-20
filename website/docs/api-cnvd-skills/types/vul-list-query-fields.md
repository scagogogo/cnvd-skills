---
outline: deep
---

# VulListQuery 字段逐项

```go
type VulListQuery struct {
    Keyword        string
    KeywordFlag    int
    StartDate      string
    Endate         string
    CnvdID         string
    CnvdIDFlag     int
    CategoryId     string
    ManufacturerId string
    Serverity      string
    ReferenceScope int
    Order          string
    NumPerPage     int
}
```

## 字段表

| 字段 | 类型 | 默认 | URL 参数 | 用途 |
| --- | --- | --- | --- | --- |
| Keyword | `string` | `""` | `keyword` | 关键词检索 |
| KeywordFlag | `int` | `0` | `keywordFlag` | 0=AND, 1=OR |
| StartDate | `string` | `""` | `startDate` | 起始公开日期 |
| Endate | `string` | `""` | `endDate` | 截止公开日期 |
| CnvdID | `string` | `""` | `cnvdId` | 按 CNVD-ID 检索 |
| CnvdIDFlag | `int` | `0` | `cnvdIdFlag` | 0=AND, 1=OR |
| CategoryId | `string` | `""` | `categoryId` | 漏洞类别 ID |
| ManufacturerId | `string` | `""` | `manufacturerId` | 厂商 ID |
| Serverity | `string` | `""` | `serverity` + `serverityIdStr` | 危害级别 ID |
| ReferenceScope | `int` | `0` | `referenceScope` | 参考编号范围 |
| Order | `string` | `""` | `order` | 排序方式 |
| NumPerPage | `int` | `0` | `numPerPage` + `max` | 每页条数（0→10） |

## 零值语义

零值字段不拼入 query string，按 CNVD 默认行为处理。`NumPerPage<=0` 时用默认 10（`itoaOrDefault`）。

## buildQueryURL 映射

```mermaid
flowchart LR
    Q[VulListQuery] --> B[buildQueryURL offset]
    B --> URL["/flaw/list?numPerPage&offset&max..."]
```

分组详解：[日期](./vul-list-query-date)、[标志位](./vul-list-query-flags)、[ID 类](./vul-list-query-ids)。

## 示例

```go
q := cnvd_skills.VulListQuery{
    Keyword:   "Apache",
    StartDate: "2024-01-01",
    Endate:    "2024-06-30",
    NumPerPage: 10,
}
list, _ := x.RequestVulListByQuery(ctx, q, 0, proxy)
```
