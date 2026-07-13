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

// isProxyInvalid 判定是否把此错误归类于代理错误（应换 IP 重试）。
// 覆盖：TCP 读错误、EOF、代理连接拒绝、context 超时/取消。
func isProxyInvalid(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "read tcp "):
		return true
	case strings.HasSuffix(msg, "unexpected EOF"):
		return true
	case strings.Contains(msg, "proxyconnect"):
		return true
	case strings.Contains(msg, "EOF"):
		return true
	case strings.Contains(msg, "connection refused"):
		return true
	case strings.Contains(msg, "i/o timeout"):
		return true
	case strings.Contains(msg, "context deadline exceeded"):
		return true
	}
	return false
}

// ------------------------------------------------ ---------------------------------------------------------------------
