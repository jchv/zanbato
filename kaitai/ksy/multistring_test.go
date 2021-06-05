package ksy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestMultiString(t *testing.T) {
	type TestStruct struct {
		Multi MultiString
	}
	test := TestStruct{}
	assert.Nil(t, yaml.Unmarshal([]byte(`multi: 'scalar value'`), &test))
	assert.Equal(t, MultiString{"scalar value"}, test.Multi)

	assert.Nil(t, yaml.Unmarshal([]byte(`multi: ['array', 'value']`), &test))
	assert.Equal(t, MultiString{"array", "value"}, test.Multi)

	out, err := yaml.Marshal(TestStruct{Multi: MultiString{"scalar value"}})
	assert.Nil(t, err)
	assert.Equal(t, "multi:\n  - scalar value\n", string(out))

	out, err = yaml.Marshal(TestStruct{Multi: MultiString{"array", "value"}})
	assert.Nil(t, err)
	assert.Equal(t, "multi:\n  - array\n  - value\n", string(out))
}
