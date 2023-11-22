package mpath

import (
	"encoding/json"
	"fmt"
	"log"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/atotto/clipboard"
)

func Test_CueFromString(t *testing.T) {
	cueString1 := `
	
	step1: {
		num: int
		_dependencies: [] 
		result: [{
			name: string
			age: int
		}]
	}  
	
	step2: {
		_dependencies: ["step1"]
		result: [...{
			name: string
			age: int
		}]
	}
	`

	var err error

	// bigQuery := `{OR, $.a.Equal(12), $.a.Equal(16),{OR, $.a.Equal(12), $.a.Equal(16)}}`
	// bigQuery := `$._b.arrayOfInts.Sum(1,$._a.result).Equal(4).NotEqual({OR,$._b.bool})`
	// bigQuery := `$._b.bool.Equal($._b.bool)`
	// bigQuery := `$._b.results[@.bool].First().example`

	bigQuery := `$.step1.result[@.age.Equal($.step1.num)].First()`
	// bigQuery := `$.step1.result.First()`

	// bigQuery := `$._b.results[@.bool].Any()`
	// bigQuery := `$._b.results[@.bool].First().Multiply(12).GreaterOrEqual($._input.num)`
	tc, rdm, err := CueValidate(bigQuery, cueString1, "step2")

	// tc, rdm, err := CueValidate(`{OR,$._b.results.First().bool}`, cueString1, "_c")
	// tc, rdm, err := CueValidate(`$._b.results[{AND,{OR,@.example.Equal("or op")},@.example.Equal("something")}].First().example.AnyOf("bob","jones")`, cueString1, "_c")
	// tc, rdm, err := CueValidate(`$._b.results[@.example.Equal("something")].example`, cueString1, "_c")
	// tc, rdm, err := CueValidate(`$._b.results.First().example.AnyOf("bob","jones")`, cueString1, "_c")
	if err != nil {
		t.Error(err)
	}

	jstr, err := json.MarshalIndent(tc, "", "\t")
	if err != nil {
		t.Error(err)
	}
	clipboard.WriteAll(string(jstr))

	// jstr, _ = json.MarshalIndent(rdm, "", "\t")
	// clipboard.WriteAll(string(jstr))

	_ = tc
	_ = rdm

}

func Test_CueHiddenParse(t *testing.T) {
	cueFile := `
	step1: {
		num: int
		_dependencies: [] 
		result: [...{
			name: string
			age: int
		}]
	}  
	
	step2: {
		_dependencies: ["step1"]
		result: [...{
			name: string
			age: int
		}]
	}
	`

	var rootValue cue.Value
	ctx := cuecontext.New()
	rootValue = ctx.CompileString(cueFile)
	if err := rootValue.Err(); err != nil {
		log.Fatal(err)
	}

	v1, err := findValueAtPath(rootValue, CuePath{"step1", "result", "age"})
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%#v\n", v1)
}
