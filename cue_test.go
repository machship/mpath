package mpath

import (
	"encoding/json"
	"testing"

	"github.com/atotto/clipboard"
)

func Test_CueFromString(t *testing.T) {
	cueString1 := `
	"0f2fbb3e-c95a-4760-8bad-ae5610c28454": {
		_dependencies: ["87e54fe3-6e64-454e-acf7-1a36801f1b87"]
		...
	}
	"4ebec022-4119-4cc8-bb7f-7a713790305c": {
		result:     string
		"_errored": bool
		_dependencies: ["f17e90d1-d3f7-4f55-a50f-4323206c7fd1"]
		"_error"?: {
			message: string
		}
	}
	"87e54fe3-6e64-454e-acf7-1a36801f1b87": {
		result:     _
		"_errored": bool
		_dependencies: ["4ebec022-4119-4cc8-bb7f-7a713790305c"]
		"_error"?: {
			message: string
		}
	}
	"97abc773-2f79-4d77-9859-2e78f38161d7": {
		_dependencies: ["0f2fbb3e-c95a-4760-8bad-ae5610c28454"]
		...
	}
	"f17e90d1-d3f7-4f55-a50f-4323206c7fd1": {
		result:     float
		"_errored": bool
		_dependencies: []
		"_error"?: {
			message: string
		}
	}
	input: {
		_dependencies: []
		...
	}
	variables: {
		test: string
		_dependencies: []
	}
	`

	var err error

	// bigQuery := `{OR, $.a.Equal(12), $.a.Equal(16),{OR, $.a.Equal(12), $.a.Equal(16)}}`
	// bigQuery := `$._b.arrayOfInts.Sum(1,$._a.result).Equal(4).NotEqual({OR,$._b.bool})`
	// bigQuery := `$._b.bool.Equal($._b.bool)`
	// bigQuery := `$._b.results[@.bool].First().example`
	bigQuery := `$`

	// bigQuery := `$._b.results[@.bool].Any()`
	// bigQuery := `$._b.results[@.bool].First().Multiply(12).GreaterOrEqual($._input.num)`
	tc, rdm, err := CueValidate(bigQuery, cueString1, "87e54fe3-6e64-454e-acf7-1a36801f1b87")

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
