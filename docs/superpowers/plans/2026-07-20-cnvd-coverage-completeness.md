# CNVD 目标网站封装完善度补齐 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: `superpowers:subagent-driven-development`
> Steps use checkbox (`- [ ]`) syntax.

**Goal:** 补齐 CNVD 封装的三处完善度缺口：补丁页 `WithConfig` 变体缺失、`VulList` 主流程不透传验证码识别器、列表页搜索/筛选能力完全缺失（无法按关键词/日期/厂商/级别定向检索）。

**Architecture:** 数据流按"请求层补齐 → 检索能力补齐 → 测试与文档"三层推进。请求层：补丁页补 `WithConfig` 变体，`VulList` 主流程改为内部统一走 `*WithConfig` 透传 `config.CaptchaSolver`，使全量落盘抓取遇验证码可自动通过。检索层：新增 `VulListQuery` 结构体（封装真实表单字段 keyword/startDate/endDate/categoryId/manufacturerId/serverity 等，已用真实列表页探测确认字段名），新增 `RequestVulListByQuery[WithConfig]` 构造查询 URL，`VulList` 主流程新增可选 `Query` 入参（向后兼容：nil 时走原全量翻页路径）。测试层：补丁与搜索各加离线 fixture 解析测试。设计选择：不改既有 `RequestVulListByOffset` 签名（向后兼容），新增 `RequestVulListByQuery` 与 `VulListWithQuery` 并存；`VulList` 改为内部委托 `*WithConfig`，签名不变。

**Tech Stack:** Go 1.18, goquery 1.8.1, go-jsl（jsl.CaptchaSolver）, testify v1.11.1。无新增依赖。

**Risks:**
- T2 改 `VulList` 内部调用为 `*WithConfig`，若 `config==nil` 行为应与原版完全一致 → 缓解：`VulList` 入口 `config==nil` 时 `config=DefaultConfig()`，DefaultConfig 的 CaptchaSolver 为 nil，等价原行为；离线逻辑无网络不受影响，用现有 `TestCnvdSkills_VulList` 验证不破坏
- T3 查询参数字段名来自真实页面探测，可能随 CNVD 改版变化 → 缓解：字段名作为 `VulListQuery` 命名导出字段，调用方按名设值，改版只需改 URL 拼装一处；离线 fixture 测试锁定字段→URL 映射
- T3 新增 `VulListWithQuery` 与 `VulList` 并存可能造成 API 冗余 → 缓解：`VulList` 保持全量抓取语义，`VulListWithQuery` 为定向抓取入口，职责清晰不重叠

---

### Task 1: 补丁页补 WithConfig 变体 — 让补丁抓取能过验证码

**Depends on:** None
**Files:**
- Modify: `cnvd_skills/vul_patch.go:36-53`

- [ ] **Step 1: 修改 RequestVulPatchByID — 拆出 WithConfig 并委托**

文件: `cnvd_skills/vul_patch.go:36-53`（替换 RequestVulPatchByID 与 RequestVulPatchByURL 两个函数，并在其前新增两个 WithConfig 函数）

```go
// RequestVulPatchByID 根据补丁ID请求补丁详情，如 289241。
func (x *CnvdSkills) RequestVulPatchByID(ctx context.Context, patchID string, proxyProvider ProxyProvider) (*VulPatch, error) {
	return x.RequestVulPatchByIDWithConfig(ctx, patchID, proxyProvider, nil)
}

// RequestVulPatchByIDWithConfig 同 RequestVulPatchByID，但接收 config，
// 可传入 CaptchaSolver 以通过加速乐验证码挑战。
func (x *CnvdSkills) RequestVulPatchByIDWithConfig(ctx context.Context, patchID string, proxyProvider ProxyProvider, config *Config) (*VulPatch, error) {
	targetUrl := "https://www.cnvd.org.cn/patchInfo/show/" + patchID
	return x.RequestVulPatchByURLWithConfig(ctx, targetUrl, proxyProvider, config)
}

// RequestVulPatchByURL 根据补丁详情页URL请求并解析。内部走 requestWithRetry。
func (x *CnvdSkills) RequestVulPatchByURL(ctx context.Context, patchPageURL string, proxyProvider ProxyProvider) (*VulPatch, error) {
	return x.RequestVulPatchByURLWithConfig(ctx, patchPageURL, proxyProvider, nil)
}

// RequestVulPatchByURLWithConfig 同 RequestVulPatchByURL，但接收 config，
// 可传入 CaptchaSolver 以通过加速乐验证码挑战。
func (x *CnvdSkills) RequestVulPatchByURLWithConfig(ctx context.Context, patchPageURL string, proxyProvider ProxyProvider, config *Config) (*VulPatch, error) {
	body, err := x.requestWithRetry(ctx, proxyProvider, config, patchPageURL)
	if err != nil {
		return nil, err
	}
	patch, err := x.ParseVulPatch(body)
	if err != nil {
		return nil, err
	}
	patch.URL = patchPageURL
	return patch, nil
}
```

- [ ] **Step 2: 验证补丁 WithConfig 编译与离线测试不破坏**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go build ./... && go test ./cnvd_skills/ -short -count=1 -run "Patch" -v`
Expected:
  - Exit code: 0
  - Output contains: `PASS`
  - Output does NOT contain: `undefined`、`FAIL`

- [ ] **Step 3: 提交**
Run: `git add cnvd_skills/vul_patch.go && git commit -m "$(cat <<'EOF'
feat(patch): add WithConfig variants for RequestVulPatch to support captcha solver

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"`

---

### Task 2: VulList 主流程透传 CaptchaSolver — 全量落盘抓取能过验证码

**Depends on:** Task 1
**Files:**
- Modify: `cnvd_skills/vul_list.go:49-91`（VulList 函数体）
- Modify: `cnvd_skills/vul_list.go:119-177`（fetchAndSaveDetail 调用点）

- [ ] **Step 1: 修改 VulList — 内部改走 *WithConfig 透传 solver**

文件: `cnvd_skills/vul_list.go:49-91`（替换 VulList 函数，仅改列表请求与详情请求的调用为 WithConfig 变体）

```go
// VulList 抓取漏洞列表并逐条抓取详情，写入输出文件（JSONL）。
// 接收 config 控制输出路径与节奏；接收 ctx 支持取消。
// config.CaptchaSolver 非空时，列表与详情请求遇加速乐验证码挑战自动通过。
// 不再 panic，所有错误返回 error。当 TotalPage 可解析时按总页数停止，否则持续翻页直到详情列表为空。
func (x *CnvdSkills) VulList(ctx context.Context, proxyProvider ProxyProvider, config *Config) error {
	if config == nil {
		config = DefaultConfig()
	}
	page := 1
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		offset := (page - 1) * config.NumPerPage
		list, err := x.RequestVulListByOffsetWithConfig(ctx, offset, proxyProvider, config)
		if err != nil {
			if isProxyInvalid(err) {
				time.Sleep(time.Duration(config.ProxyRetryIntervalSeconds) * time.Second)
				continue // 同一页重试，换代理
			}
			return err
		}

		// 列表为空 → 抓取完成
		if len(list.VulListItems) == 0 {
			fmt.Println("当前页无漏洞条目，抓取完成")
			return nil
		}

		for _, item := range list.VulListItems {
			if err := x.fetchAndSaveDetail(ctx, proxyProvider, config, item); err != nil {
				return err
			}
		}

		// 有总页数则按其停止
		if list.TotalPage != nil && page >= *list.TotalPage {
			fmt.Printf("已抓取到最后一页（第 %d 页），抓取完成\n", page)
			return nil
		}
		page++
		time.Sleep(time.Duration(config.ListPageIntervalSeconds) * time.Second)
	}
}
```

- [ ] **Step 2: 修改 fetchAndSaveDetail — 详情请求改走 WithConfig**

文件: `cnvd_skills/vul_list.go:141`（仅替换详情请求那一行）

```go
		detail, err := x.RequestVulDetailByURLWithConfig(ctx, "https://www.cnvd.org.cn"+item.Href, proxyProvider, config)
```

- [ ] **Step 3: 验证编译 + 离线测试不破坏**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go build ./... && go test ./cnvd_skills/ -short -count=1 -run "VulList|ParseVulList" -v`
Expected:
  - Exit code: 0
  - Output contains: `PASS`
  - Output does NOT contain: `undefined`、`not enough arguments`、`FAIL`

- [ ] **Step 4: 提交**
Run: `git add cnvd_skills/vul_list.go && git commit -m "$(cat <<'EOF'
feat(list): thread CaptchaSolver through VulList main flow via WithConfig calls

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"`

---

### Task 3: 列表搜索/筛选能力 — 新增 VulListQuery 与 RequestVulListByQuery

**Depends on:** Task 2
**Files:**
- Create: `cnvd_skills/vul_list_query.go`
- Create: `cnvd_skills/vul_list_query_test.go`

- [ ] **Step 1: 创建 vul_list_query.go — 定义查询结构体与 URL 构造**

```go
package cnvd_skills

import (
	"context"
	"fmt"
	"net/url"
)

// VulListQuery 封装 CNVD 列表页的检索条件。
// 字段名对应 CNVD 真实列表页表单字段（已探测确认）。
// 零值字段不拼入查询，按 CNVD 默认行为处理。
type VulListQuery struct {
	// Keyword 关键词，匹配标题/描述等
	Keyword string

	// KeywordFlag 关键词逻辑：0=与(AND)，1=或(OR)，默认 0
	KeywordFlag int

	// StartDate 起始公开日期，格式 2006-01-02
	StartDate string

	// EndDate 截止公开日期，格式 2006-01-02
	Endate string

	// CnvdID 按 CNVD-ID 检索
	CnvdID string

	// CnvdIDFlag CNVD-ID 逻辑：0=与，1=或，默认 0
	CnvdIDFlag int

	// CategoryId 漏洞类别 ID（CNVD 内部编号）
	CategoryId string

	// ManufacturerId 厂商 ID（CNVD 内部编号）
	ManufacturerId string

	// Serverity 危害级别 ID
	Serverity string

	// ReferenceScope 参考编号范围：-1=无,1=CVE,2=BID,3=其他
	ReferenceScope int

	// Order 排序方式
	Order string

	// NumPerPage 每页条数，0 时用默认 10
	NumPerPage int
}

// buildQueryURL 构造 CNVD 列表页查询 URL。
// offset 从 0 开始。非空字段拼入 query string。
func (q *VulListQuery) buildQueryURL(offset int) string {
	v := url.Values{}
	v.Set("numPerPage", itoaOrDefault(q.NumPerPage, 10))
	v.Set("offset", fmt.Sprintf("%d", offset))
	v.Set("max", itoaOrDefault(q.NumPerPage, 10))
	if q.Keyword != "" {
		v.Set("keyword", q.Keyword)
		v.Set("keywordFlag", fmt.Sprintf("%d", q.KeywordFlag))
	}
	if q.StartDate != "" {
		v.Set("startDate", q.StartDate)
	}
	if q.Endate != "" {
		v.Set("endDate", q.Endate)
	}
	if q.CnvdID != "" {
		v.Set("cnvdId", q.CnvdID)
		v.Set("cnvdIdFlag", fmt.Sprintf("%d", q.CnvdIDFlag))
	}
	if q.CategoryId != "" {
		v.Set("categoryId", q.CategoryId)
	}
	if q.ManufacturerId != "" {
		v.Set("manufacturerId", q.ManufacturerId)
	}
	if q.Serverity != "" {
		v.Set("serverity", q.Serverity)
		v.Set("serverityIdStr", q.Serverity)
	}
	if q.ReferenceScope != 0 {
		v.Set("referenceScope", fmt.Sprintf("%d", q.ReferenceScope))
	}
	if q.Order != "" {
		v.Set("order", q.Order)
	}
	return "https://www.cnvd.org.cn/flaw/list?" + v.Encode()
}

// itoaOrDefault 把 n 转字符串，n<=0 返回 defVal。
func itoaOrDefault(n, defVal int) string {
	if n <= 0 {
		return fmt.Sprintf("%d", defVal)
	}
	return fmt.Sprintf("%d", n)
}

// RequestVulListByQuery 按检索条件抓取列表页并解析。
// offset 从 0 开始。内部走 requestWithRetry。
func (x *CnvdSkills) RequestVulListByQuery(ctx context.Context, query VulListQuery, offset int, proxyProvider ProxyProvider) (*VulList, error) {
	return x.RequestVulListByQueryWithConfig(ctx, query, offset, proxyProvider, nil)
}

// RequestVulListByQueryWithConfig 同 RequestVulListByQuery，但接收 config，
// 可传入 CaptchaSolver 以通过加速乐验证码挑战。
func (x *CnvdSkills) RequestVulListByQueryWithConfig(ctx context.Context, query VulListQuery, offset int, proxyProvider ProxyProvider, config *Config) (*VulList, error) {
	targetUrl := query.buildQueryURL(offset)
	body, err := x.requestWithRetry(ctx, proxyProvider, config, targetUrl)
	if err != nil {
		return nil, err
	}
	return x.ParseVulList(body)
}

// VulListWithQuery 按检索条件翻页抓取并逐条详情落盘。
// 与 VulList 区别：先按 query 过滤再翻页。query 为零值时等价于全量抓取。
func (x *CnvdSkills) VulListWithQuery(ctx context.Context, query VulListQuery, proxyProvider ProxyProvider, config *Config) error {
	if config == nil {
		config = DefaultConfig()
	}
	page := 1
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		offset := (page - 1) * config.NumPerPage
		list, err := x.RequestVulListByQueryWithConfig(ctx, query, offset, proxyProvider, config)
		if err != nil {
			if isProxyInvalid(err) {
				time.Sleep(time.Duration(config.ProxyRetryIntervalSeconds) * time.Second)
				continue
			}
			return err
		}

		if len(list.VulListItems) == 0 {
			fmt.Println("当前页无漏洞条目，抓取完成")
			return nil
		}

		for _, item := range list.VulListItems {
			if err := x.fetchAndSaveDetail(ctx, proxyProvider, config, item); err != nil {
				return err
			}
		}

		if list.TotalPage != nil && page >= *list.TotalPage {
			fmt.Printf("已抓取到最后一页（第 %d 页），抓取完成\n", page)
			return nil
		}
		page++
		time.Sleep(time.Duration(config.ListPageIntervalSeconds) * time.Second)
	}
}
```

注：`time` 需在 import。最终 import 块：

```go
import (
	"context"
	"fmt"
	"net/url"
	"time"
)
```

- [ ] **Step 2: 创建 vul_list_query_test.go — 离线验证 URL 拼装**

```go
package cnvd_skills

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVulListQuery_BuildQueryURL_KeywordOnly(t *testing.T) {
	q := VulListQuery{Keyword: "XStream"}
	got := q.buildQueryURL(0)
	u, err := url.Parse(got)
	assert.Nil(t, err)
	assert.Equal(t, "www.cnvd.org.cn", u.Host)
	assert.Equal(t, "/flaw/list", u.Path)
	assert.Equal(t, "XStream", u.Query().Get("keyword"))
	assert.Equal(t, "0", u.Query().Get("keywordFlag"))
	assert.Equal(t, "0", u.Query().Get("offset"))
	assert.Equal(t, "10", u.Query().Get("numPerPage"))
	// 未设字段不应出现
	assert.Empty(t, u.Query().Get("startDate"))
}

func TestVulListQuery_BuildQueryURL_DateRange(t *testing.T) {
	q := VulListQuery{StartDate: "2024-01-01", Endate: "2024-06-30", NumPerPage: 20}
	got := q.buildQueryURL(30)
	u, _ := url.Parse(got)
	assert.Equal(t, "2024-01-01", u.Query().Get("startDate"))
	assert.Equal(t, "2024-06-30", u.Query().Get("endDate"))
	assert.Equal(t, "30", u.Query().Get("offset"))
	assert.Equal(t, "20", u.Query().Get("numPerPage"))
}

func TestVulListQuery_BuildQueryURL_Empty(t *testing.T) {
	q := VulListQuery{}
	got := q.buildQueryURL(0)
	u, _ := url.Parse(got)
	assert.Equal(t, "0", u.Query().Get("offset"))
	assert.Equal(t, "10", u.Query().Get("numPerPage"))
	assert.Empty(t, u.Query().Get("keyword"))
}

func TestVulListQuery_BuildQueryURL_SeverityAndCategory(t *testing.T) {
	q := VulListQuery{Serverity: "3", CategoryId: "5", ReferenceScope: 1}
	got := q.buildQueryURL(0)
	u, _ := url.Parse(got)
	assert.Equal(t, "3", u.Query().Get("serverity"))
	assert.Equal(t, "3", u.Query().Get("serverityIdStr"))
	assert.Equal(t, "5", u.Query().Get("categoryId"))
	assert.Equal(t, "1", u.Query().Get("referenceScope"))
}

func TestItoaOrDefault(t *testing.T) {
	assert.Equal(t, "10", itoaOrDefault(0, 10))
	assert.Equal(t, "10", itoaOrDefault(-1, 10))
	assert.Equal(t, "20", itoaOrDefault(20, 10))
}
```

- [ ] **Step 3: 验证查询模块离线测试**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go test ./cnvd_skills/ -run "VulListQuery|ItoaOrDefault" -v -count=1`
Expected:
  - Exit code: 0
  - Output contains: `PASS`
  - Output contains: `5 passed` 或各测试 `--- PASS`

- [ ] **Step 4: 提交**
Run: `git add cnvd_skills/vul_list_query.go cnvd_skills/vul_list_query_test.go && git commit -m "$(cat <<'EOF'
feat(list): add VulListQuery + RequestVulListByQuery for keyword/date/vendor/severity search

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"`

---

### Task 4: 全量验证 + 真实集成测试 + 文档更新

**Depends on:** Task 1, Task 2, Task 3
**Files:**
- Create: `cnvd_skills/vul_list_query_real_test.go`
- Modify: `README.md`

- [ ] **Step 1: 创建真实集成测试 — 按关键词检索验证端到端**

```go
package cnvd_skills

import (
	"context"
	"testing"
	"time"

	"github.com/scagogogo/go-jsl"
	"github.com/stretchr/testify/assert"
)

// TestRequestVulListByQuery_Real 真实集成测试：按关键词 "XStream" 检索列表，
// 验证查询参数拼装 + jsl 三层 + 验证码全链路。CNVD 触发验证码时用
// CommandCaptchaSolver 调 gojsl/scripts/ddddocr_solver.py 自动识别。-short 跳过。
func TestRequestVulListByQuery_Real(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cfg := &Config{
		MaxRetry:              3,
		RequestTimeoutSeconds: 30,
		CaptchaSolver: jsl.CommandCaptchaSolver{
			Command: "python3",
			Args:    []string{"../gojsl/scripts/ddddocr_solver.py"},
		},
	}
	q := VulListQuery{Keyword: "XStream"}
	list, err := NewCnvdSkills().RequestVulListByQueryWithConfig(ctx, q, 0, FixedProxyProvider(""), cfg)
	if err != nil {
		t.Fatalf("真实检索失败（检查网络/CNVD/ddddocr）: %v", err)
	}
	assert.NotNil(t, list)
	assert.NotEmpty(t, list.VulListItems, "XStream 关键词应至少返回一条")
	for _, item := range list.VulListItems {
		assert.Regexp(t, `^/flaw/show/CNVD-\d{4}-\d+$`, item.Href)
	}
}
```

- [ ] **Step 2: 全量离线测试 + vet + build + panic 检查**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go vet ./... && go build ./... && go test ./cnvd_skills/ -short -v -count=1 2>&1 | tail -30 && grep -rn "panic(" cnvd_skills/ --include="*.go" | grep -v "_test.go" | grep -v "//" || echo NO_PANIC`
Expected:
  - Exit code: 0
  - 离线测试全 PASS、集成测试 SKIP
  - 输出 `NO_PANIC`

- [ ] **Step 3: 真实跑关键词检索集成测试 — 验证查询端到端打通**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go test ./cnvd_skills/ -run "TestRequestVulListByQuery_Real" -v -count=1 -timeout 400s`
Expected:
  - Exit code: 0 或明确失败诊断
  - 若 PASS：查询参数拼装 + jsl + 验证码 + 解析全链路打通
  - 若 FAIL：如实记录失败原因（如 CNVD 改版字段名变化、验证码识别失败）

- [ ] **Step 4: 更新 README — 补 WithConfig 补丁变体、VulListWithQuery、VulListQuery 字段说明**

文件: `README.md`（功能列表、用法、配置段补充）

补充内容要点：
- 功能列表「厂商补丁」行补 `RequestVulPatchByIDWithConfig` / `RequestVulPatchByURLWithConfig`
- 功能列表新增「列表检索」行：`RequestVulListByQuery[WithConfig]` + `VulListWithQuery`，按关键词/日期/厂商/级别定向检索
- 用法段补 VulListQuery 示例（关键词检索 + 日期范围）
- WithConfig API 变体段补 4 个补丁变体

- [ ] **Step 5: 提交**
Run: `git add cnvd_skills/vul_list_query_real_test.go README.md && git commit -m "$(cat <<'EOF'
docs+test: document VulListQuery search and WithConfig patch variants, add real search test

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"`

---

## 跨 Task 一致性说明

- **WithConfig 命名**：T1 的 `RequestVulPatchByIDWithConfig` / `RequestVulPatchByURLWithConfig` 与现有 `RequestVulDetailByIDWithConfig` / `RequestVulListByOffsetWithConfig` 命名风格一致
- **config 透传**：T2 的 `VulList` 内部改调 `RequestVulListByOffsetWithConfig` / `RequestVulDetailByURLWithConfig`，T3 的 `VulListWithQuery` 内部调 `RequestVulListByQueryWithConfig` / `fetchAndSaveDetail`，签名与现有 `*WithConfig` 一致
- **VulListQuery 字段**：T3 定义于 `vul_list_query.go`，字段名（Keyword/StartDate/Endate/CategoryId/ManufacturerId/Serverity/ReferenceScope）对应 CNVD 真实表单字段名，T3 测试与 T4 真实测试均引用同名
- **buildQueryURL / itoaOrDefault**：T3 定义于 `vul_list_query.go`，测试引用同名
- **Endate 字段命名注意**：CNVD 表单字段为 `endDate`，但 Go 风格避免与内置冲突用 `Endate`，buildQueryURL 内 Set("endDate", q.Endate) 完成映射，测试断言 `endDate` query key
