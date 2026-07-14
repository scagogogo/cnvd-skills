package cnvd_skills

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseCnvdDate(t *testing.T) {
	// Happy Path：纯日期
	d := parseCnvdDate("2021-09-03")
	assert.NotNil(t, d)
	assert.Equal(t, 2021, d.Year())
	assert.Equal(t, time.September, d.Month())
	assert.Equal(t, 3, d.Day())

	// Happy Path：日期+时间
	d2 := parseCnvdDate("2021-08-23 12:30:00")
	assert.NotNil(t, d2)
	assert.Equal(t, 12, d2.Hour())
	assert.Equal(t, 30, d2.Minute())

	// Happy Path：斜杠分隔
	d3 := parseCnvdDate("2021/09/03")
	assert.NotNil(t, d3)
	assert.Equal(t, 2021, d3.Year())

	// Edge Case：空串
	assert.Nil(t, parseCnvdDate(""))

	// Edge Case：空白
	assert.Nil(t, parseCnvdDate("   "))

	// Error Path：无法识别的格式
	assert.Nil(t, parseCnvdDate("not-a-date"))
	assert.Nil(t, parseCnvdDate("2021-13-99"))
}

func TestCnvdSkills_ParseVulDetail_TimeFields(t *testing.T) {
	body, err := os.ReadFile("testdata/vul_detail_with_time.html")
	assert.Nil(t, err)

	detail, err := NewCnvdSkills().ParseVulDetail(string(body))
	assert.Nil(t, err)

	// 公开日期：纯日期
	assert.NotNil(t, detail.PublishTime)
	assert.Equal(t, "2021-09-03", detail.PublishTimeStr)
	assert.Equal(t, time.September, detail.PublishTime.Month())

	// 报送时间：日期+时间
	assert.NotNil(t, detail.PostTime)
	assert.Equal(t, 12, detail.PostTime.Hour())

	// 收录时间
	assert.NotNil(t, detail.RecordTime)
}
