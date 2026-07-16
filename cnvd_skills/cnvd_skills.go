package cnvd_skills

// CnvdSkills 是 CNVD 网站抓取的入口。
// 持有一个默认的加速乐客户端实例 jslClient，用于无 config 的简单请求场景；
// 带 config 的请求会在 requestWithRetry 内按请求派生独立客户端（并发安全）。
type CnvdSkills struct {
	jslClient *JslClient
}

// NewCnvdSkills 构造一个 CnvdSkills，默认直连、不限时、不配验证码识别器。
func NewCnvdSkills() *CnvdSkills {
	return &CnvdSkills{
		jslClient: NewJslClient("", 0, nil),
	}
}

// JslClient 返回 CnvdSkills 持有的默认加速乐客户端实例（只读引用）。
// 外部可用它直接访问任意被加速乐保护的 URL。
func (x *CnvdSkills) JslClient() *JslClient {
	return x.jslClient
}
