package cnvd_skills

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCnvdSkills_ParseVulList_Offline(t *testing.T) {
	body, err := os.ReadFile("testdata/vul_list_page1.html")
	assert.Nil(t, err)

	list, err := NewCnvdSkills().ParseVulList(string(body))
	assert.Nil(t, err)
	assert.NotNil(t, list)

	assert.NotNil(t, list.Page)
	assert.Equal(t, 1, *list.Page)
	assert.NotNil(t, list.TotalPage)
	assert.Equal(t, 200, *list.TotalPage)
	assert.NotNil(t, list.TotalRecord)
	assert.Equal(t, 2000, *list.TotalRecord)

	assert.Len(t, list.VulListItems, 3)
	assert.Equal(t, "/flaw/show/CNVD-2021-67823", list.VulListItems[0].Href)
	assert.Equal(t, "XStream任意代码执行漏洞", list.VulListItems[0].Title)
}

func TestCnvdSkills_ParseVulList_EmptyBody(t *testing.T) {
	list, err := NewCnvdSkills().ParseVulList("")
	assert.Nil(t, err)
	assert.NotNil(t, list)
	assert.Nil(t, list.Page)
	assert.Empty(t, list.VulListItems)
}

func TestCnvdSkills_ParseVulList_NoPagination(t *testing.T) {
	list, err := NewCnvdSkills().ParseVulList(`<a href="/flaw/show/CNVD-2021-00001" title="t">t</a>`)
	assert.Nil(t, err)
	assert.Len(t, list.VulListItems, 1)
	assert.Nil(t, list.TotalPage)
}

// TestCnvdSkills_ParseVulList_StepPaging 真实 CNVD 列表页用 a.step 分页
// （无 totalPage class），应从 a.step 文本取最大值作为总页数。
func TestCnvdSkills_ParseVulList_StepPaging(t *testing.T) {
	body, err := os.ReadFile("testdata/vul_list_with_step_paging.html")
	assert.Nil(t, err)

	list, err := NewCnvdSkills().ParseVulList(string(body))
	assert.Nil(t, err)
	assert.NotNil(t, list.Page)
	assert.Equal(t, 1, *list.Page)
	// 应从 a.step 取最大值 1000 作为总页数
	assert.NotNil(t, list.TotalPage)
	assert.Equal(t, 1000, *list.TotalPage)
	assert.Len(t, list.VulListItems, 2)
	assert.Equal(t, "/flaw/show/CNVD-2024-10001", list.VulListItems[0].Href)
}
