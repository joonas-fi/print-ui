package xsfi

import (
	"testing"

	"github.com/function61/gokit/testing/assert"
)

func TestMain(t *testing.T) {
	var joonasAssets = Root.Sub("joonas-assets")

	assert.Equal(t, joonasAssets.ID("milk").URL(), "https://xs.fi/joonas-assets/milk")
}
