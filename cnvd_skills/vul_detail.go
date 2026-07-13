package cnvd_skills

import (
	"context"
	jsl_sdk "github.com/JSREP/go-jsl-sdk"
	"github.com/PuerkitoBio/goquery"
	"strings"
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

	// 收录时间
	RecordTimeStr string

	// 更新时间
	UpdateTimeStr string

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

// RequestVulDetailByID 根据CNVD漏洞ID请求漏洞信息，比如： CNVD-2021-67823
func (x *CnvdSkills) RequestVulDetailByID(ctx context.Context, cnvd string, proxyProvider ProxyProvider) (*VulDetail, error) {
	targetUrl := "https://www.cnvd.org.cn/flaw/show/" + cnvd
	return x.RequestVulDetailByURL(ctx, targetUrl, proxyProvider)
}

// RequestVulDetailByURL 根据详情页URL请求并解析漏洞信息。
// ParseVulDetail 出错时返回 (nil, err) 而非 (detail, err)，避免调用方对 nil detail 解引用。
func (x *CnvdSkills) RequestVulDetailByURL(ctx context.Context, detailPageURL string, proxyProvider ProxyProvider) (*VulDetail, error) {
	proxy, err := proxyProvider()
	if err != nil {
		return nil, err
	}
	response, err := jsl_sdk.NewJslClient(&jsl_sdk.ClientOptions{
		Proxy: proxy,
	}).Get(detailPageURL)
	if err != nil {
		return nil, err
	}
	detail, err := x.ParseVulDetail(response.String())
	if err != nil {
		return nil, err // 关键：返回 nil detail，不再返回可能为 nil 的 detail
	}
	detail.URL = detailPageURL
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
		case "收录时间":
			detail.RecordTimeStr = valueText
		case "更新时间":
			detail.UpdateTimeStr = valueText
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

// ------------------------------------------------ ---------------------------------------------------------------------
