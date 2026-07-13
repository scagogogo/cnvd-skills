package cnvd_skills

import (
	"context"
	"encoding/json"
	"fmt"
	jsl_sdk "github.com/JSREP/go-jsl-sdk"
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

func (x *CnvdSkills) VulList(proxyProvider ProxyProvider) error {
	proxy, err := proxyProvider()
	if err != nil {
		return err
	}
	offset := 0
	for {

		// 请求列表页
		list, err := x.RequestVulListByOffset(context.Background(), offset, FixedProxyProvider(proxy))
		if err != nil {
			// TODO 连续N次错误时再切换代理
			if isProxyInvalid(err) {
				// 代理失效了，换个新的代理
				time.Sleep(time.Second * 3)
				proxy, err = proxyProvider()
				if err != nil {
					panic(err)
				}
				fmt.Println("切换新的代理IP： " + proxy)
				continue
			} else {
				panic(err)
			}
		}

		// 抓取详情页
		for _, item := range list.VulListItems {
			fmt.Println("开始请求： " + item.Title)
			for {
				detail, err := x.RequestVulDetailByURL(context.Background(), "https://www.cnvd.org.cn"+item.Href, FixedProxyProvider(proxy))
				if err != nil {
					if isProxyInvalid(err) {
						// 代理失效了，换个新的代理
						time.Sleep(time.Second * 3)
						proxy, err = proxyProvider()
						if err != nil {
							panic(err)
						}
						fmt.Println("切换新的代理IP： " + proxy)
						continue
					} else {
						panic(err)
					}
				}

				// 校验有效性
				if detail.CNVD == "" {
					fmt.Println(detail.URL + ", 抓取错误，重新抓取...")
					continue
				}

				marshal, err := json.Marshal(detail)
				if err != nil {
					fmt.Println(err.Error())
					continue
				}
				fmt.Println("抓取成功： " + string(marshal))

				marshal = append(marshal, []byte("\n")...)
				file, err := os.OpenFile("data/test.jsonl", os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
				if err != nil {
					panic(err)
				}
				_, err = file.Write(marshal)
				if err != nil {
					panic(err)
				}
				err = file.Close()
				if err != nil {
					panic(err)
				}
				break
			}
		}

		offset += 10
		time.Sleep(time.Second * 3)
	}
}

// RequestVulListByOffset 请求指定偏移量的漏洞列表页并解析。
// offset 从 0 开始，每页 10 条。
func (x *CnvdSkills) RequestVulListByOffset(ctx context.Context, offset int, proxyProvider ProxyProvider) (*VulList, error) {
	proxy, err := proxyProvider()
	if err != nil {
		return nil, err
	}
	targetUrl := fmt.Sprintf("https://www.cnvd.org.cn/flaw/list?numPerPage=10&offset=%d&max=10", offset)
	response, err := jsl_sdk.NewJslClient(&jsl_sdk.ClientOptions{
		Proxy: proxy,
	}).Get(targetUrl)
	if err != nil {
		return nil, err
	}
	return x.ParseVulList(response.String())
}

func (x *CnvdSkills) ParseVulList(responseBody string) (*VulList, error) {
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
			vulList.Page = pointer.ToPointer(pageNum)
		}
	}

	// 列表
	document.Find("a[href^='/flaw/show/CNVD-']").Each(func(i int, selection *goquery.Selection) {
		title, _ := selection.Attr("title")
		href, _ := selection.Attr("href")
		vulList.VulListItems = append(vulList.VulListItems, &VulListItem{
			Title: title,
			Href:  href,
		})
	})
	return vulList, nil
}

// ------------------------------------------------ ---------------------------------------------------------------------
