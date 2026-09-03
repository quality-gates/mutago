package numbers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAndFormatIntLiteral(t *testing.T) {
	tests := []struct {
		input       string
		decExpected string
		incExpected string
	}{
		{"10", "9", "11"},
		{"0", "(-1)", "1"},
		{"1_000", "999", "1001"},
		{"0x10", "0xf", "0x11"},
		{"0X10", "0Xf", "0X11"},
		{"0x0", "(-0x1)", "0x1"},
		{"0b10", "0b1", "0b11"},
		{"0B10", "0B1", "0B11"},
		{"0b0", "(-0b1)", "0b1"},
		{"0o10", "0o7", "0o11"},
		{"0O10", "0O7", "0O11"},
		{"0o0", "(-0o1)", "0o1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			info, ok := parseIntLiteral(tt.input)
			assert.True(t, ok)
			assert.Equal(t, tt.decExpected, formatIntLiteral(info.val-1, info))
			assert.Equal(t, tt.incExpected, formatIntLiteral(info.val+1, info))
		})
	}

	_, ok := parseIntLiteral("invalid")
	assert.False(t, ok)
}
