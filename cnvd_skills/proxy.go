package cnvd_skills

import (
	"context"
	"fmt"
	"github.com/crawler-go-go-go/go-requests"
	"strings"
)

// ------------------------------------------------ ---------------------------------------------------------------------

// ProxyProvider 用于生产代理IP供内容使用
type ProxyProvider func() (string, error)

// ------------------------------------------------ ---------------------------------------------------------------------

// PinYiProxyProvider 品易的代理IP
func PinYiProxyProvider() (string, error) {
	targetUrl := "http://zltiqu.pyhttp.taolop.com/getip?count=1&neek=75958&type=2&yys=0&port=2&sb=&mr=1&sep=0&ts=1&time=2"
	json, err := requests.GetJson[*ProxyResponse](context.Background(), targetUrl)
	if err != nil {
		return "", err
	}
	if len(json.Data) != 1 {
		return "", fmt.Errorf("failed: %#v", json)
	}
	return fmt.Sprintf("http://%s:%d", json.Data[0].IP, json.Data[0].Port), nil
}

type ProxyResponse struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
	Data    []struct {
		IP         string `json:"ip"`
		Port       int    `json:"port"`
		ExpireTime string `json:"expire_time"`
		City       string `json:"city"`
		Isp        string `json:"isp"`
	} `json:"data"`
}

// ------------------------------------------------ ---------------------------------------------------------------------

// FixedProxyProvider 始终使用一个固定的IP
func FixedProxyProvider(proxy string) ProxyProvider {
	return func() (string, error) {
		return proxy, nil
	}
}

// ------------------------------------------------ ---------------------------------------------------------------------

// 判定是否把此错误归类于代理错误
func isProxyInvalid(err error) bool {
	return strings.HasPrefix(err.Error(), "read tcp ") ||
		strings.HasSuffix(err.Error(), "unexpected EOF") ||
		strings.Contains(err.Error(), "proxyconnect")
}

// ------------------------------------------------ ---------------------------------------------------------------------
