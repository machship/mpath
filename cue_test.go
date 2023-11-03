package mpath

import (
	"encoding/json"
	"testing"

	"github.com/atotto/clipboard"
)

func Test_CueFromString(t *testing.T) {
	cueString1 := `
	"input": { 
		num: int,
		_dependencies: []
	}
	"_3dedbf75-1c91-4ec5-8018-99b1efe47462": {
		result: int,
		_dependencies: []
	}
	"_52a015ef-6e51-407d-82e2-72fb218ae65b": {
		results: [{
			example: string
			array: [{
				object: {
					nested: {
						boolean: bool
					}
				}
			}]
		}],
		_dependencies: ["_3dedbf75-1c91-4ec5-8018-99b1efe47462"]
	}
	"_bd33058f-d866-4800-aa97-098c0137e8c0": {
		result: string
		_dependencies: ["_52a015ef-6e51-407d-82e2-72fb218ae65b"]
	}
	`

	var err error

	// bigQuery := `{OR, $.a.Equal(12), $.a.Equal(16),{OR, $.a.Equal(12), $.a.Equal(16)}}`
	// bigQuery := `$._b.arrayOfInts.Sum(1,$._a.result).Equal(4).NotEqual({OR,$._b.bool})`
	// bigQuery := `$._b.bool.Equal($._b.bool)`
	// bigQuery := `$._b.results[@.bool].First().example`
	bigQuery := `$._3dedbf75-1c91-4ec5-8018-99b1efe47462.result.Equal(12)`

	// bigQuery := `$._b.results[@.bool].Any()`
	// bigQuery := `$._b.results[@.bool].First().Multiply(12).GreaterOrEqual($._input.num)`
	tc, rdm, err := CueValidate(bigQuery, cueString1, "_bd33058f-d866-4800-aa97-098c0137e8c0")

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
