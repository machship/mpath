package mpath

import (
	"encoding/json"
	"testing"
)

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
		{
			name:         "try to access step that is not available",
			mq:           `$.step2.result[@.age.Equal($.step2.num)].First()`,
			cp:           "step2",
			expectErrors: true,
		},
		{
			name: "test that optional value in optional step can be referenced",
			mq:   `$.stepOptional.result.name`,
			cp:   "step3",
		},
	}

	for _, test := range tests {
		tc, err := CueValidate(test.mq, cueStringForTests, test.cp)
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

		stepOptional?: {
			result?: {
				name: int
			}
			_dependencies: ["step2"]
		}
		
		step3 : {
			_dependencies: ["stepOptional","step2"]
		}

		def: {
			result: float
			_dependencies: ["abc"]
		}

		input: {
			_dependencies: []
			...
		}
	`
)
