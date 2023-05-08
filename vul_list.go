package cnvd_crawler

import (
	"fmt"
	jsl_sdk "github.com/JSREP/go-jsl-sdk"
	"github.com/PuerkitoBio/goquery"
	"strconv"
	"strings"
	"time"
)

// ------------------------------------------------ ---------------------------------------------------------------------

type VulList struct {
	Page int
	VulListItems []*VulListItem
}

type VulListItem struct {
	Title string
	Href string
}

// ------------------------------------------------ ---------------------------------------------------------------------

// VulList TODO 代理IP
func (x *CnvdCrawler) VulList() error {
	offset := 0
	for  {
		targetUrl := fmt.Sprintf("https://www.cnvd.org.cn/flaw/list?numPerPage=10&offset=%d&max=10", offset)
		response, err := jsl_sdk.NewJslClient().Get(targetUrl)
		if err != nil {
			return err
		}

		list, err := x.ParseVulList(response.String())
		if err != nil {
			return err
		}
		fmt.Println(list)

		offset += 10
		time.Sleep(time.Second*3)
	}
}

func (x *CnvdCrawler) ParseVulList(responseBody string) (*VulList, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(responseBody))
	if err != nil {
		return nil, err
	}
	vulList := &VulList{}

	// 页码
	pageNumStr := document.Find("span.currentStep").Text()
	if pageNumStr != "" {
		pageNum, err := strconv.Atoi(pageNumStr)
		if err == nil {
			vulList.Page = pageNum
		}
	}

	// 列表
	document.Find("a[href^='/flaw/show/CNVD-']").Each(func(i int, selection *goquery.Selection) {
		title, _ := selection.Attr("title")
		href, _ := selection.Attr("href")
		vulList.VulListItems = append(vulList.VulListItems, &VulListItem {
			Title: title,
			Href: href,
		})
	})
	return vulList, nil
}

// ------------------------------------------------ ---------------------------------------------------------------------
