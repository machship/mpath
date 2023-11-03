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
	"string": string
	results:     [{ 
		result:     float  
		"example": string
		"bool": bool
		"boolean": bool
		array: [{
			object: {
				nested: {
					boolean: bool
				}
			}
		}]
	}]
	arrayOfInts: [int]
	arrayOfStrings: [string]
	_dependencies: ["_a"]
	"bool": bool
}
_c: {
	iNeedAString: string
	_dependencies: ["_b"]
}
_input: {
	_dependencies: []
	consignmentID: number
	num: int
}
_variables: {
	varname?: {
		name: string
	}
	_dependencies: []
}
	`

	var err error

	// bigQuery := `{OR, $.a.Equal(12), $.a.Equal(16),{OR, $.a.Equal(12), $.a.Equal(16)}}`
	// bigQuery := `$._b.arrayOfInts.Sum(1,$._a.result).Equal(4).NotEqual({OR,$._b.bool})`
	// bigQuery := `$._b.bool.Equal($._b.bool)`
	// bigQuery := `$._b.results[@.bool].First().example`
	bigQuery := `$._b.results[AND,@.example.Greater(12),@.example.Greater(16)].First().boolean`

	// bigQuery := `$._b.results[@.bool].Any()`
	// bigQuery := `$._b.results[@.bool].First().Multiply(12).GreaterOrEqual($._input.num)`
	tc, rdm, err := CueValidate(bigQuery, cueString1, "_c")

	// tc, rdm, err := CueValidate(`{OR,$._b.results.First().bool}`, cueString1, "_c")
	// tc, rdm, err := CueValidate(`$._b.results[{AND,{OR,@.example.Equal("or op")},@.example.Equal("something")}].First().example.AnyOf("bob","jones")`, cueString1, "_c")
	// tc, rdm, err := CueValidate(`$._b.results[@.example.Equal("something")].example`, cueString1, "_c")
	// tc, rdm, err := CueValidate(`$._b.results.First().example.AnyOf("bob","jones")`, cueString1, "_c")
	if err != nil {
		t.Error(err)
	}

	jstr, _ := json.MarshalIndent(tc, "", "\t")
	clipboard.WriteAll(string(jstr))

	// jstr, _ = json.MarshalIndent(rdm, "", "\t")
	// clipboard.WriteAll(string(jstr))

	_ = tc
	_ = rdm

}
