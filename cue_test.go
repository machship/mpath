package mpath

import (
	"encoding/json"
	"testing"

	"github.com/atotto/clipboard"
)

func Test_CueFromString(t *testing.T) {
	cueString1 := `
"_input": {
    num: int,
    _dependencies: []
}
"_a": {
    result: int,
    _dependencies: []
}
"_b": {
    result: int,
    _dependencies: ["_a"]
}
	`

	var err error

	// bigQuery := `{OR, $.a.Equal(12), $.a.Equal(16),{OR, $.a.Equal(12), $.a.Equal(16)}}`
	// bigQuery := `$._b.arrayOfInts.Sum(1,$._a.result).Equal(4).NotEqual({OR,$._b.bool})`
	// bigQuery := `$._b.bool.Equal($._b.bool)`
	// bigQuery := `$._b.results[@.bool].First().example`
	bigQuery := `$._input.num.Equal(12)`

	// bigQuery := `$._b.results[@.bool].Any()`
	// bigQuery := `$._b.results[@.bool].First().Multiply(12).GreaterOrEqual($._input.num)`
	tc, rdm, err := CueValidate(bigQuery, cueString1, "_b")

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
