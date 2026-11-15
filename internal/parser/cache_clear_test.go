package parser

import "testing"

func TestClearPackageCacheDoesNotRepopulateFromAnEarlierLoad(t *testing.T) {
	c := packageCache{packages: map[string]*pkgCacheEntry{}, files: map[string]loadedFile{}}
	started := make(chan struct{})
	finish := make(chan struct{})
	done := make(chan struct{})
	go func() {
		_, _, version := c.packageFor("dir")
		close(started)
		<-finish
		c.publish(version, "dir", &pkgCacheEntry{})
		close(done)
	}()
	<-started
	c.clear()
	close(finish)
	<-done
	if _, ok, _ := c.packageFor("dir"); ok {
		t.Fatal("load started before clear repopulated the cleared cache")
	}
}
