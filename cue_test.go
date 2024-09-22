package mpath

import (
	"encoding/json"
	"sort"
	"testing"
)

func Benchmark_CueStringTableTests(b *testing.B) {
	sort.Slice(cueTableTests, func(i, j int) bool {
		return len(cueTableTests[i].name) > len(cueTableTests[j].name)
	})

	for _, test := range cueTableTests {
		b.Run(test.name, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				b.ReportAllocs()
				tc, err := CueValidate(test.mq, cueStringForTests, test.cp)
				if err != nil && !test.expectErrors {
					b.Errorf("test '%s'; got unexpected returned error: %v", test.name, err)
				}
				if tc != nil && tc.HasErrors() != test.expectErrors {
					b.Errorf("test '%s'; expected %t got %t for HasErrors(); err was '%v'", test.name, test.expectErrors, tc.HasErrors(), tc.GetErrors())
				}
				b.SetBytes(int64(len(test.mq + cueStringForTests + test.cp)))
			}
		})
	}
}

func Benchmark_BigCueFile(b *testing.B) {

	cueFile := `"input": {
    num: int,
    _dependencies: []
}
"3dedbf75-1c91-4ec5-8018-99b1efe47462": {
    result: int,
    _dependencies: []
}
"52a015ef-6e51-407d-82e2-72fb218ae65b": {
    result: {
        #company: {
            id: number
            address?: {
                name?: string
                contact?: string
                phone?: string
                email?: string
                addressLine1?: string
                addressLine2?: string
                addressLinesFriendly?: string
                hasAddressLine2: bool
            }
            name: string
            accountCode: string
            isEnterprise: bool
        }
        
        consignments: [...{
                id: number
                consignmentNumber: string
                carrierConsignmentId: string
                dateCreated: string
                insertedBy: number
                service: {
                    name: string
                    abbreviation: string
                    serialisedSettings?: {...}
                }
                despatchDateTime: string
                fromAddress: {
                    location: {
                        id: number
                        postcode: string
                        paddedPostcode: string
                        state: {
                            id: number
                            name: string
                            code: string
                        }
                        subLocality?: string
                        suburb: string
                        description: string
                        timeZone: string
                        country: {
                            id: number
                            name: string
                            code2: string
                            code3: string
                            numeric: string
                            isInternational: bool
                        }
                    }
                    name: string
                    contact?: string
                    phone?: string
                    email?: string
                    addressLine1: string
                    addressLine2?: string
                    carrierZone?: {
                        id: number
                        name: string
                        abbreviation: string
                    }
                }
                fromLocationCarrierZoneSettings?: {...}
                toAddress: {
                    location: {
                        id: number
                        postcode: string
                        paddedPostcode: string
                        state: {
                            id: number
                            name: string
                            code: string
                        }
                        subLocality?: string
                        suburb: string
                        description: string
                        timeZone: string
                        country: {
                            id: number
                            name: string
                            code2: string
                            code3: string
                            numeric: string
                            isInternational: bool
                        }
                    }
                    name: string
                    contact?: string
                    phone?: string
                    email?: string
                    addressLine1: string
                    addressLine2?: string
                    carrierZone?: {
                        id: number
                        name: string
                        abbreviation: string
                    }
                }
                toLocationCarrierZoneSettings?: {...}
                specialInstructions?: string
                customerReference?: string
                customerReference2?: string
                totalWeight: number
                totalCubic: number
                totalVolume: number
                validFromCompanyLocation: bool
                validToCompanyLocation: bool
                fromCompanyLocationId?: number
                toCompanyLocationId?: number
                fromCompanyLocationAbbreviation?: string
                toCompanyLocationAbbreviation?: string
                receiverCode?: string
                isReceiverPays: bool
                isDgConsignment: bool
                dgsDeclaration: bool
                dgsDeclarationDateTimeUtc?: string
                dgsDeclarationUserId?: number
                emergencyContactNumber?: string
                surcharges?: [...{
                    name: string      
                    abbreviation: string      
                    quantity: number      
                }]
                etaLocal: string
                primaryReferenceType: number
                primaryReference: string
                carrierReference?: string
                totalItemCount: number
                items: [...{
                        id: number
                        carrierItemType?: string
                        itemType: string
                        name: string
                        sku?: string
                        height: number
                        weight: number
                        length: number
                        width: number
                        quantity: number
                        palletSpaces?: number
                        volume: number
                        isDgItem: bool
                        integrationItemDgItems?: [...{
                                id: number
                                companyDgItemId?: number
                                unNumber: number
                                packingGroup: number
                                packingGroupStringValue: string
                                containerType: number
                                containerTypeStringValue: string
                                aggregateQuantity: number
                                isAggregateQuantityWeight: bool
                                numberOfContainers: number
                                dgClassType: number
                                dgClassTypeStringValue: string
                                dgClassTypeStringDescription: string
                                subDgClassTypes?: [...number]
                                subDgClassTypesSeparator?: string
                                subDgClassTypesStringValue?: string
                                hasSubRisk: bool
                                properShippingNameValue: string
                                properShippingName: string
                                technicalOrChemicalGroupNames?: string
                                hasTechnicalName: bool
                                isMarinePollutant: bool
                                isTemperatureControlled: bool
                                isEmptyDgContainer: bool
                                quantityPerContainer: number
                                totalAggregateQuantity: number
                                totalNumberOfContainers: number
                                dgClassDisplay: string
                            }
                        ]
                        integrationItemReferences?: [...{
                                id: number
                                carrierItemReference: string
                                itemNumber: number
                                carrierReference?: string
                            }
                        ]
                        integrationItemContents?: [...{
                                id: number
                                description: string
                                reference1?: string
                                reference2?: string
                                reference3?: string
                                quantity: number
                                dollarValue?: number
                                ciMarksAndNumbers?: string
                                harmonizedCode?: string
                                partNumber?: string
                                purpose?: string
                                countryOfManufactureId?: string
                                countryOfManufacture?: {
                                    id: number
                                    name: string
                                    code2: string
                                    code3: string
                                    numeric: string
                                }
                            }
                        ]
                    }
                ]
                internationalFromCity?: string
                internationalFromPostcode?: string
                internationalFromProvince?: string
                fromCountry?: string
                internationalToCity?: string
                internationalToPostcode?: string
                internationalToProvince?: string
                toCountry?: string
                isInternational: bool
            }
        ]
        pickup: {
            pickupDateTime: string
            palletSpaces?: number
            pickupBooked: bool
            pickupClosingTime?: string
            pickupSpecialInstructions?: string
            pickupRequired: bool
            permanentPickup: bool
            totalWeightKgs: number
        }
        consigningCompany: #company
        accountHolderCompany: #company
        carrier: {
            id: number
            name: string
            code: string
            serialisedSettings?: {...}
        }
        account: {
            name: string
            accountCode?: string
            abbreviation: string
            serialisedSettings?: {...}
        }
        bookingType?: string
    },
    _dependencies: ["3dedbf75-1c91-4ec5-8018-99b1efe47462"]
}
"bd33058f-d866-4800-aa97-098c0137e8c0": {
    result: string
    _dependencies: ["52a015ef-6e51-407d-82e2-72fb218ae65b"]
}`

	testName := "Big Cue"

	b.Run(testName, func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			b.ReportAllocs()
			tc, err := CueValidate(`$.52a015ef-6e51-407d-82e2-72fb218ae65b.result.consignments`, cueFile, `bd33058f-d866-4800-aa97-098c0137e8c0`)
			if err != nil {
				b.Errorf("test '%s'; got unexpected returned error: %v", testName, err)
			}
			if tc != nil && tc.HasErrors() != false {
				b.Errorf("test '%s'; expected %t got %t for HasErrors(); err was '%v'", testName, false, tc.HasErrors(), tc.GetErrors())
			}
			b.SetBytes(int64(len(`$.52a015ef-6e51-407d-82e2-72fb218ae65b.result.consignments` + cueFile + `bd33058f-d866-4800-aa97-098c0137e8c0`)))
		}
	})
}

func Test_CueStringTableTests(t *testing.T) {
	t.Parallel()

	var onlyRunTest string
	var copyAndLog bool

	// onlyRunTest = "check that optional fields return"
	// copyAndLog = true

	for _, test := range cueTableTests {
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
			// clipboard.WriteAll(string(tcb))
			t.Log(string(tcb))
		}
	}
}

type tableTest struct {
	name         string
	mq           string
	expectErrors bool
	cp           string
}

var (
	cueTableTests = []tableTest{
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
		{
			name:         "test get error for incomplete",
			mq:           `$.52a015ef-6e51-407d-82e2-72fb218ae65b.results[AND,@.ex]`,
			cp:           "bd33058f-d866-4800-aa97-098c0137e8c0",
			expectErrors: true,
		},
		{
			name: "retrieve any value",
			mq:   `$.step9.result`,
			cp:   "step10",
		},
		{
			name: "retrieve key from any value",
			mq:   `$.step9.result.subkey.subkey`,
			cp:   "step10",
		},
		{
			name: "retrieve any value in slice",
			mq:   `$.step10.result.sl.First().value`,
			cp:   "step11",
		},
		{
			name: "retrieve key from any value in slice",
			mq:   `$.step10.result.sl.First().value.subkey.subkey`,
			cp:   "step11",
		},
		{
			name: "retrieve bytes from result",
			mq:   `$.step11.result`,
			cp:   "step12",
		},
		{
			name: "check that optional fields return",
			mq:   `$.step2.result.First()`,
			cp:   "stepOptional",
		},
		{
			name: "can get into field afer 'First()' function call",
			mq:   `$.step2.result.First().age`,
			cp:   "stepOptional",
		},
	}
)

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
				name?: string
				age!: int
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

		"3dedbf75-1c91-4ec5-8018-99b1efe47462": {
			result: int,
			_dependencies: []
		}
		"52a015ef-6e51-407d-82e2-72fb218ae65b": {
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
			_dependencies: ["3dedbf75-1c91-4ec5-8018-99b1efe47462"]
		}
		"bd33058f-d866-4800-aa97-098c0137e8c0": {
			result: string
			_dependencies: ["52a015ef-6e51-407d-82e2-72fb218ae65b"]
		}

		"step9": {
			result: _
			_dependencies: ["step7"]
		}
		
		"step10": {
			_dependencies: ["step9"]
			result: {
				sl: [{
					value: _
				}]
			}
		}

		"step11": {
			_dependencies: ["step10"]
			result: bytes
		}

		"step12": {
			_dependencies: ["step11"]
			result: _
		}

		variables: {
			test: string
			_dependencies: []
		}
	`
)

func Test_CueStringNoCurrentPath(t *testing.T) {
	t.Parallel()

	type tableTest struct {
		name         string
		mq           string
		expectErrors bool
		cp           string
	}

	test := tableTest{
		name:         "get root objects",
		mq:           `$.c`,
		cp:           ``,
		expectErrors: false,
	}

	cueString := `
	"a": string
	"b": int
	"c": {
		"x": string
	}
	`

	tc, err := CueValidate(test.mq, cueString, test.cp)
	if err != nil && !test.expectErrors {
		t.Errorf("test '%s'; got unexpected returned error: %v", test.name, err)
	}
	if tc != nil && tc.HasErrors() != test.expectErrors {
		t.Errorf("test '%s'; expected %t got %t for HasErrors(); err was '%v'", test.name, test.expectErrors, tc.HasErrors(), tc.GetErrors())
	}

	tcb, _ := json.MarshalIndent(tc, "", "  ")
	// clipboard.WriteAll(string(tcb))
	t.Log(string(tcb))
}

func Test_CueStringManual(t *testing.T) {
	t.Parallel()

	type tableTest struct {
		name         string
		mq           string
		expectErrors bool
		cp           string
	}

	test := tableTest{
		name:         "manual test",
		mq:           `$.52a015ef-6e51-407d-82e2-72fb218ae65b.results[AND,@.ex]`,
		cp:           `bd33058f-d866-4800-aa97-098c0137e8c0`,
		expectErrors: true,
	}

	cueString := `
	"input": {
		num: int,
		_dependencies: []
	}
	"3dedbf75-1c91-4ec5-8018-99b1efe47462": {
		result: int,
		_dependencies: []
	}
	"52a015ef-6e51-407d-82e2-72fb218ae65b": {
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
		_dependencies: ["3dedbf75-1c91-4ec5-8018-99b1efe47462"]
	}
	"bd33058f-d866-4800-aa97-098c0137e8c0": {
		result: string
		_dependencies: ["52a015ef-6e51-407d-82e2-72fb218ae65b"]
	}
	`

	tc, err := CueValidate(test.mq, cueString, test.cp)
	if err != nil && !test.expectErrors {
		t.Errorf("test '%s'; got unexpected returned error: %v", test.name, err)
	}
	if tc != nil && tc.HasErrors() != test.expectErrors {
		t.Errorf("test '%s'; expected %t got %t for HasErrors(); err was '%v'", test.name, test.expectErrors, tc.HasErrors(), tc.GetErrors())
	}

	tcb, _ := json.MarshalIndent(tc, "", "  ")
	// clipboard.WriteAll(string(tcb))
	t.Log(string(tcb))
}
