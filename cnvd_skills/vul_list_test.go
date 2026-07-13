package cnvd_skills

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCnvdSkills_VulList(t *testing.T) {
	err := NewCnvdSkills().VulList(PinYiProxyProvider)
	assert.NotNil(t, err)
}
