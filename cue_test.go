package mpath

import (
	"encoding/json"
	"testing"

	"github.com/atotto/clipboard"
)

func Test_CueFromString(t *testing.T) {
	cueString1 := `
_a: {
	result:     float  
	"_errored": bool 
	_dependencies: []
	"_error"?: {
		message: string
	}
}
_b: {
	number: int
	results:     [{ 
		"example": string
	}]
	_dependencies: ["_a"]
}
_c: {
	iNeedAString: string
	_dependencies: ["_b"]
}
_input: {
	_dependencies: []
	consignmentID: number
}
_variables: {
	varname?: {
		name: string
	}
	_dependencies: []
}
	`

	var err error

	err = CueFromString(cueString1)
	if err != nil {
		t.Error(err)
	}

	fmJson, _ := json.Marshal(funcMap)
	clipboard.WriteAll(string(fmJson))
}