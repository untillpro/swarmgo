/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

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
