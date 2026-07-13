package cnvd_skills

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCnvdSkills_ParseVulPatch_Offline(t *testing.T) {
	body, err := os.ReadFile("testdata/vul_patch_289241.html")
	assert.Nil(t, err)

	patch, err := NewCnvdSkills().ParseVulPatch(string(body))
	assert.Nil(t, err)
	assert.NotNil(t, patch)

	assert.Equal(t, "XStream任意代码执行漏洞（CNVD-2021-67823）的补丁", patch.Name)
	assert.Equal(t, "XStream", patch.Vendor)
	assert.Equal(t, "http://x-stream.github.io/changes.html", patch.Link)
	assert.Contains(t, patch.Description, "1.4.18")
	assert.Equal(t, "2021-09-03", patch.PublishTimeStr)
}

func TestCnvdSkills_ParseVulPatch_EmptyBody(t *testing.T) {
	patch, err := NewCnvdSkills().ParseVulPatch("")
	assert.Nil(t, err)
	assert.NotNil(t, patch)
	assert.Empty(t, patch.Name)
}

func TestCnvdSkills_ParseVulPatch_MalformedHTML(t *testing.T) {
	patch, err := NewCnvdSkills().ParseVulPatch("<html><body><tr><td>补丁名称")
	assert.Nil(t, err)
	assert.NotNil(t, patch)
}
