package mpath

import (
	"encoding/json"
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

	b.Log("Parse:")

	for _, test := range testQueries {
		b.Run("Parse "+test.Name, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				b.ReportAllocs()
				op, _, err = ParseString(test.Query)

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
			op, _, err = ParseString(test.Query)

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
				op, _, err = ParseString(test.Query)

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

func Test_Sprint(t *testing.T) {
	// We need only test that the Sprint doesn't throw an error
	for _, test := range testQueries {
		op, _, err := ParseString(test.Query)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		if len(op.Sprint(0)) == 0 {
			t.Errorf("Got 0 length Sprint string")
		}
	}
}

func Test_ForPath(t *testing.T) {
	var onlyRun string

	// onlyRun = "complex 1"

	for _, test := range testQueries {
		if onlyRun != "" && test.Name != onlyRun {
			continue
		}

		_, forPath, err := ParseString(test.Query)
		if err != nil {
			t.Error(err)
		}

		gotError := false

		if lGot, lWanted := len(forPath), len(test.Expect_ForPath); lGot != lWanted {
			t.Errorf("'%s': got bad forPath length: wanted: %d; got: %d", test.Name, lWanted, lGot)
			gotError = true
			goto Error
		}

		for i, fpWanted := range test.Expect_ForPath {
			fpGot := forPath[i]
			if len(fpWanted) != len(fpGot) {
				t.Errorf("'%s': got bad forPath length at index %d: wanted: %d; got: %d", test.Name, i, len(fpWanted), len(fpGot))
				gotError = true
				continue
			}

			for ii, fpWI := range fpWanted {
				fpGI := fpGot[ii]

				if fpGI != fpWI {
					gotError = true
					t.Errorf("'%s': wrong forPath value at index %d: wanted: '%s'; got: '%s'", test.Name, ii, fpWI, fpGI)
				}
			}
		}

	Error:
		if gotError {
			for _, w := range test.Expect_ForPath {
				t.Errorf("'%s': wanted: %v", test.Name, w)
			}
			for _, g := range forPath {
				t.Errorf("'%s': got: %v", test.Name, g)
			}
		}
	}
}

func Test_ParseErrors(t *testing.T) {
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
		op, _, err := ParseString(test.Query)
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

func Test_DecimalAtRoot(t *testing.T) {
	const testName = "Test_DecimalAtRoot"

	data := float64(40)
	op, _, err := ParseString("$")
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

	onlyRunName := ""

	//`{$.List.Last().SomeSettings[@.Key.Equal("DEF")].Any().Equal(true)}`
	// onlyRunName = "complex 1"

	for _, data := range datas {
		for _, test := range testQueries {
			if onlyRunName != "" {
				if test.Name != onlyRunName {
					continue
				}
			}

			op, _, err := ParseString(test.Query)

			if err != nil { // This is to avoid nil dereference errors
				t.Errorf("'%s' has error: %v,", test.Name, err)
				continue
			}

			if op == nil {
				t.Errorf("'%s' failed to return an operation: %s", test.Name, test.Query)
				continue
			}

			dataToUse, err := op.Do(data, data)
			if err != nil {
				t.Errorf("'%s' got error from Do(): %v", test.Name, err)
				continue
			}

			switch test.ExpectedResultType {
			case RT_string:
				if d, ok := dataToUse.(string); !ok {
					t.Errorf("'%s' data was not of expected type '%s'; was %T", test.Name, test.ExpectedResultType, dataToUse)
				} else {
					if d != test.Expect_string {
						t.Errorf("'%s' data did not match expected value '%s': got %s", test.Name, test.Expect_string, d)
					}
				}
			case RT_decimal:
				if d, ok := dataToUse.(decimal.Decimal); !ok {
					t.Errorf("'%s' data was not of expected type '%s'; was %T", test.Name, test.ExpectedResultType, dataToUse)
				} else {
					if !d.Equal(test.Expect_decimal) {
						t.Errorf("'%s' data did not match expected value '%s': got %s", test.Name, test.Expect_decimal, d)
					}
				}
			case RT_bool:
				if d, ok := dataToUse.(bool); !ok {
					t.Errorf("'%s' data was not of expected type '%s'; was %T", test.Name, test.ExpectedResultType, dataToUse)
				} else {
					if d != test.Expect_bool {
						t.Errorf("'%s' data did not match expected value '%t': got %t", test.Name, test.Expect_bool, d)
					}
				}
			}
		}
	}
}

func Test_CustomStringTypeMap(t *testing.T) {
	const outStrConst = "hello world"
	data := map[CustomStringTypeForTest]any{"varname": outStrConst}
	queryString := "$.varname"

	op, _, err := ParseString(queryString)

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
		Name               string
		Query              string
		ExpectedResultType ResultType
		Expect_string      string
		Expect_decimal     decimal.Decimal
		Expect_bool        bool
		Expect_ForPath     [][]string
	}{
		// { //todo: add expected error states into the tests (this should error)
		// 	Name:               "Test misspelt operation type",
		// 	Query:              `{AN,$.string.Equals("test")}`,
		// 	Expect_bool:        false,
		// 	ExpectedResultType: RT_bool,
		// 	// Expect_ForPath:     [][]string{{"numberInString"}},
		// },
		{
			Name:               "Add number to string number",
			Query:              `$.numberInString.Add(21111.123)`,
			Expect_decimal:     decimal.NewFromFloat(33456.123),
			ExpectedResultType: RT_decimal,
			Expect_ForPath:     [][]string{{"numberInString"}},
		},
		{
			Name:               "Add string number to string number",
			Query:              `$.numberInString.Add("21111.123")`,
			Expect_decimal:     decimal.NewFromFloat(33456.123),
			ExpectedResultType: RT_decimal,
			Expect_ForPath:     [][]string{{"numberInString"}},
		},
		{
			Name:               "trim right of string by n",
			Query:              `$.string.TrimRight(3)`,
			Expect_string:      "abc",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"string"}},
		},
		{
			Name:               "trim right of string by n > length of string",
			Query:              `$.string.TrimRight(7)`,
			Expect_string:      "",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"string"}},
		},
		{
			Name:               "trim right of string by n = length of string",
			Query:              `$.string.TrimRight(6)`,
			Expect_string:      "",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"string"}},
		},
		{
			Name:               "trim right of string by n = 0",
			Query:              `$.string.TrimRight(0)`,
			Expect_string:      "abcDEF",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"string"}},
		},

		{
			Name:               "trim left of string by n",
			Query:              `$.string.TrimLeft(3)`,
			Expect_string:      "DEF",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"string"}},
		},
		{
			Name:               "trim left of string by n > length of string",
			Query:              `$.string.TrimLeft(7)`,
			Expect_string:      "",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"string"}},
		},
		{
			Name:               "trim left of string by n = length of string",
			Query:              `$.string.TrimLeft(6)`,
			Expect_string:      "",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"string"}},
		},
		{
			Name:               "trim left of string by n = 0",
			Query:              `$.string.TrimLeft(0)`,
			Expect_string:      "abcDEF",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"string"}},
		},

		{
			Name:               "get left n of string",
			Query:              `$.string.Left(2)`,
			Expect_string:      "ab",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"string"}},
		},
		{
			Name:               "get right n of string",
			Query:              `$.string.Right(2)`,
			Expect_string:      "EF",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"string"}},
		},

		{
			Name:               "get left n > len of string",
			Query:              `$.string.Left(7)`,
			Expect_string:      "abcDEF",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"string"}},
		},
		{
			Name:               "get right n > len of string",
			Query:              `$.string.Right(7)`,
			Expect_string:      "abcDEF",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"string"}},
		},

		{
			Name:               "regex matches string",
			Query:              `$.string.DoesMatchRegex("a[bc]+[A-Za-z]+")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath:     [][]string{{"string"}},
		},
		{
			Name:               "regex doesn't match string",
			Query:              `$.string.DoesMatchRegex("z[bc]+[A-Za-z]+")`,
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			Expect_ForPath:     [][]string{{"string"}},
		},

		{
			Name:               "replace regex",
			Query:              `$.string.ReplaceRegex("(a)","z")`,
			Expect_string:      "zbcDEF",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"string"}},
		},

		{
			Name:               "replace regex $",
			Query:              `$.regexstring.ReplaceRegex("^(MSRWC)([a-zA-Z0-9]{7})(.*)","$1$2")`,
			Expect_string:      "MSRWC1234567",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"regexstring"}},
		},

		{
			Name:               "replace all",
			Query:              `$.string.ReplaceAll("a","z")`,
			Expect_string:      "zbcDEF",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"string"}},
		},

		{
			Name:               "get data from string JSON field",
			Query:              `$.result.json.ParseJSON().consignmentID`,
			Expect_decimal:     decimal.NewFromInt(112357),
			ExpectedResultType: RT_decimal,
			Expect_ForPath:     [][]string{{"result", "json"}},
		},
		{
			Name:               "get data from string JSON field, then put it back to JSON",
			Query:              `$.result.json.ParseJSON().AsJSON()`,
			Expect_string:      "{\"consignmentID\":112357,\"consignmentName\":\"Test consignment\"}",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"result", "json"}},
		},
		{
			Name:               "get data from string JSON field, select a field, then put it back to JSON",
			Query:              `$.result.json.ParseJSON().consignmentID.AsJSON()`,
			Expect_string:      `"112357"`,
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"result", "json"}},
		},
		{
			Name:               "get data from string XML field",
			Query:              `$.result.xml.ParseXML().root.consignmentID`,
			Expect_string:      "112358",
			ExpectedResultType: RT_string,
			Expect_ForPath:     [][]string{{"result", "xml"}},
		},
		{
			Name:               "get data from string YAML field",
			Query:              `$.result.yaml.ParseYAML().consignmentID`,
			Expect_decimal:     decimal.NewFromInt(112359),
			ExpectedResultType: RT_decimal,
			Expect_ForPath:     [][]string{{"result", "yaml"}},
		},
		{
			Name:               "get data from string TOML field",
			Query:              `$.result.toml.ParseTOML().consignmentID`,
			Expect_decimal:     decimal.NewFromInt(112360),
			ExpectedResultType: RT_decimal,
			Expect_ForPath:     [][]string{{"result", "toml"}},
		},
		{
			Name:               "complex 1",
			Query:              `{$.List.Last().SomeSettings[@.Key.Equal("DEF")].Any().Equal(true)}`,
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"List", "SomeSettings"},
				{"List", "SomeSettings", "Key"},
			},
		},
		{
			Name:               "complex 2",
			Query:              `$.List[@.ID.AnyOf(1,2)].First().SomeSettings[@.Key.Equal("DEF")].First().Number`,
			Expect_decimal:     decimal.NewFromFloat(222),
			ExpectedResultType: RT_decimal,
			Expect_ForPath: [][]string{
				{"List", "ID"},
				{"List", "SomeSettings", "Key"},
				{"List", "SomeSettings", "Number"},
			},
		},
		{
			Name:               "complex 3",
			Query:              `$.List.First().SomeSettings[@.Key.Equal("ABC")].First().Number`,
			Expect_decimal:     decimal.NewFromFloat(1234),
			ExpectedResultType: RT_decimal,
			Expect_ForPath: [][]string{
				{"List", "SomeSettings", "Key"},
				{"List", "SomeSettings", "Number"},
			},
		},
		{
			Name:               "simple 1",
			Query:              `$.List.Count()`,
			Expect_decimal:     decimal.NewFromFloat(4),
			ExpectedResultType: RT_decimal,
			Expect_ForPath: [][]string{
				{"List"},
			},
		},
		{
			Name:               "simple 2",
			Query:              `$.sTRinG`,
			Expect_string:      "abcDEF",
			ExpectedResultType: RT_string,
			Expect_ForPath: [][]string{
				{"sTRinG"},
			},
		},
		{
			Name:               "simple 3",
			Query:              `{OR,$.string.Equal("ABCD"),$.string.Equal("abcDEF")}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"string"},
				{"string"},
			},
		},
		{
			Name:               "simple 4",
			Query:              `$[@.index.Equal(1)].Any()`,
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"index"},
			},
		},
		{
			Name:               "simple 5",
			Query:              `$[@.index.Equal(6)].Any()`,
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"index"},
			},
		},
		{
			Name:               "simple 6",
			Query:              `{OR,$[@.index.Equal(1)].Any(),$[@.index.Equal(7)].Any()}`,
			Expect_bool:        false,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"index"},
				{"index"},
			},
		},
		{
			Name:               "simple 7",
			Query:              `{AND,{AND,$.index.Equal(6)}}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"index"},
			},
		},
		{
			Name:               "simple 8",
			Query:              `{$.index.Equal($.index)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"index"},
			},
		},
		{
			Name:               "simple 9",
			Query:              `{$.string.Equal($.string)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"string"},
			},
		},
		{
			Name:               "simple 10",
			Query:              `{$.bool.Equal($.bool)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"bool"},
			},
		},
		{
			Name:               "simple 11",
			Query:              `{$.numbers.First().AnyOf($.numbers)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"numbers"},
			},
		},
		{
			Name:               "simple 12",
			Query:              `{$.strings.First().AnyOf($.strings)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"strings"},
			},
		},
		{
			Name:               "simple 13",
			Query:              `{$.bools.First().AnyOf($.bools)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"bools"},
			},
		},
		{
			Name:               "simple 14",
			Query:              `{$.floats.First().AnyOf($.floats)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"floats"},
			},
		},
		{
			Name:               "simple 15",
			Query:              `{$.ints.First().AnyOf($.ints)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"ints"},
			},
		},
		{
			Name:               "simple 16",
			Query:              `{$.bools.Last().AnyOf(false)}`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"bools"},
			},
		},

		{
			Name:               "func Less",
			Query:              `$.number.Less(10000)`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"number"},
			},
		},
		{
			Name:               "func LessOrEqual",
			Query:              `$.number.LessOrEqual(1234)`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"number"},
			},
		},
		{
			Name:               "func Greater",
			Query:              `$.number.Greater(1)`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"number"},
			},
		},
		{
			Name:               "func GreaterOrEqual",
			Query:              `$.number.GreaterOrEqual(1234)`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"number"},
			},
		},

		{
			Name:               "func Equal",
			Query:              `$.string.Equal("abcDEF")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"string"},
			},
		},
		{
			Name:               "func NotEqual",
			Query:              `$.string.NotEqual("abcDEFG")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"string"},
			},
		},
		{
			Name:               "func Contains",
			Query:              `$.string.Contains("abc")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"string"},
			},
		},
		{
			Name:               "func NotContains",
			Query:              `$.string.NotContains("zzzzz")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"string"},
			},
		},
		{
			Name:               "func Prefix",
			Query:              `$.string.Prefix("ab")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"string"},
			},
		},
		{
			Name:               "func NotPrefix",
			Query:              `$.string.NotPrefix("cd")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"string"},
			},
		},
		{
			Name:               "func Suffix",
			Query:              `$.string.Suffix("EF")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"string"},
			},
		},
		{
			Name:               "func NotSuffix",
			Query:              `$.string.NotSuffix("DE")`,
			Expect_bool:        true,
			ExpectedResultType: RT_bool,
			Expect_ForPath: [][]string{
				{"string"},
			},
		},
		{
			Name:               "func Index",
			Query:              `$.tags.Index(1)`,
			Expect_string:      "bbb",
			ExpectedResultType: RT_string,
			Expect_ForPath: [][]string{
				{"tags"},
			},
		},

		{
			Name:               "func Sum",
			Query:              "$.number.Sum(1000,2000)",
			Expect_decimal:     decimal.NewFromFloat(4234),
			ExpectedResultType: RT_decimal,
			Expect_ForPath: [][]string{
				{"number"},
			},
		},
		{
			Name:               "func Average",
			Query:              "$.number.Average(5678)",
			Expect_decimal:     decimal.NewFromFloat(3456),
			ExpectedResultType: RT_decimal,
			Expect_ForPath: [][]string{
				{"number"},
			},
		},
		{
			Name:               "func Max",
			Query:              "$.number.Maximum(9999)",
			Expect_decimal:     decimal.NewFromFloat(9999),
			ExpectedResultType: RT_decimal,
			Expect_ForPath: [][]string{
				{"number"},
			},
		},
		{
			Name:               "func Min",
			Query:              "$.number.Minimum(9999)",
			Expect_decimal:     decimal.NewFromFloat(1234),
			ExpectedResultType: RT_decimal,
			Expect_ForPath: [][]string{
				{"number"},
			},
		},
		{
			Name:               "func Add",
			Query:              "$.number.Add(1)",
			Expect_decimal:     decimal.NewFromFloat(1235),
			ExpectedResultType: RT_decimal,
			Expect_ForPath: [][]string{
				{"number"},
			},
		},
		{
			Name:               "func Sub",
			Query:              "$.number.Subtract(2)",
			Expect_decimal:     decimal.NewFromFloat(1232),
			ExpectedResultType: RT_decimal,
			Expect_ForPath: [][]string{
				{"number"},
			},
		},
		{
			Name:               "func Div",
			Query:              "$.number.Divide(2)",
			Expect_decimal:     decimal.NewFromFloat(617),
			ExpectedResultType: RT_decimal,
			Expect_ForPath: [][]string{
				{"number"},
			},
		},
		{
			Name:               "func Mul",
			Query:              "$.number.Multiply(11)",
			Expect_decimal:     decimal.NewFromFloat(13574),
			ExpectedResultType: RT_decimal,
			Expect_ForPath: [][]string{
				{"number"},
			},
		},
		{
			Name:               "func Mod",
			Query:              "$.number.Modulo(100)",
			Expect_decimal:     decimal.NewFromFloat(34),
			ExpectedResultType: RT_decimal,
			Expect_ForPath: [][]string{
				{"number"},
			},
		},
		{
			Name:               "select many",
			Query:              "$.list.id.Sum(10)",
			Expect_decimal:     decimal.NewFromFloat(17),
			ExpectedResultType: RT_decimal,
			Expect_ForPath: [][]string{
				{"list", "id"},
			},
		},
	}
)

type ResultType string

const (
	RT_string  ResultType = "string"
	RT_decimal ResultType = "decimal"
	RT_bool    ResultType = "bool"
)

type CustomStringTypeForTest string

// Generated using https://json-generator.com/
var jsn = `
  {
	"result": {
	  "json": "{\"consignmentID\":112357,\"consignmentName\":\"Test consignment\"}",
	  "xml": "<?xml version=\"1.0\" encoding=\"UTF-8\" ?><root><consignmentID>112358</consignmentID><consignmentName>Test consignment</consignmentName></root>",
	  "yaml": "---\nconsignmentID: 112359\nconsignmentName: Test consignment",
	  "toml": "consignmentID = 112_360\nconsignmentName = \"Test consignment\"\n"
	},
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
	Result struct {
		JSON string `json:"json"`
		XML  string `json:"xml"`
		YAML string `json:"yaml"`
		TOML string `json:"toml"`
	} `json:"result"`
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
	List           []struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`
		SomeSettings []struct {
			Key    string `json:"Key"`
			Number int    `json:"Number,omitempty"`
			String string `json:"String,omitempty"`
		} `json:"someSettings,omitempty"`
	} `json:"list"`
}
