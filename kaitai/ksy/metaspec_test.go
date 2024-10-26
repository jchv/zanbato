package ksy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMetaSpec(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected MetaSpec
	}{
		{
			"ks-version 0.9",
			`ks-version: 0.9`,
			MetaSpec{KSVersion: "0.9"},
		},
		{
			"ks-version 1",
			`ks-version: 1`,
			MetaSpec{KSVersion: "1"},
		},
		{
			"endian simple",
			`endian: le`,
			MetaSpec{Endian: EndianSpec{Value: "le"}},
		},
		{
			"endian switch",
			`
endian:
  switch-on: _parent.indicator
  cases:
    '[0x49, 0x49]': le
    '[0x4d, 0x4d]': be
`,
			MetaSpec{Endian: EndianSpec{SwitchOn: "_parent.indicator", Cases: EndianCaseMapSpec{"[0x49, 0x49]": "le", "[0x4d, 0x4d]": "be"}}},
		},
		{
			"bit endian simple",
			`bit-endian: le`,
			MetaSpec{BitEndian: BitEndianSpec{Value: "le"}},
		},
		{
			"gettext_mo meta",
			`
id: gettext_mo
file-extension: mo
encoding: UTF-8
title: gettext binary database
application:
  - GNU gettext
  - libintl
license: BSD-2-Clause
`,
			MetaSpec{
				ID:            "gettext_mo",
				Title:         "gettext binary database",
				Application:   MultiString{"GNU gettext", "libintl"},
				FileExtension: MultiString{"mo"},
				License:       "BSD-2-Clause",
				Encoding:      "UTF-8",
			},
		},
		{
			"tsm meta",
			`
id: tsm
title: InfluxDB TSM file
application: InfluxDB
license: MIT
file-extension: tsm
endian: be
`,
			MetaSpec{
				ID:            "tsm",
				Title:         "InfluxDB TSM file",
				Application:   MultiString{"InfluxDB"},
				License:       "MIT",
				FileExtension: MultiString{"tsm"},
				Endian:        EndianSpec{Value: "be"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := MetaSpec{}
			require.Nil(t, yaml.Unmarshal([]byte(test.input), &actual))
			assert.Equal(t, test.expected, actual)
		})
	}
}
