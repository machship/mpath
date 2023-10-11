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

	tc, rdm, err := CueValidate(`$._b.results[{AND,{OR,@.example.Equal("or op")},@.example.Equal("something")}].example.AnyOf("bob","jones")`, cueString1, "_c")
	// tc, rdm, err := CueValidate(`$._b.results[@.example.Equal("something")].example`, cueString1, "_c")
	// tc, rdm, err := CueValidate(`$._b.results.First().example.AnyOf("bob","jones")`, cueString1, "_c")
	if err != nil {
		t.Error(err)
	}

	jstr, _ := json.MarshalIndent(tc, "", "\t")
	clipboard.WriteAll(string(jstr))

	_ = tc
	_ = rdm
	return
}
