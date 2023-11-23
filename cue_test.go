package mpath

import (
	"encoding/json"
	"testing"

	"github.com/atotto/clipboard"
)

func Test_CueFromString(t *testing.T) {
	cueString := `
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

	mpathQuery := `$.step1.result[@.age.Equal($.step1.num)].First()`
	tc, _, err := CueValidate(mpathQuery, cueString, "step2")
	if err != nil {
		t.Error(err)
	}

	jstr, err := json.MarshalIndent(tc, "", "\t")
	if err != nil {
		t.Error(err)
	}
	clipboard.WriteAll(string(jstr))
}

func Test_CueStringTableTests(t *testing.T) {
	type tableTest struct {
		name         string
		mq           string
		expectErrors bool
		cp           string
	}

	tests := []tableTest{
		{
			name: "check that array without ... can be assessed",
			mq:   `$.step1.result[@.age.Equal($.step1.num)].First()`,
			cp:   "step2",
		},
		{
			name: "check that array with ... can be assessed",
			mq:   `$.step2.result[@.age.Equal($.step2.num)].First()`,
			cp:   "step3",
		},
	}

	for _, test := range tests {
		tc, _, err := CueValidate(test.mq, cueStringForTests, test.cp)
		if err != nil {
			t.Errorf("test '%s'; got unexpected returned error: %v", test.name, err)
		}
		if tc != nil && tc.HasErrors() != test.expectErrors {
			t.Errorf("test '%s'; expected %t got %t for HasErrors(); err was '%v'", test.name, test.expectErrors, tc.HasErrors(), tc.GetErrors())
		}
		tcb, _ := json.MarshalIndent(tc, "", "  ")
		t.Log(string(tcb))
	}
}

const (
	cueStringForTests = `
		step1: {
			num: int
			_dependencies: [] 
			result: [{
				name: string
				age: int
			}]
		}  

		step2: {
			num: int
			_dependencies: ["step1"] 
			result: [...{
				name: string
				age: int
			}]
		}  
		
		step3: {
			_dependencies: ["step2"]
		}
	`
)
