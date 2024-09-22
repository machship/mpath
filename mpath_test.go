package mpath

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/shopspring/decimal"
)

func Benchmark_ParseAndDo(b *testing.B) {
	var data map[string]any

	err := json.Unmarshal([]byte(jsn), &data)
	if err != nil {
		b.Error("got unexpected json marshal error: %w", err)
	}

	var op Operation

	sort.Slice(testQueries, func(i, j int) bool {
		return len(testQueries[i].Name) > len(testQueries[j].Name)
	})

	b.Log("Parse:")

	for _, test := range testQueries {
		b.Run("Parse "+test.Name, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				b.ReportAllocs()
				op, err = ParseString(test.Query)

				if err != nil { // This is to avoid nil dereference errors
					b.Errorf("'%s' has error: %v,", test.Name, err)
					continue
				}

				if op == nil {
					b.Errorf("'%s' failed to return an operation: %s", test.Name, test.Query)
				}
				b.SetBytes(int64(len(test.Query)))
			}
		})
	}

	b.Log("Do:")

	for _, test := range testQueries {
		b.Run("Do "+test.Name, func(b *testing.B) {
			op, err = ParseString(test.Query)

			for n := 0; n < b.N; n++ {
				b.ReportAllocs()
				_, err = op.Do(data, data)
				if err != nil {
					b.Errorf("'%s' got error from Do(): %v", test.Name, err)
				}
				b.SetBytes(int64(len(test.Query)))
			}
		})
	}

	b.Log("Parse and Do:")

	for _, test := range testQueries {
		b.Run("Parse and Do "+test.Name, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				op, err = ParseString(test.Query)

				b.ReportAllocs()
				_, err = op.Do(data, data)
				if err != nil {
					b.Errorf("'%s' got error from Do(): %v", test.Name, err)
				}
				b.SetBytes(int64(len(test.Query)))
			}
		})
	}
}

func Test_GetRootFieldsAccessed(t *testing.T) {
	t.Parallel()

	for _, test := range testQueries {
		op, err := ParseString(test.Query)
		if err != nil {
			t.Errorf("got error for Test_GetRootFieldsAccessed '%s': %v", test.Name, err)
			continue
		}

		erf := test.ExpectedRootFields
		rfa := GetRootFieldsAccessed(op)
		if len(erf) != len(rfa) {
			t.Errorf("got different lists for Test_GetRootFieldsAccessed '%s'; was %v, expected %v", test.Name, rfa, erf)
			continue
		}

		sort.Strings(test.ExpectedRootFields)

		for i, rf := range erf {
			if rf != rfa[i] {
				t.Errorf("got different value in list for Test_GetRootFieldsAccessed '%s'; index %d was %s, expected %s", test.Name, i, rfa[i], rf)
				goto next
			}
		}
	next:
		continue
	}
}

func Test_Sprint(t *testing.T) {
	t.Parallel()

	// We need only test that the Sprint doesn't throw an error
	for _, test := range testQueries {
		op, err := ParseString(test.Query)
		if err != nil {
			t.Errorf("got error for '%s': %v", test.Name, err)
			t.FailNow()
		}
		if len(op.Sprint(0)) == 0 {
			t.Errorf("Got 0 length Sprint string")
		}
	}
}

func Test_AddressedPaths(t *testing.T) {
	t.Parallel()

	var onlyRun string

	for _, test := range testQueries {
		if onlyRun != "" && test.Name != onlyRun {
			continue
		}

		op, err := ParseString(test.Query)
		if err != nil {
			t.Error(err)
		}

		addressedPaths := AddressedPaths(op)

		gotError := false

		if lGot, lWanted := len(addressedPaths), len(test.ExpectedAddressedPaths); lGot != lWanted {
			t.Errorf("'%s': got bad addressedPaths length: wanted: %d; got: %d", test.Name, lWanted, lGot)
			gotError = true
			goto Error
		}

		for i, fpWanted := range test.ExpectedAddressedPaths {
			fpGot := addressedPaths[i]
			if len(fpWanted) != len(fpGot) {
				t.Errorf("'%s': got bad addressedPaths length at index %d: wanted: %d; got: %d", test.Name, i, len(fpWanted), len(fpGot))
				gotError = true
				continue
			}

			for ii, fpWI := range fpWanted {
				fpGI := fpGot[ii]

				if fpGI != fpWI {
					gotError = true
					t.Errorf("'%s': wrong addressedPaths value at index %d: wanted: '%s'; got: '%s'", test.Name, ii, fpWI, fpGI)
				}
			}
		}

	Error:
		if gotError {
			for _, w := range test.ExpectedAddressedPaths {
				t.Errorf("'%s': wanted: %v", test.Name, w)
			}
			for _, g := range addressedPaths {
				t.Errorf("'%s': got: %v", test.Name, g)
			}
		}
	}
}

func Test_ParseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name              string
		Query             string
		ExpectErrorString string
	}{
		{
			Name:              "opPath access root in filter",
			Query:             `$.x[$.a.Equal("b")]`,
			ExpectErrorString: "cannot use '$' (root) inside filter: error at line 1 col 6: invalid next character '$': must be '@'",
		},
	}

	// We need only test that the Sprint doesn't throw an error
	for _, test := range tests {
		op, err := ParseString(test.Query)
		if err == nil {
			t.Errorf("%s: expected error, got none", test.Name)
			continue
		}
		if err.Error() != test.ExpectErrorString {
			t.Errorf("%s: got wrong error string; wanted '%s', got: '%s'", test.Name, test.ExpectErrorString, err.Error())
			continue
		}
		if op != nil {
			t.Errorf("%s: expected nil op", test.Name)
		}
	}
}

func Test_ManualMap(t *testing.T) {
	t.Parallel()

	query := "$.x.Multiply(12.146)"

	data := map[string]any{
		"x": 10,
	}

	op, err := ParseString(query)

	if err != nil {
		t.Fatalf("Failed to parse string: %s", err)
	}

	_, err = op.Do(data, data)

	if err != nil {
		t.Fatalf("Failed to do mpath: %s", err)
	}
}

func Test_DecimalAtRoot(t *testing.T) {
	t.Parallel()

	const testName = "Test_DecimalAtRoot"

	data := float64(40)
	op, err := ParseString("$")
	if err != nil {
		t.Errorf("'%s' has error: %v,", testName, err)
		return
	}

	dataToUse, err := op.Do(data, data)
	if err != nil {
		t.Errorf("'%s' got error from Do(): %v", testName, err)
		return
	}

	if d, ok := dataToUse.(decimal.Decimal); ok {
		t.Logf("value was float and was %s", d)
	} else {
		t.Error("wasn't decimal.Decimal")
	}
}

func Test_ParseAndDo(t *testing.T) {
	t.Parallel()

	var err error
	var dataAsMap map[string]any
	var dataAsStruct TestDataStruct

	err = json.Unmarshal([]byte(jsn), &dataAsMap)
	if err != nil {
		t.Error("got unexpected json marshal error: %w", err)
		t.FailNow()
	}

	err = json.Unmarshal([]byte(jsn), &dataAsStruct)
	if err != nil {
		t.Error("got unexpected json marshal error: %w", err)
		t.FailNow()
	}

	datas := []any{dataAsMap, dataAsStruct}
	// datas = []any{dataAsStruct}

	onlyRunName := ""

	//`{$.List.Last().SomeSettings[@.Key.Equal("DEF")].Any().Equal(true)}`
	// onlyRunName = "complex 1"

	// onlyRunName = "test add negative number"

	for dataIteration, data := range datas {
		iterationName := "map"
		if dataIteration == 1 {
			iterationName = "struct"
		}

		for _, test := range testQueries {
			if onlyRunName != "" {
				if test.Name != onlyRunName {
					continue
				}
			}

			t.Run(fmt.Sprintf("%s: %s", test.Name, iterationName), func(t *testing.T) {
				op, err := ParseString(test.Query)
				if err != nil { // This is to avoid nil dereference errors
					t.Errorf("'%s' has error: %v,", test.Name, err)
					return
				}

				if op == nil {
					t.Errorf("'%s' failed to return an operation: %s", test.Name, test.Query)
					return
				}

				dataToUse, err := op.Do(data, data)
				if err != nil && test.ExpectedResultType != RT_error {
					t.Errorf("'%s' got error from Do(): %v", test.Name, err)
					return
				}

				switch test.ExpectedResultType {
				case RT_string:
					if d, ok := dataToUse.(string); !ok {
						t.Errorf("'%s' (%s) data was not of expected type '%s'; was %T", test.Name, iterationName, test.ExpectedResultType, dataToUse)
					} else {
						if d != test.Expect_string {
							t.Errorf("'%s' (%s) data did not match expected value '%s': got %s", test.Name, iterationName, test.Expect_string, d)
						}
					}
				case RT_decimal:
					if d, ok := dataToUse.(decimal.Decimal); !ok {
						t.Errorf("'%s' data was not of expected type '%s'; was %T", test.Name, test.ExpectedResultType, dataToUse)
					} else {
						if !d.Equal(test.Expect_decimal) {
							t.Errorf("'%s' (%s) data did not match expected value '%s': got %s", test.Name, iterationName, test.Expect_decimal, d)
						}
					}
				case RT_bool:
					if d, ok := dataToUse.(bool); !ok {
						t.Errorf("'%s' (%s) data was not of expected type '%s'; was %T", test.Name, iterationName, test.ExpectedResultType, dataToUse)
					} else {
						if d != test.Expect_bool {
							t.Errorf("'%s' (%s) data did not match expected value '%t': got %t", test.Name, iterationName, test.Expect_bool, d)
						}
					}
				case RT_array:
					if d, ok := dataToUse.([]any); !ok {
						t.Errorf("'%s' (%s) data was not of expected type '%s'; was %T", test.Name, iterationName, test.ExpectedResultType, dataToUse)
					} else {
						if !reflect.DeepEqual(d, test.Expect_array) {
							t.Errorf("'%s' (%s) data did not match expected value '%t': got %t", test.Name, iterationName, test.Expect_array, d)
						}
					}
				case RT_error:
					if err == nil {
						t.Errorf("'%s' (%s) expected error, got none", test.Name, iterationName)
					} else if err.Error() != test.Expect_error.Error() {
						t.Errorf("'%s' (%s) data did not match expected value '%v': got '%v'", test.Name, iterationName, test.Expect_error, err)
					}
				}
			})
		}
	}
}

func Test_CustomStringTypeMap(t *testing.T) {
	t.Parallel()

	const outStrConst = "hello world"
	data := map[CustomStringTypeForTest]any{"varname": outStrConst}
	queryString := "$.varname"

	op, err := ParseString(queryString)

	if err != nil {
		t.Errorf("failed to parse query: %s", err)
	}

	out, err := op.Do(data, data)

	if err != nil {
		t.Errorf("failed to do mpath: %s", err)
	}

	outStr, ok := out.(string)
	if !ok {
		t.Error("out was not a string")
	}

	if outStr != outStrConst {
		t.Errorf("outStr was wrong; expected: '%s', got '%s'", outStrConst, outStr)
	}
}

var (
	testQueries = []struct {
		Name                   string
		Query                  string
		ExpectedRootFields     []string
		ExpectedAddressedPaths [][]string
		ExpectedResultType     ResultType
		Expect_string          string
		Expect_decimal         decimal.Decimal
		Expect_bool            bool
		Expect_array           []any
		Expect_error           error
	}{
		// { //todo: add expected error states into the tests (this should error)
		// 	Name:               "Test misspelt operation type",
		// 	Query:              `{AN,$.string.Equals("test")}`,
		// 	Expect_bool:        false,
		// 	ExpectedResultType: RT_bool,
		// },
		{
			Name:               "Logical operation as function parameter",
			Query:              `$.bool.NotEqual({OR,$.bool})`,
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"bool"},
			ExpectedAddressedPaths: [][]string{
				[]string{"bool"},
			},
		},
		{
			Name:               "Sum numbers alone",
			Query:              `$.numbers.Sum()`,
			Expect_decimal:     decimal.NewFromFloat(6912),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"numbers"},
			ExpectedAddressedPaths: [][]string{
				[]string{"numbers"},
			},
		},
		{
			Name:               "Sum numbers with string number",
			Query:              `$.numbers.Sum("1000")`,
			Expect_decimal:     decimal.NewFromFloat(7912),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"numbers"},
			ExpectedAddressedPaths: [][]string{
				[]string{"numbers"},
			},
		},
		{
			Name:               "Sum numbers with one value",
			Query:              `$.numbers.Sum(1)`,
			Expect_decimal:     decimal.NewFromFloat(6913),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"numbers"},
			ExpectedAddressedPaths: [][]string{
				[]string{"numbers"},
			},
		},
		{
			Name:               "Sum numbers with multiple",
			Query:              `$.numbers.Sum(1,2.5)`,
			Expect_decimal:     decimal.NewFromFloat(6915.5),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"numbers"},
			ExpectedAddressedPaths: [][]string{
				[]string{"numbers"},
			},
		},
		{
			Name:               "Sum number with value",
			Query:              `$.number.Sum(2.5)`,
			Expect_decimal:     decimal.NewFromFloat(1236.5),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
		{
			Name:               "Sum number with multiple",
			Query:              `$.number.Sum(1,1.5)`,
			Expect_decimal:     decimal.NewFromFloat(1236.5),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
		{
			Name:               "Add number to string number",
			Query:              `$.numberInString.Add(21111.123)`,
			Expect_decimal:     decimal.NewFromFloat(33456.123),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"numberInString"},
			ExpectedAddressedPaths: [][]string{
				[]string{"numberInString"},
			},
		},
		{
			Name:               "Add string number to string number",
			Query:              `$.numberInString.Add("21111.123")`,
			Expect_decimal:     decimal.NewFromFloat(33456.123),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"numberInString"},
			ExpectedAddressedPaths: [][]string{
				[]string{"numberInString"},
			},
		},
		{
			Name:               "trim right of string by n",
			Query:              `$.string.TrimRight(3)`,
			Expect_string:      "abc",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "trim right of string by n > length of string",
			Query:              `$.string.TrimRight(7)`,
			Expect_string:      "",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "trim right of string by n = length of string",
			Query:              `$.string.TrimRight(6)`,
			Expect_string:      "",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "trim right of string by n = 0",
			Query:              `$.string.TrimRight(0)`,
			Expect_string:      "abcDEF",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},

		{
			Name:               "trim left of string by n",
			Query:              `$.string.TrimLeft(3)`,
			Expect_string:      "DEF",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "trim left of string by n > length of string",
			Query:              `$.string.TrimLeft(7)`,
			Expect_string:      "",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "trim left of string by n = length of string",
			Query:              `$.string.TrimLeft(6)`,
			Expect_string:      "",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "trim left of string by n = 0",
			Query:              `$.string.TrimLeft(0)`,
			Expect_string:      "abcDEF",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},

		{
			Name:               "get left n of string",
			Query:              `$.string.Left(2)`,
			Expect_string:      "ab",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "get right n of string",
			Query:              `$.string.Right(2)`,
			Expect_string:      "EF",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},

		{
			Name:               "get left n > len of string",
			Query:              `$.string.Left(7)`,
			Expect_string:      "abcDEF",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "get right n > len of string",
			Query:              `$.string.Right(7)`,
			Expect_string:      "abcDEF",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},

		{
			Name:               "string equal another path string",
			Query:              `$.strings.First().Equal($.string)`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"string", "strings"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
				[]string{"strings"},
			},
		},

		{
			Name:               "regex matches string",
			Query:              `$.string.DoesMatchRegex("a[bc]+[A-Za-z]+")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "regex doesn't match string",
			Query:              `$.string.DoesMatchRegex("z[bc]+[A-Za-z]+")`,
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},

		{
			Name:               "replace regex",
			Query:              `$.string.ReplaceRegex("(a)","z")`,
			Expect_string:      "zbcDEF",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},

		{
			Name:               "replace regex $",
			Query:              `$.regexstring.ReplaceRegex("^(MSRWC)([a-zA-Z0-9]{7})(.*)","$1$2")`,
			Expect_string:      "MSRWC1234567",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"regexstring"},
			ExpectedAddressedPaths: [][]string{
				[]string{"regexstring"},
			},
		},

		{
			Name:               "replace all",
			Query:              `$.string.ReplaceAll("a","z")`,
			Expect_string:      "zbcDEF",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},

		{
			Name:               "get data from string JSON field",
			Query:              `$.result.json.ParseJSON().consignmentID`,
			Expect_decimal:     decimal.NewFromInt(112357),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"result"},
			ExpectedAddressedPaths: [][]string{
				[]string{"result", "json", "consignmentID"},
			},
		},
		{
			Name:               "get data from string JSON field, then put it back to JSON",
			Query:              `$.result.json.ParseJSON().AsJSON()`,
			Expect_string:      "{\"consignmentID\":112357,\"consignmentName\":\"Test consignment\"}",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"result"},
			ExpectedAddressedPaths: [][]string{
				[]string{"result", "json"},
			},
		},
		{
			Name:               "get data from string JSON field, select a field, then put it back to JSON",
			Query:              `$.result.json.ParseJSON().consignmentID.AsJSON()`,
			Expect_string:      `"112357"`,
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"result"},
			ExpectedAddressedPaths: [][]string{
				[]string{"result", "json", "consignmentID"},
			},
		},
		{
			Name:               "get data from string XML field",
			Query:              `$.result.xml.ParseXML().root.consignmentID`,
			Expect_string:      "112358",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"result"},
			ExpectedAddressedPaths: [][]string{
				[]string{"result", "xml", "root", "consignmentID"},
			},
		},
		{
			Name:               "get data from string YAML field",
			Query:              `$.result.yaml.ParseYAML().consignmentID`,
			Expect_decimal:     decimal.NewFromInt(112359),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"result"},
			ExpectedAddressedPaths: [][]string{
				[]string{"result", "yaml", "consignmentID"},
			},
		},
		{
			Name:               "get data from string TOML field",
			Query:              `$.result.toml.ParseTOML().consignmentID`,
			Expect_decimal:     decimal.NewFromInt(112360),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"result"},
			ExpectedAddressedPaths: [][]string{
				[]string{"result", "toml", "consignmentID"},
			},
		},
		{
			Name:               "complex 1",
			Query:              `{$.List.Index(1).SomeSettings[@.Key.Equal("DEFE")].Any().Equal(true)}`,
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"List"},
			ExpectedAddressedPaths: [][]string{
				[]string{"List", "SomeSettings", "Key"},
			},
		},
		{
			Name:               "complex 2",
			Query:              `$.List[@.ID.AnyOf(1,2)].First().SomeSettings[@.Key.Equal("DEF")].First().Number`,
			Expect_decimal:     decimal.NewFromFloat(222),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"List"},
			ExpectedAddressedPaths: [][]string{
				[]string{"List", "ID"},
				[]string{"List", "SomeSettings", "Key"},
				[]string{"List", "SomeSettings", "Number"},
			},
		},
		{
			Name:               "complex 3",
			Query:              `$.List.First().SomeSettings[@.Key.Equal("ABC")].First().Number`,
			Expect_decimal:     decimal.NewFromFloat(1234),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"List"},
			ExpectedAddressedPaths: [][]string{
				[]string{"List", "SomeSettings", "Key"},
				[]string{"List", "SomeSettings", "Number"},
			},
		},
		{
			Name:               "simple 1",
			Query:              `$.List.Count()`,
			Expect_decimal:     decimal.NewFromFloat(4),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"List"},
			ExpectedAddressedPaths: [][]string{
				[]string{"List"},
			},
		},
		{
			Name:               "simple 2",
			Query:              `$.sTRinG`,
			Expect_string:      "abcDEF",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"sTRinG"},
			ExpectedAddressedPaths: [][]string{
				[]string{"sTRinG"},
			},
		},
		{
			Name:               "simple 3",
			Query:              `{OR,$.string.Equal("ABCD"),$.string.Equal("abcDEF")}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "simple 4",
			Query:              `$[@.index.Equal(1)].Any()`,
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{},
			ExpectedAddressedPaths: [][]string{
				[]string{"index"},
			},
		},
		{
			Name:               "simple 5",
			Query:              `$[@.index.Equal(6)].Any()`,
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{},
			ExpectedAddressedPaths: [][]string{
				[]string{"index"},
			},
		},
		{
			Name:               "simple 6",
			Query:              `{OR,$[@.index.Equal(1)].Any(),$[@.index.Equal(7)].Any()}`,
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{},
			ExpectedAddressedPaths: [][]string{
				[]string{"index"},
			},
		},
		{
			Name:               "simple 7",
			Query:              `{AND,{AND,$.index.Equal(6)}}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"index"},
			ExpectedAddressedPaths: [][]string{
				[]string{"index"},
			},
		},
		{
			Name:               "simple 8",
			Query:              `{$.index.Equal($.index)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"index"},
			ExpectedAddressedPaths: [][]string{
				[]string{"index"},
			},
		},
		{
			Name:               "simple 9",
			Query:              `{$.string.Equal($.string)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "simple 10",
			Query:              `{$.bool.Equal($.bool)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"bool"},
			ExpectedAddressedPaths: [][]string{
				[]string{"bool"},
			},
		},
		{
			Name:               "simple 11",
			Query:              `{$.numbers.First().AnyOf($.numbers)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"numbers"},
			ExpectedAddressedPaths: [][]string{
				[]string{"numbers"},
			},
		},
		{
			Name:               "simple 12",
			Query:              `{$.strings.First().AnyOf($.strings)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"strings"},
			ExpectedAddressedPaths: [][]string{
				[]string{"strings"},
			},
		},
		{
			Name:               "simple 13",
			Query:              `{$.bools.First().AnyOf($.bools)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"bools"},
			ExpectedAddressedPaths: [][]string{
				[]string{"bools"},
			},
		},
		{
			Name:               "simple 14",
			Query:              `{$.floats.First().AnyOf($.floats)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"floats"},
			ExpectedAddressedPaths: [][]string{
				[]string{"floats"},
			},
		},
		{
			Name:               "simple 15",
			Query:              `{$.ints.First().AnyOf($.ints)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"ints"},
			ExpectedAddressedPaths: [][]string{
				[]string{"ints"},
			},
		},
		{
			Name:               "simple 16",
			Query:              `{$.bools.Last().AnyOf(false)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"bools"},
			ExpectedAddressedPaths: [][]string{
				[]string{"bools"},
			},
		},

		{
			Name:               "func Less",
			Query:              `$.number.Less(10000)`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
		{
			Name:               "func LessOrEqual",
			Query:              `$.number.LessOrEqual(1234)`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
		{
			Name:               "func Greater",
			Query:              `$.number.Greater(1)`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
		{
			Name:               "func GreaterOrEqual",
			Query:              `$.number.GreaterOrEqual(1234)`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},

		{
			Name:               "func Equal",
			Query:              `$.string.Equal("abcDEF")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "func NotEqual",
			Query:              `$.string.NotEqual("abcDEFG")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "func Contains",
			Query:              `$.string.Contains("abc")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "func NotContains",
			Query:              `$.string.NotContains("zzzzz")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "func Prefix",
			Query:              `$.string.Prefix("ab")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "func NotPrefix",
			Query:              `$.string.NotPrefix("cd")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "func Suffix",
			Query:              `$.string.Suffix("EF")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "func NotSuffix",
			Query:              `$.string.NotSuffix("DE")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "func Index",
			Query:              `$.tags.Index(1)`,
			Expect_string:      "bbb",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"tags"},
			ExpectedAddressedPaths: [][]string{
				[]string{"tags"},
			},
		},

		{
			Name:               "func Sum",
			Query:              "$.number.Sum(1000,2000)",
			Expect_decimal:     decimal.NewFromFloat(4234),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
		{
			Name:               "func Average",
			Query:              "$.number.Average(5678)",
			Expect_decimal:     decimal.NewFromFloat(3456),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
		{
			Name:               "func Max",
			Query:              "$.number.Maximum(9999)",
			Expect_decimal:     decimal.NewFromFloat(9999),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
		{
			Name:               "func Min",
			Query:              "$.number.Minimum(9999)",
			Expect_decimal:     decimal.NewFromFloat(1234),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
		{
			Name:               "func Add",
			Query:              "$.number.Add(1)",
			Expect_decimal:     decimal.NewFromFloat(1235),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
		{
			Name:               "func Sub",
			Query:              "$.number.Subtract(2)",
			Expect_decimal:     decimal.NewFromFloat(1232),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
		{
			Name:               "func Div",
			Query:              "$.number.Divide(2)",
			Expect_decimal:     decimal.NewFromFloat(617),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
		{
			Name:               "func Mul",
			Query:              "$.number.Multiply(11)",
			Expect_decimal:     decimal.NewFromFloat(13574),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
		{
			Name:               "func Mod",
			Query:              "$.number.Modulo(100)",
			Expect_decimal:     decimal.NewFromFloat(34),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
		{
			Name:               "select many",
			Query:              "$.list.id.Sum(10)",
			Expect_decimal:     decimal.NewFromFloat(17),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"list"},
			ExpectedAddressedPaths: [][]string{
				[]string{"list", "id"},
			},
		},
		{
			Name:               "multiple addresses",
			Query:              "$.struct.field1.Equal($.struct.field2)",
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"struct"},
			ExpectedAddressedPaths: [][]string{
				[]string{"struct", "field2"},
				[]string{"struct", "field1"},
			},
		},
		{
			Name:               "func AsArray",
			Query:              "$.string.AsArray()",
			Expect_array:       []any{"abcDEF"},
			ExpectedResultType: RT_array,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		},
		{
			Name:               "func Not",
			Query:              "$.bool.Not()",
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"bool"},
			ExpectedAddressedPaths: [][]string{
				[]string{"bool"},
			},
		}, {
			Name:               "func IsNull",
			Query:              "$.isNull?.IsNull()",
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"isNull"},
			ExpectedAddressedPaths: [][]string{
				[]string{"isNull"},
			},
		}, {
			Name:               "func IsNotNull",
			Query:              "$.list.IsNotNull()",
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"list"},
			ExpectedAddressedPaths: [][]string{
				[]string{"list"},
			},
		}, {
			Name:               "func IsNullOrEmpty (on null)",
			Query:              "$.isNull.IsNullOrEmpty()",
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"isNull"},
			ExpectedAddressedPaths: [][]string{
				[]string{"isNull"},
			},
		}, {
			Name:               "func IsNullOrEmpty (on array = true)",
			Query:              "$.emptyArray.IsNullOrEmpty()",
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"emptyArray"},
			ExpectedAddressedPaths: [][]string{
				[]string{"emptyArray"},
			},
		}, {
			Name:               "func IsNullOrEmpty (on string = true)",
			Query:              "$.emptyString.IsNullOrEmpty()",
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"emptyString"},
			ExpectedAddressedPaths: [][]string{
				[]string{"emptyString"},
			},
		}, {
			Name:               "func IsNullOrEmpty (on object = true)",
			Query:              "$.emptyObject.IsNullOrEmpty()",
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"emptyObject"},
			ExpectedAddressedPaths: [][]string{
				[]string{"emptyObject"},
			},
		}, {
			Name:               "func IsNullOrEmpty (on number = true)",
			Query:              "$.emptyNumber.IsNullOrEmpty()",
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"emptyNumber"},
			ExpectedAddressedPaths: [][]string{
				[]string{"emptyNumber"},
			},
		}, {
			Name:               "func IsNullOrEmpty (on array = false)",
			Query:              "$.numbers.IsNullOrEmpty()",
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"numbers"},
			ExpectedAddressedPaths: [][]string{
				[]string{"numbers"},
			},
		}, {
			Name:               "func IsNullOrEmpty (on string = false)",
			Query:              "$.string.IsNullOrEmpty()",
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"string"},
			ExpectedAddressedPaths: [][]string{
				[]string{"string"},
			},
		}, {
			Name:               "func IsNullOrEmpty (on object = false)",
			Query:              "$.result.IsNullOrEmpty()",
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"result"},
			ExpectedAddressedPaths: [][]string{
				[]string{"result"},
			},
		}, {
			Name:               "func IsNullOrEmpty (on number = false)",
			Query:              "$.number.IsNullOrEmpty()",
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		}, {
			Name:               "func IsNotEmpty (on numbers = true)",
			Query:              "$.number.IsNotEmpty()",
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		}, {
			Name:               "func IsNotNullOrEmpty (on isNull = false)",
			Query:              "$.isNull.IsNotNullOrEmpty()",
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"isNull"},
			ExpectedAddressedPaths: [][]string{
				[]string{"isNull"},
			},
		}, {
			Name:               "null propagation",
			Query:              "$.isNull?.field?.does?.not?.exist?.IsNotNull()",
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"isNull"},
			ExpectedAddressedPaths: [][]string{
				[]string{"isNull", "field", "does", "not", "exist"},
			},
		}, {
			Name:               "null propagation 2",
			Query:              "$.isNull?.field?.does?.not?.exist?.IsNull()",
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			ExpectedRootFields: []string{"isNull"},
			ExpectedAddressedPaths: [][]string{
				[]string{"isNull", "field", "does", "not", "exist"},
			},
		}, {
			Name:               "null propagation error",
			Query:              "$.isNull?.field?.does?.not.exist?.IsNull()",
			Expect_error:       fmt.Errorf("key not found"),
			ExpectedResultType: RT_error,
			ExpectedRootFields: []string{"isNull"},
			ExpectedAddressedPaths: [][]string{
				[]string{"isNull", "field", "does", "not", "exist"},
			},
		}, {
			Name:               "does not exist",
			Query:              "$.xxx.Equal(\"test\")",
			Expect_error:       fmt.Errorf("key not found"),
			ExpectedResultType: RT_error,
			ExpectedRootFields: []string{"xxx"},
			ExpectedAddressedPaths: [][]string{
				[]string{"xxx"},
			},
		}, {
			Name:               "report generator first returns",
			Query:              "$.report_generator.result.First().ext",
			Expect_string:      ".pdf",
			ExpectedResultType: RT_string,
			ExpectedRootFields: []string{"report_generator"},
			ExpectedAddressedPaths: [][]string{
				[]string{"report_generator", "result", "ext"},
			},
		}, {
			Name:               "test add negative number",
			Query:              "$.number.Add(-1)",
			Expect_decimal:     decimal.NewFromInt(1233),
			ExpectedResultType: RT_decimal,
			ExpectedRootFields: []string{"number"},
			ExpectedAddressedPaths: [][]string{
				[]string{"number"},
			},
		},
	}
)

type ResultType string

const (
	RT_string  ResultType = "string"
	RT_decimal ResultType = "decimal"
	RT_bool    ResultType = "bool"
	RT_array   ResultType = "array"
	RT_error   ResultType = "error"
)

type CustomStringTypeForTest string

// Generated using https://json-generator.com/
var jsn = `
  {
	"report_generator": {
		"result": [
			{
				"ext": ".pdf",
				"id": "1836941291624075264",
				"name": "output_0.pdf"
			}
		]
	},
	"result": {
	  "json": "{\"consignmentID\":112357,\"consignmentName\":\"Test consignment\"}",
	  "xml": "<?xml version=\"1.0\" encoding=\"UTF-8\" ?><root><consignmentID>112358</consignmentID><consignmentName>Test consignment</consignmentName></root>",
	  "yaml": "---\nconsignmentID: 112359\nconsignmentName: Test consignment",
	  "toml": "consignmentID = 112_360\nconsignmentName = \"Test consignment\"\n"
	},
	"isNull": null,
	"emptyArray": [],	
	"emptyString": "",
	"emptyObject": {},
	"emptyNumber": 0,
	"number": 1234,
	"string": "abcDEF",
	"regexstring": "MSRWC1234567001",
	"numberInString": "12345",
	"bool": true,
	"numbers": [
	  1234,
	  5678
	],
	"ints": [
	  1234,
	  5678
	],
	"floats": [
	  1234.56,
	  5678.9
	],
	"strings": [
	  "abcDEF",
	  "HIJklm"
	],
	"bools": [
	  true,
	  false
	],
	"index": 6,
	"isActive": false,
	"tags": [
	  "aaa",
	  "bbb",
	  "ccc"
	],
	"struct": {
		"field1": "abcDEF",
		"field2": "abcDEF",
		"field3": "GJIjkl"
	},
	"list": [
	  {
		"id": 0,
		"name": "Bruce Whitney",
		"someSettings": [
		  {
			"Key": "ABC",
			"Number": 1234
		  },
		  {
			"Key": "DEF",
			"String": "ABCD"
		  }
		]
	  },
	  {
		"id": 1,
		"name": "Gladys Daugherty",
		"someSettings": [
		  {
			"Key": "DEF",
			"Number": 222
		  }
		]
	  },
	  {
		"id": 2,
		"name": "Myrna French"
	  },
	  {
		"id": 4,
		"name": "Bob Jones"
	  }
	]
  }
`

type TestDataStruct struct {
	Report_Generator struct {
		Result []struct {
			Ext  string `json:"ext"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
	} `json:"report_generator"`
	Result struct {
		JSON string `json:"json"`
		XML  string `json:"xml"`
		YAML string `json:"yaml"`
		TOML string `json:"toml"`
	} `json:"result"`
	IsNull         *struct{}         `json:"isNull"`
	EmptyArray     []struct{}        `json:"emptyArray"`
	EmptyString    string            `json:"emptyString"`
	EmptyObject    struct{}          `json:"emptyObject"`
	EmptyNumber    int               `json:"emptyNumber"`
	Number         int               `json:"number"`
	String         string            `json:"string"`
	RegexString    string            `json:"regexstring"`
	NumberInString string            `json:"numberInString"`
	Bool           bool              `json:"bool"`
	Numbers        []decimal.Decimal `json:"numbers"`
	Floats         []float64         `json:"floats"`
	Ints           []int             `json:"ints"`
	Strings        []string          `json:"strings"`
	Bools          []bool            `json:"bools"`
	Index          int               `json:"index"`
	IsActive       bool              `json:"isActive"`
	Tags           []string          `json:"tags"`
	Struct         struct {
		Field1 string `json:"field1"`
		Field2 string `json:"field2"`
		Field3 string `json:"field3"`
	} `json:"struct"`
	List []struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`
		SomeSettings []struct {
			Key    string `json:"Key"`
			Number int    `json:"Number,omitempty"`
			String string `json:"String,omitempty"`
		} `json:"someSettings,omitempty"`
	} `json:"list"`
}

func Test_sliceContains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputSlice []string
		inputValue string
		expected   bool
	}{
		{
			name:       "True",
			inputSlice: []string{"one", "two", "three", "four"},
			inputValue: "one",
			expected:   true,
		},
		{
			name:       "False",
			inputSlice: []string{"one", "two", "three", "four"},
			inputValue: "five",
			expected:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := sliceContains(test.inputSlice, test.inputValue)

			if result != test.expected {
				t.Fatalf("unexpected result: wanted: %v; got: %v", test.expected, result)
			}
		})
	}
}

func Test_spreadSlice(t *testing.T) {
	t.Parallel()

	input := []any{"one", "two", "three", "four"}
	expected := [][]any{
		[]any{"one"},
		[]any{"one", "two"},
		[]any{"one", "two", "three"},
		[]any{"one", "two", "three", "four"},
	}

	result := spreadSlice(input)

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("unexpected result: wanted: %v; got: %v", expected, input)
	}
}

func Test_sliceContainsSubsetSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputSlices [][]string
		inputSubset []string
		expected    bool
	}{
		{
			name: "True single",
			inputSlices: [][]string{
				[]string{"one", "two", "three", "four"},
			},
			inputSubset: []string{"one"},
			expected:    true,
		},
		{
			name: "True multiple",
			inputSlices: [][]string{
				[]string{"one", "two", "three", "four"},
			},
			inputSubset: []string{"one", "two"},
			expected:    true,
		},
		{
			name: "False single",
			inputSlices: [][]string{
				[]string{"one", "two", "three", "four"},
			},
			inputSubset: []string{"five"},
			expected:    false,
		}, {
			name: "False multiple",
			inputSlices: [][]string{
				[]string{"one", "two", "three", "four"},
			},
			inputSubset: []string{"five", "one", "two"},
			expected:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := slicesContainsSubsetSlice(test.inputSlices, test.inputSubset)

			if result != test.expected {
				t.Fatalf("unexpected result: wanted: %v; got: %v", test.expected, result)
			}
		})
	}
}

func Test_EscapeCharacters(t *testing.T) {
	t.Parallel()

	query := "$.result.Equal(\"Line1,\\nLine2,\\nLin\\\"e3\\n\")"
	data := map[string]any{
		"result": "Line1,\nLine2,\nLin\"e3\n",
	}

	op, err := ParseString(query)

	if err != nil {
		t.Fatalf("failed to parse string: %s", err)
	}

	result, err := op.Do(data, data)

	if err != nil {
		t.Fatalf("failed to do: %s", err)
	}

	t.Log("result:", result)

	resBool, ok := result.(bool)
	if !ok {
		t.Fatalf("result is not a bool")
	}

	if !resBool {
		t.Fatalf("result is not true")
	}
}

func Test_UnescapeCharacters(t *testing.T) {
	t.Parallel()

	query := "$.result.Equal(\"Line1,\\nLine2,\\nLin\\\"e3\\n\")"

	op, err := ParseString(query)

	if err != nil {
		t.Fatalf("failed to parse string: %s", err)
	}

	str := op.Sprint(0)

	t.Log("str \\n query:\n", str, "\n", query)

	if query != str {
		t.Fatalf("query is not the same after sprint")
	}
}
