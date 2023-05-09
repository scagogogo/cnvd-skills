package cnvd_crawler

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCnvdCrawler_VulList(t *testing.T) {
	err := NewCnvdCrawler().VulList(PinYiProxyProvider)
	assert.NotNil(t, err)
}
