package codecs

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"net/http"
	"testing"
)

type CodecTestSuite struct {
	suite.Suite
}

func RunCodecTestSuite(t *testing.T) {
	suite.Run(t, new(CodecTestSuite))
}

func (suite *CodecTestSuite) TestMarshalStructure() {
	value := "test"
	code := http.StatusOK
	input_params := make(map[string]interface{})
	notifications := make(map[string]interface{})
	name := "testName"
	options := map[string]interface{}{
		"status":        code,
		"input_params":  input_params,
		"notifications": notifications,
		"name":          name,
		"matched_type":  BasicMimeType,
	}
	expectedStructure := map[string]interface{}{
		"meta": map[string]interface{}{
			"input_params": input_params,
			"code":         code,
		},
		"notifications": notifications,
		"response": map[string]interface{}{
			name: value,
		},
	}

	codec := new(RadioboxApiCodec)
	response, err := codec.Marshal(value, options)
	structure := make(map[string]interface{})
	err = json.Unmarshal(response, structure)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), structure, expectedStructure)
}
