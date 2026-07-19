package cnvd_skills

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVulListQuery_BuildQueryURL_KeywordOnly(t *testing.T) {
	q := VulListQuery{Keyword: "XStream"}
	got := q.buildQueryURL(0)
	u, err := url.Parse(got)
	assert.Nil(t, err)
	assert.Equal(t, "www.cnvd.org.cn", u.Host)
	assert.Equal(t, "/flaw/list", u.Path)
	assert.Equal(t, "XStream", u.Query().Get("keyword"))
	assert.Equal(t, "0", u.Query().Get("keywordFlag"))
	assert.Equal(t, "0", u.Query().Get("offset"))
	assert.Equal(t, "10", u.Query().Get("numPerPage"))
	// 未设字段不应出现
	assert.Empty(t, u.Query().Get("startDate"))
}

func TestVulListQuery_BuildQueryURL_DateRange(t *testing.T) {
	q := VulListQuery{StartDate: "2024-01-01", Endate: "2024-06-30", NumPerPage: 20}
	got := q.buildQueryURL(30)
	u, _ := url.Parse(got)
	assert.Equal(t, "2024-01-01", u.Query().Get("startDate"))
	assert.Equal(t, "2024-06-30", u.Query().Get("endDate"))
	assert.Equal(t, "30", u.Query().Get("offset"))
	assert.Equal(t, "20", u.Query().Get("numPerPage"))
}

func TestVulListQuery_BuildQueryURL_Empty(t *testing.T) {
	q := VulListQuery{}
	got := q.buildQueryURL(0)
	u, _ := url.Parse(got)
	assert.Equal(t, "0", u.Query().Get("offset"))
	assert.Equal(t, "10", u.Query().Get("numPerPage"))
	assert.Empty(t, u.Query().Get("keyword"))
}

func TestVulListQuery_BuildQueryURL_SeverityAndCategory(t *testing.T) {
	q := VulListQuery{Serverity: "3", CategoryId: "5", ReferenceScope: 1}
	got := q.buildQueryURL(0)
	u, _ := url.Parse(got)
	assert.Equal(t, "3", u.Query().Get("serverity"))
	assert.Equal(t, "3", u.Query().Get("serverityIdStr"))
	assert.Equal(t, "5", u.Query().Get("categoryId"))
	assert.Equal(t, "1", u.Query().Get("referenceScope"))
}

func TestItoaOrDefault(t *testing.T) {
	assert.Equal(t, "10", itoaOrDefault(0, 10))
	assert.Equal(t, "10", itoaOrDefault(-1, 10))
	assert.Equal(t, "20", itoaOrDefault(20, 10))
}
