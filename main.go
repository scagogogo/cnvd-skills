package main

import (
	"fmt"
	"github.com/scagogogo/cnvd-skills/cnvd_skills"
)

func main() {
	err := cnvd_skills.NewCnvdSkills().VulList(cnvd_skills.PinYiProxyProvider)
	if err != nil {
		fmt.Println("抓取出错： " + err.Error())
	} else {
		fmt.Println("正常退出！")
	}
}
