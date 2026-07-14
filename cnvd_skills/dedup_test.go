package cnvd_skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractCnvdIDFromHref(t *testing.T) {
	// Happy Path
	assert.Equal(t, "CNVD-2021-67823", extractCnvdIDFromHref("/flaw/show/CNVD-2021-67823"))
	assert.Equal(t, "CNVD-2021-67823", extractCnvdIDFromHref("  /flaw/show/CNVD-2021-67823  "))

	// Edge Case：无 CNVD 前缀
	assert.Equal(t, "", extractCnvdIDFromHref("/some/other/path"))
	assert.Equal(t, "", extractCnvdIDFromHref(""))
}

func TestLoadExistingCnvdIDs_FileNotExist(t *testing.T) {
	// Edge Case：文件不存在返回空集合
	m := loadExistingCnvdIDs("/nonexistent/path/to/file.jsonl")
	assert.NotNil(t, m)
	assert.Empty(t, m)
}

func TestLoadExistingCnvdIDs_ReadsIDs(t *testing.T) {
	// Happy Path：从 jsonl 读取 CNVD 集合
	dir := t.TempDir()
	path := filepath.Join(dir, "data.jsonl")
	content := `{"CNVD":"CNVD-2021-67823","CVE":"x"}
{"CNVD":"CNVD-2021-67822"}
{"CNVD":""}
{"not json"}
`
	err := os.WriteFile(path, []byte(content), 0644)
	assert.Nil(t, err)

	m := loadExistingCnvdIDs(path)
	assert.Len(t, m, 2)
	assert.Contains(t, m, "CNVD-2021-67823")
	assert.Contains(t, m, "CNVD-2021-67822")
}

func TestFetchAndSaveDetail_DedupSkipsExisting(t *testing.T) {
	// 验证去重行为：已存在的 CNVD 会被跳过（不发起请求）
	dir := t.TempDir()
	path := filepath.Join(dir, "data.jsonl")
	// 预置一个已抓的 CNVD
	err := os.WriteFile(path, []byte(`{"CNVD":"CNVD-2021-67823"}`+"\n"), 0644)
	assert.Nil(t, err)

	cfg := &Config{
		OutputPath:  path,
		EnableDedup: true,
	}
	// 同一 CNVD 的列表项 → 应被跳过（返回 nil，不写文件、不请求）
	item := &VulListItem{
		Title: "XStream任意代码执行漏洞",
		Href:  "/flaw/show/CNVD-2021-67823",
	}
	err = NewCnvdSkills().fetchAndSaveDetail(nil, FixedProxyProvider("http://127.0.0.1:0"), cfg, item)
	assert.Nil(t, err)

	// 文件内容应仍只有原本那 1 行（未追加）
	data, err := os.ReadFile(path)
	assert.Nil(t, err)
	assert.Equal(t, `{"CNVD":"CNVD-2021-67823"}`+"\n", string(data))
}
