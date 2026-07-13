package main

import (
	"context"
	"fmt"
	"github.com/scagogogo/cnvd-skills/cnvd_skills"
)

func main() {
	ctx := context.Background()
	err := cnvd_skills.NewCnvdSkills().VulList(ctx, cnvd_skills.PinYiProxyProvider, cnvd_skills.DefaultConfig())
	if err != nil {
		fmt.Println("抓取出错： " + err.Error())
	} else {
		fmt.Println("正常退出！")
	}
}
