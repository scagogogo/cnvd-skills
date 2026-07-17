package cnvd_skills

import "github.com/scagogogo/go-jsl"

// Config 抓取配置，控制输出路径、分页大小、请求节奏、重试与去重。
type Config struct {

	// 抓取结果输出文件路径，默认 data/test.jsonl
	OutputPath string

	// 每页漏洞条目数，默认 10（CNVD 列表页固定为 10）
	NumPerPage int

	// 列表翻页之间的休眠时长（秒），默认 3
	ListPageIntervalSeconds int

	// 详情页请求之间的休眠时长（秒），默认 3
	DetailIntervalSeconds int

	// 代理失效后重试前的休眠时长（秒），默认 3
	ProxyRetryIntervalSeconds int

	// 单次请求最大重试次数（0=不重试，直接返回错误），默认 3
	MaxRetry int

	// 单次请求超时（秒，0=不设超时），默认 30
	RequestTimeoutSeconds int

	// 是否对输出文件按 CNVD-ID 去重，默认 true
	EnableDedup bool

	// 验证码识别器。CNVD 触发图片验证码挑战时用于自动通过：
	// 配置后库自动取图→识别→提交→放行刷新；不配置则遇验证码返回 jsl.ErrCaptchaRequired。
	// 内置实现见 go-jsl 包（jsl.CommandCaptchaSolver 等）。
	CaptchaSolver jsl.CaptchaSolver
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		OutputPath:                "data/test.jsonl",
		NumPerPage:                10,
		ListPageIntervalSeconds:   3,
		DetailIntervalSeconds:     3,
		ProxyRetryIntervalSeconds: 3,
		MaxRetry:                  3,
		RequestTimeoutSeconds:     30,
		EnableDedup:               true,
	}
}
