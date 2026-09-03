package numbers

import (
	"strconv"
	"strings"
)

type intLiteralInfo struct {
	val    int64
	base   int
	prefix string
}

func parseIntLiteral(s string) (intLiteralInfo, bool) {
	cleaned := strings.ReplaceAll(s, "_", "")
	base := 10
	prefix := ""

	lower := strings.ToLower(cleaned)
	if strings.HasPrefix(lower, "0x") {
		base = 16
		prefix = s[:2]
	} else if strings.HasPrefix(lower, "0b") {
		base = 2
		prefix = s[:2]
	} else if strings.HasPrefix(lower, "0o") {
		base = 8
		prefix = s[:2]
	} else if len(cleaned) > 1 && cleaned[0] == '0' && isOctalDigits(cleaned[1:]) {
		base = 8
		prefix = "0"
	}

	val, err := strconv.ParseInt(cleaned, 0, 64)
	if err != nil {
		return intLiteralInfo{}, false
	}

	return intLiteralInfo{val: val, base: base, prefix: prefix}, true
}

func isOctalDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '7' {
			return false
		}
	}
	return true
}

func formatIntLiteral(val int64, info intLiteralInfo) string {
	if info.base == 10 {
		return strconv.FormatInt(val, 10)
	}

	if val < 0 {
		absVal := -val
		formatted := strconv.FormatInt(absVal, info.base)
		return "(-" + info.prefix + formatted + ")"
	}

	return info.prefix + strconv.FormatInt(val, info.base)
}
