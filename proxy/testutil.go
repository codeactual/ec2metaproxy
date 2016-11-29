package proxy

import "testing"

// StringsEqual asserts all string pairs are equal.
func stringsEqual(t *testing.T, pairs [][2]string) {
	for _, v := range pairs {
		if v[0] != v[1] {
			t.Fatalf("expected %#v to equal %#v", v[0], v[1])
		}
	}
}

func fatalOnErr(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}
}
