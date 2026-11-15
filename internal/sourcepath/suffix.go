package sourcepath

import "strings"

// LongestSuffixKey finds the most specific path suffix in paths. An empty key
// matches only an empty path; found distinguishes that match from a miss.
func LongestSuffixKey[V any](path string, paths map[string]V) (key string, found bool) {
	for candidate := range paths {
		if candidate == path || (candidate != "" && strings.HasSuffix(path, "/"+candidate)) {
			if !found || len(candidate) > len(key) {
				key, found = candidate, true
			}
		}
	}
	return key, found
}
