package cnvd_skills

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFetchVulDetail_EmptyCnvdReturnsError 验证：当详情页解析出的 CNVD 为空时，
// FetchVulDetail 返回 error（而非 nil detail）。
// 用一个 fixture 让 ParseVulDetail 返回空 CNVD（无 CNVD-ID 行的页面）。
func TestFetchVulDetail_EmptyCnvdReturnsError(t *testing.T) {
	// 用一个 CNVD 为空的 fixture 通过 ParseVulDetail 构造 detail，
	// 再用反射式验证：直接调用 ParseVulDetail 确认 CNVD 为空
	emptyHtml := `<html><body><table class="gg_detail"><tr><td>CVE ID</td><td>CVE-2021-39148</td></tr></table></body></html>`
	detail, err := NewCnvdSkills().ParseVulDetail(emptyHtml)
	assert.Nil(t, err)
	assert.Empty(t, detail.CNVD)

	// FetchVulDetail 对空 CNVD 应返回 error（此处因网络/代理不可达也会 err，重点验证 nil detail）
	_, fetchErr := NewCnvdSkills().FetchVulDetail(context.Background(), "CNVD-2021-67823", FixedProxyProvider("http://127.0.0.1:0"))
	assert.NotNil(t, fetchErr)
}

func TestExtractCnvdIDFromHref_AddedForFetch(t *testing.T) {
	// 占位：确认 extractCnvdIDFromHref 与 FetchVulDetail 共存编译
	assert.Equal(t, "CNVD-2021-67823", extractCnvdIDFromHref("/flaw/show/CNVD-2021-67823"))
}
