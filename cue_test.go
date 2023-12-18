package mpath

import (
	"encoding/json"
	"testing"

	"github.com/atotto/clipboard"
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
		{
			name: "test logical op at root of query",
			mq:   `{OR,$.step1.num.Greater(0),$.step2.num.Less(0)}`,
			cp:   "step3",
		},
		{
			name: "root only",
			mq:   "$",
			cp:   "87e54fe3-6e64-454e-acf7-1a36801f1b87",
		},
		{
			name: "input only",
			mq:   "$.input.name",
			cp:   "input",
		},
		{
			name: "object after ParseJSON",
			mq:   `$.step4.result.ParseJSON().object`,
			cp:   "step5",
		},
		{
			name:         "empty string",
			mq:           "",
			cp:           "input",
			expectErrors: true,
		},
		{
			name:         "new line",
			mq:           "\n",
			cp:           "input",
			expectErrors: true,
		},
		{
			name:         "incomplete ident",
			mq:           "$.step1.result[@.na]",
			cp:           "step2",
			expectErrors: true,
		},
		{
			name: "complex can be filtered",
			mq:   `{OR,$.step7.results[AND,@.example.AnyOf("Test","Something"),@.example.NotEqual("Another")].First().array[OR,@.object.nested.boolean].First().object.nested.boolean,$.step6.result.Equal($.input.num.Multiply(12))}`,
			cp:   "step8",
		},
	}

	var onlyRunTest string
	var copyAndLog bool

	// onlyRunTest = "complex can be filtered"
	// copyAndLog = true

	for _, test := range tests {
		if onlyRunTest != "" && test.name != onlyRunTest {
			continue
		}

		tc, err := CueValidate(test.mq, cueStringForTests, test.cp)
		if err != nil && !test.expectErrors {
			t.Errorf("test '%s'; got unexpected returned error: %v", test.name, err)
		}
		if tc != nil && tc.HasErrors() != test.expectErrors {
			t.Errorf("test '%s'; expected %t got %t for HasErrors(); err was '%v'", test.name, test.expectErrors, tc.HasErrors(), tc.GetErrors())
		}

		if copyAndLog {
			tcb, _ := json.MarshalIndent(tc, "", "  ")
			clipboard.WriteAll(string(tcb))
			t.Log(string(tcb))
		}
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
			name: string
			num: int
			...
		}

		step4: {
			_dependencies: []
			result: string
		}

		step5: {
			_dependencies: ["step4"]
			result: string
		}

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
		
		"step6": {
			result: int,
			_dependencies: []
		}
		"step7": {
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
			_dependencies: ["step6"]
		}
		"step8": {
			result: string
			_dependencies: ["step7"]
		}		

		variables: {
			test: string
			_dependencies: []
		}
	`
)
