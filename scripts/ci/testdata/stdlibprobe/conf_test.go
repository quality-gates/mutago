package conf

import "testing"

func TestParse(t *testing.T) {
	c, err := Parse([]byte(`{"name":"svc","port":8080}`))
	if err != nil || c.Name != "svc" || c.Port != 8080 {
		t.Fatalf("valid: %+v %v", c, err)
	}
	for _, bad := range []string{`{`, `{"name":" ","port":80}`, `{"name":"a","port":0}`, `{"name":"a","port":65536}`} {
		if c, err := Parse([]byte(bad)); err == nil || c != (Config{}) {
			t.Fatalf("%s: %+v %v", bad, c, err)
		}
	}
	if _, err := Parse([]byte(`{"name":"a","port":1}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse([]byte(`{"name":"a","port":65535}`)); err != nil {
		t.Fatal(err)
	}
}
