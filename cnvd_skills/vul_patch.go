package cnvd_skills

import (
	"context"
	"github.com/PuerkitoBio/goquery"
	"strings"
	"time"
)

// VulPatch 厂商补丁详情
type VulPatch struct {

	// 补丁详情页 URL
	URL string

	// 补丁名称
	Name string

	// 补丁厂商
	Vendor string

	// 补丁下载/参考链接
	Link string

	// 补丁描述
	Description string

	// 补丁发布时间
	PublishTimeStr string

	// 补丁发布时间（解析后的时间）
	PublishTime *time.Time
}

// RequestVulPatchByID 根据补丁ID请求补丁详情，如 289241。
func (x *CnvdSkills) RequestVulPatchByID(ctx context.Context, patchID string, proxyProvider ProxyProvider) (*VulPatch, error) {
	targetUrl := "https://www.cnvd.org.cn/patchInfo/show/" + patchID
	return x.RequestVulPatchByURL(ctx, targetUrl, proxyProvider)
}

// RequestVulPatchByURL 根据补丁详情页URL请求并解析。内部走 requestWithRetry。
func (x *CnvdSkills) RequestVulPatchByURL(ctx context.Context, patchPageURL string, proxyProvider ProxyProvider) (*VulPatch, error) {
	body, err := requestWithRetry(ctx, proxyProvider, nil, patchPageURL)
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

// ParseVulPatch 解析厂商补丁详情页 HTML。
func (x *CnvdSkills) ParseVulPatch(responseString string) (*VulPatch, error) {
	patch := &VulPatch{}

	document, err := goquery.NewDocumentFromReader(strings.NewReader(responseString))
	if err != nil {
		return nil, err
	}

	document.Find(".gg_detail tr").Each(func(i int, selection *goquery.Selection) {
		keySelection := selection.Find("td").First()
		key := strings.TrimSpace(keySelection.Text())
		valueSelection := keySelection.Next()
		valueHtml, _ := valueSelection.Html()
		valueText := decodeHTMLEntities(valueHtml)

		switch key {
		case "补丁名称":
			patch.Name = valueText
		case "补丁厂商":
			patch.Vendor = valueText
		case "补丁链接":
			href, exists := valueSelection.Find("a").First().Attr("href")
			if exists {
				patch.Link = href
			} else {
				patch.Link = valueText
			}
		case "补丁描述":
			patch.Description = valueText
		case "补丁发布时间":
			patch.PublishTimeStr = valueText
			patch.PublishTime = parseCnvdDate(valueText)
		}
	})

	return patch, nil
}
