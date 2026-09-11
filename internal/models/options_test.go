package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOptionsDefaults(t *testing.T) {
	opts := NewOptions()
	assert.NotNil(t, opts)
	assert.True(t, opts.Config.SkipFileWithoutTest, "skip_without_test must default to true")
	assert.True(t, opts.Config.SkipFileWithBuildTag, "skip_with_build_tags must default to true")
}

func TestOptionsApplyConfigDefaults(t *testing.T) {
	opts := &Options{}
	assert.False(t, opts.Config.SkipFileWithoutTest)
	assert.False(t, opts.Config.SkipFileWithBuildTag)

	opts.ApplyConfigDefaults()
	assert.True(t, opts.Config.SkipFileWithoutTest, "skip_without_test must be true after applying defaults")
	assert.True(t, opts.Config.SkipFileWithBuildTag, "skip_with_build_tags must be true after applying defaults")
}
