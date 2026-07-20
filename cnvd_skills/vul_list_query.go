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

	// Endate 截止公开日期，格式 2006-01-02（CNVD 表单字段为 endDate，此处字段名避开与内置冲突，buildQueryURL 内映射为 endDate）
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
				jitterSleep(ctx, config.ProxyRetryIntervalSeconds, config.Jitter)
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
		jitterSleep(ctx, config.ListPageIntervalSeconds, config.Jitter)
	}
}
