package cnvd_skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"github.com/PuerkitoBio/goquery"
	"github.com/golang-infrastructure/go-pointer"
	"os"
	"strconv"
	"strings"
	"time"
)

// ------------------------------------------------ ---------------------------------------------------------------------

// VulList 漏洞列表
type VulList struct {

	// 当前处在第几页
	Page *int

	// 总页数（用于判断何时停止翻页）
	TotalPage *int

	// 总记录数
	TotalRecord *int

	// 当前页列出的漏洞都有哪些
	VulListItems []*VulListItem
}

// VulListItem 列表页的一条漏洞
type VulListItem struct {

	// 漏洞的标题
	Title string

	// 相应页的链接
	Href string
}

// ------------------------------------------------ ---------------------------------------------------------------------

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
				jitterSleep(ctx, config.ProxyRetryIntervalSeconds, config.Jitter)
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
		jitterSleep(ctx, config.ListPageIntervalSeconds, config.Jitter)
	}
}

// loadExistingCnvdIDs 读取输出文件，返回已抓取过的 CNVD-ID 集合。
// 文件不存在时返回空集合。每行是一条 VulDetail 的 JSON，含 CNVD 字段。
func loadExistingCnvdIDs(outputPath string) map[string]struct{} {
	existed := make(map[string]struct{})
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return existed
	}
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var record struct {
			CNVD string `json:"CNVD"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		if record.CNVD != "" {
			existed[record.CNVD] = struct{}{}
		}
	}
	return existed
}

// fetchAndSaveDetail 抓取单条漏洞详情并追加写入输出文件。
// 代理失效时换 IP 重试，CNVD 为空（解析异常）时重试，其余错误上抛。
// 当 config.EnableDedup 为 true 时，跳过输出文件中已存在的 CNVD-ID。
func (x *CnvdSkills) fetchAndSaveDetail(ctx context.Context, proxyProvider ProxyProvider, config *Config, item *VulListItem) error {
	// 去重：若该漏洞的 CNVD-ID 已在输出文件中，跳过
	if config != nil && config.EnableDedup {
		existed := loadExistingCnvdIDs(config.OutputPath)
		if cnvdID := extractCnvdIDFromHref(item.Href); cnvdID != "" {
			if _, ok := existed[cnvdID]; ok {
				fmt.Printf("已存在，跳过： %s\n", cnvdID)
				return nil
			}
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fmt.Println("开始请求： " + item.Title)
		detail, err := x.RequestVulDetailByURLWithConfig(ctx, "https://www.cnvd.org.cn"+item.Href, proxyProvider, config)
		if err != nil {
			if isProxyInvalid(err) {
				jitterSleep(ctx, config.ProxyRetryIntervalSeconds, config.Jitter)
				continue
			}
			return err
		}
		if detail.CNVD == "" {
			fmt.Println(item.Href + ", 抓取错误，重新抓取...")
			continue
		}

		marshal, err := json.Marshal(detail)
		if err != nil {
			return fmt.Errorf("marshal detail failed: %w", err)
		}
		marshal = append(marshal, '\n')

		if err := os.MkdirAll(parentDir(config.OutputPath), os.ModePerm); err != nil {
			return err
		}
		file, err := os.OpenFile(config.OutputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
		if err != nil {
			return err
		}
		if _, err := file.Write(marshal); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		jitterSleep(ctx, config.DetailIntervalSeconds, config.Jitter)
		return nil
	}
}

// parentDir 返回路径的父目录，用于创建输出目录。
func parentDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				return "/"
			}
			return path[:i]
		}
	}
	return "."
}

// globalRand 用于 jitterSleep 的随机抖动。Go 1.18 包级 rand 不会自动 seed，
// 显式用时间戳播种，避免每次进程启动产生固定序列。
var globalRand = rand.New(rand.NewSource(time.Now().UnixNano()))

// jitterSleep 按 config.Jitter 把 baseSeconds 随机化后休眠，ctx 感知。
// Jitter=0 时固定休眠 baseSeconds；Jitter=0.5 时休眠时长在 [base*(1-0.5), base*(1+0.5)] 范围。
func jitterSleep(ctx context.Context, baseSeconds int, jitter float64) {
	if baseSeconds <= 0 {
		return
	}
	d := time.Duration(baseSeconds) * time.Second
	if jitter > 0 {
		span := float64(d) * jitter
		offset := time.Duration(globalRand.Float64()*2*span) - time.Duration(span)
		d = d + offset
		if d < 0 {
			d = 0
		}
	}
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// extractCnvdIDFromHref 从列表项相对链接提取 CNVD-ID。
// 输入如 /flaw/show/CNVD-2021-67823，返回 CNVD-2021-67823。
// 无法提取时返回空串。
func extractCnvdIDFromHref(href string) string {
	href = strings.TrimSpace(href)
	idx := strings.Index(href, "CNVD-")
	if idx < 0 {
		return ""
	}
	return href[idx:]
}

// RequestVulListByOffset 请求指定偏移量的漏洞列表页并解析。
// offset 从 0 开始，每页 10 条。内部走 requestWithRetry。
func (x *CnvdSkills) RequestVulListByOffset(ctx context.Context, offset int, proxyProvider ProxyProvider) (*VulList, error) {
	return x.RequestVulListByOffsetWithConfig(ctx, offset, proxyProvider, nil)
}

// RequestVulListByOffsetWithConfig 同 RequestVulListByOffset，但接收 config，
// 可传入 CaptchaSolver 以通过加速乐验证码挑战。
func (x *CnvdSkills) RequestVulListByOffsetWithConfig(ctx context.Context, offset int, proxyProvider ProxyProvider, config *Config) (*VulList, error) {
	targetUrl := fmt.Sprintf("https://www.cnvd.org.cn/flaw/list?numPerPage=10&offset=%d&max=10", offset)
	body, err := x.requestWithRetry(ctx, proxyProvider, config, targetUrl)
	if err != nil {
		return nil, err
	}
	return x.ParseVulList(body)
}

// ParseVulList 解析漏洞列表页 HTML。
// 解析当前页码、总页数、总记录数及当前页漏洞条目。
func (x *CnvdSkills) ParseVulList(responseBody string) (*VulList, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(responseBody))
	if err != nil {
		return nil, err
	}
	vulList := &VulList{}

	// 当前页码
	pageNumStr := strings.TrimSpace(document.Find("span.currentStep").Text())
	if pageNumStr != "" {
		if pageNum, err := strconv.Atoi(pageNumStr); err == nil {
			vulList.Page = pointer.ToPointer(pageNum)
		}
	}

	// 总页数：优先取 span.totalPage；若无则从分页链接 a.step 文本取最大值
	// （CNVD 真实列表页分页结构为 span.currentStep + a.step，最后一个 a.step 即总页数）。
	totalPageStr := strings.TrimSpace(document.Find("span.totalPage").Text())
	if totalPageStr != "" {
		if totalPage, err := strconv.Atoi(totalPageStr); err == nil {
			vulList.TotalPage = pointer.ToPointer(totalPage)
		}
	}
	if vulList.TotalPage == nil {
		maxPage := 0
		document.Find("a.step").Each(func(i int, s *goquery.Selection) {
			if n, err := strconv.Atoi(strings.TrimSpace(s.Text())); err == nil && n > maxPage {
				maxPage = n
			}
		})
		if maxPage > 0 {
			vulList.TotalPage = pointer.ToPointer(maxPage)
		}
	}

	// 总记录数（部分页面有，无则留空）
	totalRecordStr := strings.TrimSpace(document.Find("span.totalRecord").Text())
	if totalRecordStr != "" {
		if totalRecord, err := strconv.Atoi(totalRecordStr); err == nil {
			vulList.TotalRecord = pointer.ToPointer(totalRecord)
		}
	}

	// 列表条目
	document.Find("a[href^='/flaw/show/CNVD-']").Each(func(i int, selection *goquery.Selection) {
		title, _ := selection.Attr("title")
		href, _ := selection.Attr("href")
		vulList.VulListItems = append(vulList.VulListItems, &VulListItem{
			Title: strings.TrimSpace(title),
			Href:  strings.TrimSpace(href),
		})
	})
	return vulList, nil
}

// ------------------------------------------------ ---------------------------------------------------------------------
