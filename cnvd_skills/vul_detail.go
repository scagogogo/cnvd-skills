package cnvd_skills

import (
	"context"
	"fmt"
	jsl_sdk "github.com/JSREP/go-jsl-sdk"
	"github.com/PuerkitoBio/goquery"
	"strings"
	"time"
)

// ------------------------------------------------ ---------------------------------------------------------------------

// VulDetail 表示CNVD的漏洞信息
type VulDetail struct {
	URL string

	// CNVD漏洞编号
	CNVD string

	// 此漏洞对应的CVE编号
	CVE string

	// 公开日期
	PublishTimeStr string

	// 公开日期（解析后的时间）
	PublishTime *time.Time

	// 危害级别
	HazardLevel *HazardLevel

	// 影响产品
	Product string

	// 漏洞描述
	Description string

	// 漏洞类型
	Category string

	// 参考链接
	Reference string

	// 漏洞解决方案
	FixPlan string

	// 厂商补丁
	VendorPatchHTML string

	// 厂商补丁（结构化：链接+标题）
	VendorPatch *VendorPatch

	// 验证信息
	Validate string

	// 报送时间
	PostTimeStr string

	// 报送时间（解析后的时间）
	PostTime *time.Time

	// 收录时间
	RecordTimeStr string

	// 收录时间（解析后的时间）
	RecordTime *time.Time

	// 更新时间
	UpdateTimeStr string

	// 更新时间（解析后的时间）
	UpdateTime *time.Time

	// 漏洞附件
	AttachFile string
}

// HazardLevel 危害级别
type HazardLevel struct {

	// 评级，低中高严重之类的
	Level string

	// CNVD使用的评分系统是CVSS2
	CVSS2 string
}

// VendorPatch 厂商补丁
type VendorPatch struct {

	// 补丁详情页相对链接，如 /patchInfo/show/289241
	Href string

	// 补丁标题
	Title string
}

// ------------------------------------------------ ---------------------------------------------------------------------

// requestWithRetry 对单个 URL 执行 jsl_sdk.Get，失败时按 config 重试。
// 代理类错误（isProxyInvalid）会重新向 proxyProvider 取新 IP 重试；
// 非代理错误在 MaxRetry 次内重试，超出返回最后一次错误。
// config 为 nil 时退化为不重试的单次请求。全程响应 ctx 取消。
func requestWithRetry(ctx context.Context, proxyProvider ProxyProvider, config *Config, targetUrl string) (string, error) {
	var lastErr error
	proxy, err := proxyProvider()
	if err != nil {
		return "", err
	}
	maxRetry := 0
	if config != nil {
		maxRetry = config.MaxRetry
	}
	for attempt := 0; attempt <= maxRetry; attempt++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		response, getErr := jsl_sdk.NewJslClient(&jsl_sdk.ClientOptions{
			Proxy: proxy,
		}).Get(targetUrl)
		if getErr == nil {
			return response.String(), nil
		}
		lastErr = getErr

		// 代理错误：换新 IP，不计入普通重试次数衰减
		if isProxyInvalid(getErr) {
			if config != nil && config.ProxyRetryIntervalSeconds > 0 {
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(time.Duration(config.ProxyRetryIntervalSeconds) * time.Second):
				}
			}
			if newProxy, pErr := proxyProvider(); pErr == nil {
				proxy = newProxy
			}
			// 取不到新代理：沿用旧错误继续重试
			continue
		}

		// 非代理错误：等待后重试
		if config != nil && config.ProxyRetryIntervalSeconds > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(config.ProxyRetryIntervalSeconds) * time.Second):
			}
		}
	}
	return "", lastErr
}

// RequestVulDetailByID 根据CNVD漏洞ID请求漏洞信息，比如： CNVD-2021-67823
func (x *CnvdSkills) RequestVulDetailByID(ctx context.Context, cnvd string, proxyProvider ProxyProvider) (*VulDetail, error) {
	targetUrl := "https://www.cnvd.org.cn/flaw/show/" + cnvd
	return x.RequestVulDetailByURL(ctx, targetUrl, proxyProvider)
}

// RequestVulDetailByURL 根据详情页URL请求并解析漏洞信息。
// 内部走 requestWithRetry，支持重试与代理切换。
// ParseVulDetail 出错时返回 (nil, err)。
func (x *CnvdSkills) RequestVulDetailByURL(ctx context.Context, detailPageURL string, proxyProvider ProxyProvider) (*VulDetail, error) {
	body, err := requestWithRetry(ctx, proxyProvider, nil, detailPageURL)
	if err != nil {
		return nil, err
	}
	detail, err := x.ParseVulDetail(body)
	if err != nil {
		return nil, err
	}
	detail.URL = detailPageURL
	return detail, nil
}

// FetchVulDetail 按 CNVD-ID 抓取单条漏洞详情并返回结构化结果（不写文件）。
// 与 VulList 主流程的落盘行为解耦，供调用方按需取单条数据。
// 失败时返回 (nil, err)。CNVD 为空（解析异常）返回 error 提示。
func (x *CnvdSkills) FetchVulDetail(ctx context.Context, cnvd string, proxyProvider ProxyProvider) (*VulDetail, error) {
	detail, err := x.RequestVulDetailByID(ctx, cnvd, proxyProvider)
	if err != nil {
		return nil, err
	}
	if detail.CNVD == "" {
		return nil, fmt.Errorf("parsed detail for %s has empty CNVD-ID", cnvd)
	}
	return detail, nil
}

// ParseVulDetail 解析漏洞详情页 HTML，返回结构化的漏洞信息。
// 入参为详情页 HTML 字符串，不依赖网络，可用本地 fixture 测试。
func (x *CnvdSkills) ParseVulDetail(responseString string) (*VulDetail, error) {
	detail := &VulDetail{}

	document, err := goquery.NewDocumentFromReader(strings.NewReader(responseString))
	if err != nil {
		return nil, err
	}

	document.Find(".gg_detail tr").Each(func(i int, selection *goquery.Selection) {
		keySelection := selection.Find("td").First()
		key := strings.TrimSpace(keySelection.Text())
		valueSelection := keySelection.Next()
		// 用 Html() 取原始片段再用 goquery 解码实体，避免 &amp; &lt; 等脏数据
		valueHtml, _ := valueSelection.Html()
		valueText := decodeHTMLEntities(valueHtml)

		switch key {
		case "CNVD-ID":
			detail.CNVD = valueText
		case "CVE ID":
			detail.CVE = valueText
		case "公开日期":
			detail.PublishTimeStr = valueText
			detail.PublishTime = parseCnvdDate(valueText)
		case "危害级别":
			detail.HazardLevel = parseHazardLevel(valueSelection, valueText)
		case "影响产品":
			detail.Product = valueText
		case "漏洞描述":
			detail.Description = valueText
		case "漏洞类型":
			detail.Category = valueText
		case "参考链接":
			detail.Reference = valueText
		case "漏洞解决方案":
			detail.FixPlan = valueText
		case "厂商补丁":
			patchHref, _ := valueSelection.Find("a").First().Attr("href")
			patchTitle := valueSelection.Find("a").First().Text()
			detail.VendorPatchHTML = valueHtml
			if patchHref != "" {
				detail.VendorPatch = &VendorPatch{
					Href:  patchHref,
					Title: strings.TrimSpace(patchTitle),
				}
			}
		case "验证信息":
			detail.Validate = valueText
		case "报送时间":
			detail.PostTimeStr = valueText
			detail.PostTime = parseCnvdDate(valueText)
		case "收录时间":
			detail.RecordTimeStr = valueText
			detail.RecordTime = parseCnvdDate(valueText)
		case "更新时间":
			detail.UpdateTimeStr = valueText
			detail.UpdateTime = parseCnvdDate(valueText)
		case "漏洞附件":
			attachHref, _ := valueSelection.Find("a").First().Attr("href")
			if attachHref != "" {
				detail.AttachFile = attachHref
			} else {
				detail.AttachFile = valueText
			}
		}
	})

	return detail, nil
}

// parseHazardLevel 从「危害级别」单元格解析级别与 CVSS2 评分。
func parseHazardLevel(valueSelection *goquery.Selection, fallbackText string) *HazardLevel {
	level := strings.TrimSpace(valueSelection.Find("span, div, p").First().Text())
	if level == "" {
		parts := strings.SplitN(fallbackText, "(", 2)
		level = strings.TrimSpace(parts[0])
		if len(parts) == 2 {
			cvss2 := strings.TrimSuffix(strings.TrimSpace(parts[1]), ")")
			return &HazardLevel{Level: level, CVSS2: cvss2}
		}
		return &HazardLevel{Level: level}
	}
	cvss2 := ""
	if idx := strings.Index(fallbackText, "("); idx >= 0 {
		cvss2 = strings.TrimSuffix(strings.TrimSpace(fallbackText[idx+1:]), ")")
	}
	return &HazardLevel{Level: level, CVSS2: cvss2}
}

// decodeHTMLEntities 解码常见 HTML 实体并压缩多余空白。
func decodeHTMLEntities(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div>" + htmlStr + "</div>"))
	if err != nil {
		return strings.TrimSpace(htmlStr)
	}
	return strings.TrimSpace(doc.Text())
}

// parseCnvdDate 把 CNVD 日期字符串解析为 *time.Time。
// 依次尝试多种 layout，全部失败返回 nil（不报错，调用方用 Str 字段兜底）。
// 支持的 layout 覆盖 CNVD 常见格式：纯日期、日期+时间、斜杠分隔。
func parseCnvdDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02",
		"2006/01/02 15:04:05",
		"2006/01/02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return &t
		}
	}
	return nil
}

// ------------------------------------------------ ---------------------------------------------------------------------
