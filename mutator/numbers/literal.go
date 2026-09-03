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

	if len(cleaned) >= 2 && cleaned[0] == '0' {
		switch cleaned[1] {
		case 'x', 'X':
			base = 16
			prefix = s[:2]
		case 'b', 'B':
			base = 2
			prefix = s[:2]
		case 'o', 'O':
			base = 8
			prefix = s[:2]
		}
	}

	val, err := strconv.ParseInt(cleaned, 0, 64)
	if err != nil {
		return intLiteralInfo{}, false
	}

	return intLiteralInfo{val: val, base: base, prefix: prefix}, true
}

func formatIntLiteral(val int64, info intLiteralInfo) string {
	if val < 0 {
		if info.base == 10 {
			return "(" + strconv.FormatInt(val, 10) + ")"
		}
		return "(-" + info.prefix + strconv.FormatInt(-val, info.base) + ")"
	}

	return info.prefix + strconv.FormatInt(val, info.base)
}
