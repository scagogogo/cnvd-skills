package cnvd_skills

// Config 抓取配置，控制输出路径、分页大小、请求节奏。
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
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		OutputPath:               "data/test.jsonl",
		NumPerPage:               10,
		ListPageIntervalSeconds:  3,
		DetailIntervalSeconds:    3,
		ProxyRetryIntervalSeconds: 3,
	}
}
