package parser

import "testing"

func TestVerificationCache(t *testing.T) {
	for seq := 0; seq < 1024; seq++ {
		c := packageCache{packages: map[string]*pkgCacheEntry{}, files: map[string]loadedFile{}}
		entries := []*pkgCacheEntry{{}, {}}
		var expected *pkgCacheEntry
		n := seq
		for i := 0; i < 5; i++ {
			op := n % 4
			n /= 4
			switch op {
			case 0, 1:
				got := c.publish(c.currentVersion(), "dir", entries[op])
				if expected == nil {
					expected = entries[op]
				}
				if got != expected {
					t.Fatalf("first publisher replaced in sequence %d step %d", seq, i)
				}
			case 2:
				c.clear()
				expected = nil
			case 3:
				c.prepare(c.currentVersion(), map[string]loadedFile{"file": {}}, map[string]struct{}{"dir": {}}, entries[1])
				expected = entries[1]
			}
			got, ok, _ := c.packageFor("dir")
			if ok != (expected != nil) || got != expected {
				t.Fatalf("cached value mismatch seq=%d step=%d", seq, i)
			}
		}
	}
}
