package cli

import "testing"

type testpair struct {
	stdout  string
	version string
}

var tests = []testpair{
	{"", ""},
	{"Docker version 19.03.2, build 6a30dfc", "19.03.2"},
	{"Docker version 19.03, build 6a30dfc", "19.03"},
	{"Docker version, build 6a30dfc", ""},
}

func TestParseDockerVersion(t *testing.T) {
	for _, pair := range tests {
		parsedVer := ParseDockerVersion(pair.stdout)
		if parsedVer != pair.version {
			t.Error(
				"For:", pair.stdout,
				"expected:", pair.version,
				"got:", parsedVer,
			)
		}
	}
}
