package main

import (
	"fmt"
	"github.com/scagogogo/cnvd-crawler/cnvd_crawler"
)

func main() {
	err := cnvd_crawler.NewCnvdCrawler().VulList(cnvd_crawler.PinYiProxyProvider)
	if err != nil {
		fmt.Println("抓取出错： " + err.Error())
	} else {
		fmt.Println("正常退出！")
	}
}
