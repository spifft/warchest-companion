package winui

import (
	_ "embed"
	"strconv"
	"strings"
)

//go:embed icon.ico
var IconICO []byte

// newer reports whether release tag "v1.2.3" is newer than this build's version "1.2.0" (dev builds never update).
func newer(tag, have string) bool {
	if have == "" || have == "dev" {
		return false
	}
	a, b := parts(tag), parts(have)
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func parts(v string) [3]int {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	for i, p := range strings.SplitN(v, ".", 3) {
		out[i], _ = strconv.Atoi(p)
	}
	return out
}
