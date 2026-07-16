# cnvd-skills

对 [CNVD（国家信息安全漏洞共享平台）](https://www.cnvd.org.cn) 网站页面与接口的 Go 封装库，支持漏洞列表、漏洞详情、厂商补丁三类页面的抓取与解析，内置代理 IP 轮换、重试、超时、去重，以及加速乐（JSL）三层解密与图片验证码挑战的自动处理。

## 功能

- **漏洞列表** `/flaw/list` —— `RequestVulListByOffset` + `ParseVulList`，解析当前页码、总页数、总记录数与条目
- **漏洞详情** `/flaw/show/CNVD-xxx` —— `RequestVulDetailByID` / `RequestVulDetailByURL` + `ParseVulDetail`，解析 CNVD/CVE/危害级别/影响产品/描述/参考链接/补丁/附件等，时间字段同时提供字符串与 `*time.Time`
- **厂商补丁** `/patchInfo/show/:id` —— `RequestVulPatchByID` / `RequestVulPatchByURL` + `ParseVulPatch`
- **单条抓取** `FetchVulDetail(cnvd)` —— 不落盘，返回结构化结果
- **主流程** `VulList(ctx, proxyProvider, config)` —— 翻页抓取 + 逐条详情 + JSONL 落盘，按总页数停止、按 CNVD 去重
- **反爬穿透** —— 自研加速乐三层解密客户端 + 可插拔验证码识别器（`CaptchaSolver`），自动通过 CNVD 图片验证码挑战

## 安装

```bash
go get github.com/scagogogo/cnvd-skills
```

> 库已移除原先对私有仓库 `github.com/JSREP/go-jsl-sdk` 的依赖，改为自研加速乐客户端（见 `cnvd_skills/cnvd_jsl_client.go`），所有依赖均为公开模块，无需配置 `GOPRIVATE`。

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
		cnvd_skills.FixedProxyProvider(""), // 空串=直连；或填 http://ip:port
		cnvd_skills.DefaultConfig(),
	)
	if err != nil {
		fmt.Println("抓取出错： " + err.Error())
	}
}
```

### 单条抓取（带验证码识别器）

```go
cfg := &cnvd_skills.Config{
	MaxRetry:              3,
	RequestTimeoutSeconds: 30,
	CaptchaSolver: cnvd_skills.CommandCaptchaSolver{
		Command: "python3",
		Args:    []string{"scripts/ddddocr_solver.py"},
	},
}
detail, err := cnvd_skills.NewCnvdSkills().FetchVulDetailWithConfig(
	context.Background(),
	"CNVD-2021-67823",
	cnvd_skills.FixedProxyProvider(""),
	cfg,
)
if err == nil {
	fmt.Println(detail.CNVD, detail.CVE, detail.HazardLevel.Level)
}
```

## 加速乐与验证码挑战

CNVD 由加速乐（JSL）保护，访问需经过三层加密解密，且三层通过后对部分 IP 会再触发**图片验证码挑战**（创宇盾 captcha）。本库自动完成：

1. **三层解密**：第一层 `document.cookie=XXX` 混淆 JS（goja 求值 + 兼容正则提取 cookie）；第二层 `go({...})` 参数 + md5/sha1/sha256 暴力匹配算出 `__jsl_clearance_s`；第三层带 cookie GET 目标页。
2. **验证码挑战**：检测到验证码页（`本站开启了验证码保护` / `/cdn-cgi/js/captcha.js`）后，自动 GET 取图（base64 PNG + sec token）→ 调用 `CaptchaSolver` 识别 → POST 提交答案 → 放行后重新 GET 拿真实页面。识别失败自动换图重试（最多 6 次）。

验证码识别（"图→答案"）是库的责任边界外，故抽成 `CaptchaSolver` 接口由调用方注入。内置实现：

| 实现 | 说明 |
|------|------|
| `NoopCaptchaSolver` | 永不识别，遇验证码直接返回 `ErrCaptchaRequired` |
| `StaticCaptchaSolver` | 返回固定答案/错误，仅供单测 |
| `InteractiveCaptchaSolver` | 把图写到临时目录 + 轮询 `CNVD_CAPTCHA_ANSWER` 环境变量读答案（人工/外部脚本配合） |
| `CommandCaptchaSolver` | 调外部命令识别：stdin 传 base64 PNG，stdout 读答案 |

`CommandCaptchaSolver` 配合 [`ddddocr`](https://github.com/sml2h3/ddddocr) 可全自动通过 CNVD 验证码。仓库自带 `scripts/ddddocr_solver.py`：从 stdin 读 base64 PNG，用 ddddocr 识别后输出答案。安装：

```bash
pip3 install ddddocr  # 受 PEP668 限制的系统加 --break-system-packages
```

未配置 `CaptchaSolver` 时遇验证码返回 `ErrCaptchaRequired`，调用方可用 `errors.Is(err, cnvd_skills.ErrCaptchaRequired)` 判断。

### 独立使用加速乐客户端

`JslClient` 是导出的公开模块，可脱离 CNVD 业务直接访问任意被加速乐保护的站点：

```go
client := cnvd_skills.NewJslClient("", 30, cnvd_skills.CommandCaptchaSolver{
    Command: "python3",
    Args:    []string{"scripts/ddddocr_solver.py"},
})
html, err := client.Get(context.Background(), "https://www.cnvd.org.cn/flaw/show/CNVD-2021-67823")
```

`CnvdSkills` 也持有一个默认 `JslClient` 实例（直连、不限时、不配识别器），可通过 `JslClient()` 获取。带 `Config` 的请求会在 `requestWithRetry` 内按请求派生独立客户端，并发安全。

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
| CaptchaSolver | nil | 验证码识别器，不配则遇验证码返回 `ErrCaptchaRequired` |

### WithConfig API 变体

需要传入 `CaptchaSolver` 等配置时，用 `*WithConfig` 变体（普通版本等价于传 `nil` 配置）：

- `RequestVulDetailByIDWithConfig` / `RequestVulDetailByURLWithConfig`
- `RequestVulListByOffsetWithConfig`
- `FetchVulDetailWithConfig`

## 代理

实现 `ProxyProvider func() (string, error)` 即可接入任意代理源。内置：

- `FixedProxyProvider(proxy)` —— 固定 IP（传空串直连，测试用）
- `PinYiProxyProvider()` —— 品易代理 API（注：其源已下线，DNS 无法解析，仅供兼容）

## 测试

```bash
# 离线测试（解析逻辑，不依赖网络与代理）
go test ./cnvd_skills/ -short -v

# 真实集成测试（直连 CNVD，需可访问外网；含验证码自动识别，需 ddddocr）
go test ./cnvd_skills/ -run "_Real" -v -timeout 400s
```

真实集成测试 `_Real` 系列：直连 CNVD 抓取真实数据，遇验证码用 `CommandCaptchaSolver` 调 `scripts/ddddocr_solver.py` 自动识别。验证码图为中文词组、OCR 有概率性，库内最多重试 6 次，偶发失败重跑即可。

## 设计要点

- **解析与请求分离**：`ParseXxx` 接受纯字符串入参、返回结构体与 error，可用本地 HTML fixture 离线测试，无需网络与代理。
- **自研加速乐客户端**：`jsl_client.go` 复刻并修复了原 jsl_sdk 的三层解密（first 层正则兼容 `; Max-age` 大写带空格格式），接入 context、超时、代理与验证码挑战流程，移除了对私有 jsl_sdk 的依赖。已导出为公开模块 `JslClient`，`CnvdSkills` 持有默认实例，三处请求统一走 `requestWithRetry` 方法派生的独立客户端（并发安全）。
- **验证码可插拔**：识别环节抽成 `CaptchaSolver` 接口，库负责取图/提交/放行刷新/重试，调用方注入"图→答案"实现。
- **请求层重试**：`requestWithRetry` 统一封装，代理类错误自动换 IP、非代理错误按 `MaxRetry` 重试，验证码类错误（`ErrCaptchaRequired`）不重试直接上抛，全程支持 `context.Context` 取消。
- **去重**：`EnableDedup` 开启时，写文件前读取已抓 CNVD 集合，跳过重复条目，支持断点续抓。
- **不 panic**：所有错误返回 error，库代码无 `panic` 调用。
