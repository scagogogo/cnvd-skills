# cnvd-skills

对 [CNVD（国家信息安全漏洞共享平台）](https://www.cnvd.org.cn) 网站页面与接口的 Go 封装库，支持漏洞列表、漏洞详情、厂商补丁三类页面的抓取与解析，内置代理 IP 轮换、重试、超时与去重。

## 功能

- **漏洞列表** `/flaw/list` —— `RequestVulListByOffset` + `ParseVulList`，解析当前页码、总页数、总记录数与条目
- **漏洞详情** `/flaw/show/CNVD-xxx` —— `RequestVulDetailByID` / `RequestVulDetailByURL` + `ParseVulDetail`，解析 CNVD/CVE/危害级别/影响产品/描述/参考链接/补丁/附件等，时间字段同时提供字符串与 `*time.Time`
- **厂商补丁** `/patchInfo/show/:id` —— `RequestVulPatchByID` / `RequestVulPatchByURL` + `ParseVulPatch`
- **单条抓取** `FetchVulDetail(cnvd)` —— 不落盘，返回结构化结果
- **主流程** `VulList(ctx, proxyProvider, config)` —— 翻页抓取 + 逐条详情 + JSONL 落盘，按总页数停止、按 CNVD 去重

## 安装

```bash
go get github.com/scagogogo/cnvd-skills
```

> 依赖私有仓库 `github.com/JSREP/go-jsl-sdk`（带 JS 渲染的 HTTP 客户端），需配置 `GOPRIVATE` 拉取，权限申请联系 `CC111001100@qq.com`。

## 用法

```go
package main

import (
	"context"
	"fmt"

	"github.com/scagogogo/cnvd-skills/cnvd_skills"
)

func main() {
	ctx := context.Background()
	err := cnvd_skills.NewCnvdSkills().VulList(
		ctx,
		cnvd_skills.PinYiProxyProvider,
		cnvd_skills.DefaultConfig(),
	)
	if err != nil {
		fmt.Println("抓取出错： " + err.Error())
	}
}
```

### 单条抓取

```go
detail, err := cnvd_skills.NewCnvdSkills().FetchVulDetail(
	context.Background(),
	"CNVD-2021-67823",
	cnvd_skills.PinYiProxyProvider,
)
if err == nil {
	fmt.Println(detail.CNVD, detail.CVE, detail.HazardLevel.Level)
}
```

## 配置

`Config` 字段（`DefaultConfig()` 提供默认值）：

| 字段 | 默认 | 说明 |
|------|------|------|
| OutputPath | `data/test.jsonl` | 抓取结果输出路径 |
| NumPerPage | 10 | 每页条数 |
| ListPageIntervalSeconds | 3 | 翻页间隔（秒） |
| DetailIntervalSeconds | 3 | 详情请求间隔（秒） |
| ProxyRetryIntervalSeconds | 3 | 代理失效重试间隔（秒） |
| MaxRetry | 3 | 单次请求最大重试次数 |
| RequestTimeoutSeconds | 30 | 单次请求超时（秒，0=不限） |
| EnableDedup | true | 是否按 CNVD-ID 去重输出 |

## 代理

实现 `ProxyProvider func() (string, error)` 即可接入任意代理源。内置：

- `PinYiProxyProvider()` —— 品易代理 API
- `FixedProxyProvider(proxy)` —— 固定 IP（测试用）

## 测试

```bash
# 离线测试（解析逻辑，不依赖网络与代理）
go test ./cnvd_skills/ -short -v

# 全量测试（含依赖网络的集成测试，需可用代理）
go test ./cnvd_skills/ -v
```

## 设计要点

- **解析与请求分离**：`ParseXxx` 接受纯字符串入参、返回结构体与 error，可用本地 HTML fixture 离线测试，无需网络与代理。
- **请求层重试**：`requestWithRetry` 统一封装 `jsl_sdk.Get`，代理类错误自动换 IP、非代理错误按 `MaxRetry` 重试，全程支持 `context.Context` 取消。
- **去重**：`EnableDedup` 开启时，写文件前读取已抓 CNVD 集合，跳过重复条目，支持断点续抓。
- **不 panic**：所有错误返回 error，库代码无 `panic` 调用。
