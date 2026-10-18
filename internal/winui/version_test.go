package winui

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		tag, have string
		want      bool
	}{
		{"v0.2.0", "0.1.0", true}, {"v0.1.1", "0.1.0", true}, {"v1.0.0", "0.9.9", true},
		{"v0.1.0", "0.1.0", false}, {"v0.1.0", "0.2.0", false}, {"v0.2.0", "dev", false}, {"v0.10.0", "0.9.0", true},
	}
	for _, c := range cases {
		if got := newer(c.tag, c.have); got != c.want {
			t.Errorf("newer(%q, %q) = %v, want %v", c.tag, c.have, got, c.want)
		}
	}
}
