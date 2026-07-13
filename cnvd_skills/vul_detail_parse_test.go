package cnvd_skills

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCnvdSkills_ParseVulDetail_Offline(t *testing.T) {
	body, err := os.ReadFile("testdata/vul_detail_cnvd_2021_67823.html")
	assert.Nil(t, err)

	detail, err := NewCnvdSkills().ParseVulDetail(string(body))
	assert.Nil(t, err)
	assert.NotNil(t, detail)

	assert.Equal(t, "CNVD-2021-67823", detail.CNVD)
	assert.Equal(t, "CVE-2021-39148", detail.CVE)
	assert.Equal(t, "2021-09-03", detail.PublishTimeStr)

	assert.NotNil(t, detail.HazardLevel)
	assert.Equal(t, "中", detail.HazardLevel.Level)
	assert.Equal(t, "AV:N/AC:M/Au:S/C:P/I:P/A:P", detail.HazardLevel.CVSS2)

	assert.Contains(t, detail.Product, "<=1.4.17")

	assert.NotNil(t, detail.VendorPatch)
	assert.Equal(t, "/patchInfo/show/289241", detail.VendorPatch.Href)
	assert.Contains(t, detail.VendorPatch.Title, "XStream任意代码执行漏洞")

	assert.Equal(t, "(无附件)", detail.AttachFile)
}

func TestCnvdSkills_ParseVulDetail_EmptyBody(t *testing.T) {
	detail, err := NewCnvdSkills().ParseVulDetail("")
	assert.Nil(t, err)
	assert.NotNil(t, detail)
	assert.Empty(t, detail.CNVD)
}

func TestCnvdSkills_ParseVulDetail_MalformedHTML(t *testing.T) {
	detail, err := NewCnvdSkills().ParseVulDetail("<html><body><tr><td>CNVD-ID</td><td>CNVD-2021-67823")
	assert.Nil(t, err)
	assert.NotNil(t, detail)
}

func TestCnvdSkills_decodeHTMLEntities(t *testing.T) {
	assert.Equal(t, "a<=b", decodeHTMLEntities("a&lt;=b"))
	assert.Equal(t, "x & y", decodeHTMLEntities("x &amp; y"))
	assert.NotPanics(t, func() { decodeHTMLEntities("<<<") })
}
