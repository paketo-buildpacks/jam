package internal

import (
	"testing"
)

func TestUnitCompareVersions(t *testing.T) {
	for _, tc := range []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "equal versions", a: "21.0.13", b: "21.0.13", want: 0},
		{name: "equal with v prefix", a: "v21.0.13", b: "21.0.13", want: 0},
		{name: "plain is older than metadata", a: "21.0.13", b: "21.0.13+0.11", want: -1},
		{name: "metadata is newer than plain", a: "21.0.13+0.11", b: "21.0.13", want: 1},
		{name: "lower build", a: "21.0.13+0.10", b: "21.0.13+0.11", want: -1},
		{name: "numeric build segments", a: "21.0.13+0.9", b: "21.0.13+0.10", want: -1},
		{name: "lts metadata", a: "21.0.13+9.0.LTS", b: "21.0.13+10.0.LTS", want: -1},
		{name: "longer metadata", a: "21.0.13+0.11", b: "21.0.13+0.11.1", want: -1},
		{name: "lexical metadata", a: "21.0.13+a", b: "21.0.13+b", want: -1},
		{name: "patch build metadata", a: "21.0.13+0.3.5", b: "21.0.13+0.3.7", want: -1},
		{name: "precedence wins over metadata", a: "21.0.13", b: "21.0.14", want: -1},
		{name: "large metadata does not beat precedence", a: "21.0.13+99", b: "21.0.14", want: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := compareVersions(tc.a, tc.b); sign(got) != tc.want {
				t.Errorf("compareVersions(%q, %q) = %d, want sign %d", tc.a, tc.b, got, tc.want)
			}
			if got := compareVersions(tc.b, tc.a); sign(got) != -tc.want {
				t.Errorf("compareVersions(%q, %q) = %d, want sign %d", tc.b, tc.a, got, -tc.want)
			}
		})
	}
}

func sign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}
