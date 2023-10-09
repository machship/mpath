package mpath

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	xj "github.com/basgys/goxml2json"
	"github.com/pelletier/go-toml/v2"
	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v2"
)

type PT_ParameterType string

const (
	PT_String                 PT_ParameterType = "String"
	PT_Number                 PT_ParameterType = "Number"
	PT_Array                  PT_ParameterType = "Array"
	PT_ArrayOfNumbers         PT_ParameterType = "ArrayOfNumbers"
	PT_NumberOrArrayOfNumbers PT_ParameterType = "NumberOrArrayOfNumbers"
	PT_SameAsInput            PT_ParameterType = "SameAsInput"
	PT_Any                    PT_ParameterType = "Any"
	PT_MapStringOfAny         PT_ParameterType = "MapStringOfAny"
)

type FunctionDescriptor struct {
	Name        string
	Description string
	Params      []ParameterDescriptor
	ValidOn     PT_ParameterType
	Returns     PT_ParameterType
	fn          func(rtParams runtimeParams, val any) (any, error)
}

type ParameterDescriptor struct {
	Name string
	Type PT_ParameterType
}

func paramsGetFirstOfAny(rtParams runtimeParams) (val any, err error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return nil, fmt.Errorf("expected %d params, got %d", 1, got)
	}

	for _, p := range rtParams.paramsNumber {
		return p, nil
	}

	for _, p := range rtParams.paramsString {
		return p, nil
	}

	for _, p := range rtParams.paramsBool {
		return p, nil
	}

	return nil, fmt.Errorf("no parameters found")
}

func paramsGetFirstOfNumber(rtParams runtimeParams) (val decimal.Decimal, err error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return val, fmt.Errorf("expected %d params, got %d", 1, got)
	}

	for _, p := range rtParams.paramsNumber {
		return p, nil
	}

	for _, p := range rtParams.paramsString {
		if wasNumber, number := convertToDecimalIfNumberAndCheck(p); wasNumber {
			return number, nil
		}
	}

	return val, fmt.Errorf("no number parameter found")
}

func paramsGetFirstOfString(rtParams runtimeParams) (val string, err error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return val, fmt.Errorf("expected %d params, got %d", 1, got)
	}

	for _, p := range rtParams.paramsString {
		return p, nil
	}

	return val, fmt.Errorf("no string parameter found")
}

func paramsGetAll(rtParams runtimeParams) (val []any, err error) {
	for _, p := range rtParams.paramsNumber {
		val = append(val, p)
	}

	for _, p := range rtParams.paramsString {
		val = append(val, p)
	}

	for _, p := range rtParams.paramsBool {
		val = append(val, p)
	}

	return
}

func (rtParams *runtimeParams) checkLengthOfParams(allowed int) (got int, ok bool) {
	got = len(rtParams.paramsNumber) +
		len(rtParams.paramsString) +
		len(rtParams.paramsBool)

	if allowed == -1 {
		return got, true
	}

	return got, allowed == got
}

func errBool(name string, err error) (bool, error) {
	return false, fmt.Errorf("func %s: %w", name, err)
}

func errString(name string, err error) (string, error) {
	return "", fmt.Errorf("func %s: %w", name, err)
}

func errNumParams(name string, expected, got int) error {
	return fmt.Errorf("(%s) expected %d params, got %d", name, expected, got)
}

const FN_Equal = "Equal"

func func_Equal(rtParams runtimeParams, val any) (any, error) {
	param, err := paramsGetFirstOfAny(rtParams)
	if err != nil {
		return errBool(FN_Equal, err)
	}

	switch vt := val.(type) {
	case decimal.Decimal:
		switch pt := param.(type) {
		case decimal.Decimal:
			return vt.Equal(pt), nil
		}
		return false, nil
	}

	return val == param, nil
}

const FN_NotEqual = "NotEqual"

func func_NotEqual(rtParams runtimeParams, val any) (bool, error) {
	param, err := paramsGetFirstOfAny(rtParams)
	if err != nil {
		return errBool(FN_NotEqual, err)
	}

	switch vt := val.(type) {
	case decimal.Decimal:
		switch pt := param.(type) {
		case decimal.Decimal:
			return !vt.Equal(pt), nil
		}
		return true, nil
	}

	return val != param, nil
}

func decimalBoolFunc(rtParams runtimeParams, val any, fn func(d1, d2 decimal.Decimal) bool, fnName string) (bool, error) {
	param, err := paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return errBool(fnName, err)
	}

	if valIfc, ok := val.(decimal.Decimal); ok {
		return fn(valIfc, param), nil
	}

	return false, fmt.Errorf("parameter wasn't number")
}

const FN_Less = "Less"

func func_Less(rtParams runtimeParams, val any) (any, error) {
	return decimalBoolFunc(rtParams, val, func(d1, d2 decimal.Decimal) bool {
		return d1.LessThan(d2)
	}, FN_Less)
}

const FN_LessOrEqual = "LessOrEqual"

func func_LessOrEqual(rtParams runtimeParams, val any) (any, error) {
	return decimalBoolFunc(rtParams, val, func(d1, d2 decimal.Decimal) bool {
		return d1.LessThanOrEqual(d2)
	}, FN_LessOrEqual)
}

const FN_Greater = "Greater"

func func_Greater(rtParams runtimeParams, val any) (any, error) {
	return decimalBoolFunc(rtParams, val, func(d1, d2 decimal.Decimal) bool {
		return d1.GreaterThan(d2)
	}, FN_Greater)
}

const FN_GreaterOrEqual = "GreaterOrEqual"

func func_GreaterOrEqual(rtParams runtimeParams, val any) (any, error) {
	return decimalBoolFunc(rtParams, val, func(d1, d2 decimal.Decimal) bool {
		return d1.GreaterThanOrEqual(d2)
	}, FN_GreaterOrEqual)
}

func stringBoolFunc(rtParams runtimeParams, val any, fn func(string, string) bool, invert bool, fnName string) (bool, error) {
	param, err := paramsGetFirstOfString(rtParams)
	if err != nil {
		return errBool(fnName, err)
	}

	if valIfc, ok := val.(string); ok {
		res := fn(valIfc, param)

		if invert {
			res = !res
		}

		return res, nil
	}

	return false, fmt.Errorf("parameter wasn't string")
}

const FN_Contains = "Contains"

func func_Contains(rtParams runtimeParams, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.Contains, false, FN_Contains)
}

const FN_NotContains = "NotContains"

func func_NotContains(rtParams runtimeParams, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.Contains, true, FN_NotContains)
}

const FN_Prefix = "Prefix"

func func_Prefix(rtParams runtimeParams, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.HasPrefix, false, FN_Prefix)
}

const FN_NotPrefix = "NotPrefix"

func func_NotPrefix(rtParams runtimeParams, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.HasPrefix, true, FN_NotPrefix)
}

const FN_Suffix = "Suffix"

func func_Suffix(rtParams runtimeParams, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.HasSuffix, false, FN_Suffix)
}

const FN_NotSuffix = "NotSuffix"

func func_NotSuffix(rtParams runtimeParams, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.HasSuffix, true, FN_NotSuffix)
}

const FN_Count = "Count"

func func_Count(rtParams runtimeParams, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return decimal.Zero, fmt.Errorf("(%s) expected %d params, got %d", FN_Count, 0, got)
	}

	v := reflect.ValueOf(val)
	if isEmptyValue(v) {
		return decimal.Zero, nil
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		return decimal.NewFromInt(int64(v.Len())), nil
	}

	return decimal.Zero, nil
}

const FN_Any = "Any"

func func_Any(rtParams runtimeParams, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return false, errNumParams(FN_Any, 0, got)
	}

	v := reflect.ValueOf(val)
	if isEmptyValue(v) {
		return false, nil
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		return v.Len() > 0, nil
	case reflect.Struct:
		return v.IsZero(), nil
	}

	return false, nil
}

const FN_First = "First"

func func_First(rtParams runtimeParams, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return false, errNumParams(FN_First, 0, got)
	}

	v := reflect.ValueOf(val)
	if isEmptyValue(v) {
		return decimal.Zero, nil
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		if v.Len() > 0 {
			return convertToDecimalIfNumber(v.Index(0).Interface()), nil
		} else {
			return nil, fmt.Errorf("nothing in array")
		}
	}

	return false, fmt.Errorf("not array")
}

const FN_Last = "Last"

func func_Last(rtParams runtimeParams, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return false, errNumParams(FN_Last, 0, got)
	}

	v := reflect.ValueOf(val)
	if isEmptyValue(v) {
		return decimal.Zero, nil
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		if v.Len() > 0 {
			return convertToDecimalIfNumber(v.Index(v.Len() - 1).Interface()), nil
		} else {
			return nil, fmt.Errorf("nothing in array")
		}
	}

	return false, fmt.Errorf("not array")
}

const FN_Index = "Index"

func func_Index(rtParams runtimeParams, val any) (any, error) {
	param, err := paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return errBool(FN_Index, err)
	}

	v := reflect.ValueOf(val)
	if isEmptyValue(v) {
		return decimal.Zero, nil
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		if v.Len()-1 >= int(param.IntPart()) {
			return convertToDecimalIfNumber(v.Index(int(param.IntPart())).Interface()), nil
		} else {
			return nil, fmt.Errorf("nothing in array")
		}
	}

	return false, fmt.Errorf("not array")
}

func func_decimalSlice(rtParams runtimeParams, val any, decimalSliceFunction func(decimal.Decimal, ...decimal.Decimal) decimal.Decimal) (any, error) {
	if vd, ok := val.(decimal.Decimal); ok {
		val = []decimal.Decimal{vd}
	}

	var paramNumbers []decimal.Decimal
	paramNumbers = append(paramNumbers, rtParams.paramsNumber...)
	for _, ps := range rtParams.paramsString {
		if wasNumber, number := convertToDecimalIfNumberAndCheck(ps); wasNumber {
			paramNumbers = append(paramNumbers, number)
		}
	}

	var newSlc []decimal.Decimal
	if valIfc, ok := val.([]decimal.Decimal); ok {
		newSlc = append([]decimal.Decimal{}, valIfc...)
		newSlc = append(newSlc, paramNumbers...)
	} else if valIfc, ok := val.([]any); ok {
		newSlc = append([]decimal.Decimal{}, paramNumbers...)
		for _, vs := range valIfc {
			if vd, ok := vs.(decimal.Decimal); ok {
				newSlc = append(newSlc, vd)
			} else if vd, ok := vs.(string); ok {
				// Check if the string can be converted to an integer
				wasNumber, number := convertToDecimalIfNumberAndCheck(vd)
				if wasNumber {
					newSlc = append(newSlc, number)
					continue
				}
				goto notArrayOfNumbers
			} else {
				goto notArrayOfNumbers
			}
		}
	}

	if len(newSlc) == 0 {
		return decimal.Zero, nil
	}

	if len(newSlc) == 1 {
		return newSlc[0], nil
	}

	return decimalSliceFunction(newSlc[0], newSlc[1:]...), nil

notArrayOfNumbers:
	return false, fmt.Errorf("not array of numbers")
}

const FN_Sum = "Sum"

func func_Sum(rtParams runtimeParams, val any) (any, error) {
	return func_decimalSlice(rtParams, val, decimal.Sum)
}

const FN_Avg = "Avg"

func func_Avg(rtParams runtimeParams, val any) (any, error) {
	return func_decimalSlice(rtParams, val, decimal.Avg)
}

const FN_Min = "Min"

func func_Min(rtParams runtimeParams, val any) (any, error) {
	return func_decimalSlice(rtParams, val, decimal.Min)
}

const FN_Max = "Max"

func func_Max(rtParams runtimeParams, val any) (any, error) {
	return func_decimalSlice(rtParams, val, decimal.Max)
}

func func_decimal(rtParams runtimeParams, val any, decSlcFunc func(decimal.Decimal, decimal.Decimal) decimal.Decimal, name string) (any, error) {
	param, err := paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return errBool(name, err)
	}

	if valIfc, ok := val.(decimal.Decimal); ok {
		return decSlcFunc(valIfc, param), nil
	}

	return false, fmt.Errorf("not a number")
}

const FN_Add = "Add"

func func_Add(rtParams runtimeParams, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Add, FN_Add)
}

const FN_Sub = "Sub"

func func_Sub(rtParams runtimeParams, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Sub, FN_Sub)
}

const FN_Div = "Div"

func func_Div(rtParams runtimeParams, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Div, FN_Div)
}

const FN_Mul = "Mul"

func func_Mul(rtParams runtimeParams, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Mul, FN_Mul)
}

const FN_Mod = "Mod"

func func_Mod(rtParams runtimeParams, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Mod, FN_Mod)
}

const FN_AnyOf = "AnyOf"

func func_AnyOf(rtParams runtimeParams, val any) (any, error) {
	params, err := paramsGetAll(rtParams)
	if err != nil {
		return errBool(FN_AnyOf, err)
	}

	for _, p := range params {
		switch vt := val.(type) {
		case decimal.Decimal:
			switch pt := p.(type) {
			case decimal.Decimal:
				if vt.Equal(pt) {
					return true, nil
				}
				continue
			}
			return false, nil
		}

		if val == p {
			return true, nil
		}
	}

	return false, nil
}

func stringPartFunc(rtParams runtimeParams, val any, fn func(string, int) (string, error), fnName string) (string, error) {
	param, err := paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return errString(fnName, err)
	}

	if !param.IsInteger() {
		return "", fmt.Errorf("parameter must be an integer")
	}

	paramAsInt := int(param.IntPart())

	if valIfc, ok := val.(string); ok {
		return fn(valIfc, paramAsInt)
	}

	return "", fmt.Errorf("value wasn't string")
}

const FN_TrimRightN = "TrimRightN"

func func_TrimRightN(rtParams runtimeParams, val any) (any, error) {
	return stringPartFunc(rtParams, val, func(s string, i int) (string, error) {
		if len(s) <= i {
			return "", nil
		}

		return s[:len(s)-i], nil
	}, FN_TrimRightN)
}

const FN_TrimLeftN = "TrimLeftN"

func func_TrimLeftN(rtParams runtimeParams, val any) (any, error) {
	return stringPartFunc(rtParams, val, func(s string, i int) (string, error) {
		if len(s) <= i {
			return "", nil
		}

		return s[i:], nil
	}, FN_TrimLeftN)
}

const FN_RightN = "RightN"

func func_RightN(rtParams runtimeParams, val any) (any, error) {
	return stringPartFunc(rtParams, val, func(s string, i int) (string, error) {
		if len(s) < i {
			return s, nil
		}

		return s[len(s)-i:], nil
	}, FN_RightN)
}

const FN_LeftN = "LeftN"

func func_LeftN(rtParams runtimeParams, val any) (any, error) {
	return stringPartFunc(rtParams, val, func(s string, i int) (string, error) {
		if len(s) < i {
			return s, nil
		}

		return s[:i], nil
	}, FN_LeftN)
}

const FN_DoesMatchRegex = "DoesMatchRegex"

func func_DoesMatchRegex(rtParams runtimeParams, val any) (any, error) {
	param, err := paramsGetFirstOfString(rtParams)
	if err != nil {
		return errBool(FN_DoesMatchRegex, err)
	}

	exp, err := regexp.Compile(param)
	if err != nil {
		return false, fmt.Errorf("regular expression is invalid")
	}

	if valIfc, ok := val.(string); ok {
		return exp.MatchString(valIfc), nil
	}

	return false, fmt.Errorf("value wasn't string")
}

const FN_ReplaceRegex = "ReplaceRegex"

func func_ReplaceRegex(rtParams runtimeParams, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(2); !ok {
		return "", errNumParams(FN_ReplaceRegex, 1, got)
	}

	var rgx, replace string
	var foundReplace bool

	for i, p := range rtParams.paramsString {
		switch i {
		case 0:
			if p == "" {
				return "", fmt.Errorf("find parameter must not be an empty string")
			}
			rgx = p
		case 1:
			replace = p
			foundReplace = true
		}
		if i > 1 {
			break
		}
	}

	if !foundReplace {
		return "", fmt.Errorf("replace parameter missing")
	}
	exp, err := regexp.Compile(rgx)
	if err != nil {
		return "", fmt.Errorf("regular expression is invalid")
	}

	if valIfc, ok := val.(string); ok {
		return exp.ReplaceAllString(valIfc, replace), nil
	}

	return "", fmt.Errorf("value wasn't string")
}

const FN_ReplaceAll = "ReplaceAll"

func func_ReplaceAll(rtParams runtimeParams, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(2); !ok {
		return "", errNumParams(FN_ReplaceAll, 1, got)
	}

	var find, replace string
	var foundReplace bool

	for i, p := range rtParams.paramsString {
		switch i {
		case 0:
			if p == "" {
				return "", fmt.Errorf("find parameter must not be an empty string")
			}
			find = p
		case 1:
			replace = p
			foundReplace = true
		}
		if i > 1 {
			break
		}
	}

	if !foundReplace {
		return "", fmt.Errorf("replace parameter missing")
	}

	if valIfc, ok := val.(string); ok {
		return strings.ReplaceAll(valIfc, find, replace), nil
	}

	return "", fmt.Errorf("value wasn't string")
}

const FN_AsJSON = "AsJSON"

func func_AsJSON(rtParams runtimeParams, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return "", errNumParams(FN_AsJSON, 0, got)
	}

	v := reflect.ValueOf(val)
	if isEmptyValue(v) {
		return "", nil
	}

	outBytes, err := json.Marshal(val)
	if err != nil {
		return "", fmt.Errorf("unable to marshal to JSON: %w", err)
	}

	return string(outBytes), nil
}

func stringToObjectFunc(rtParams runtimeParams, val any, fn func(s string) (map[string]any, error), name string) (map[string]any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return nil, errNumParams(FN_ParseJSON, 0, got)
	}

	v := reflect.ValueOf(val)
	if isEmptyValue(v) {
		return nil, nil
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.String:
		if cdString, ok := val.(string); ok {
			return fn(cdString)
		}
	}

	return nil, fmt.Errorf("value is not a string")
}

const FN_ParseJSON = "ParseJSON"

func func_ParseJSON(rtParams runtimeParams, val any) (any, error) {
	return stringToObjectFunc(rtParams, val, func(s string) (map[string]any, error) {
		nm := map[string]any{}
		err := json.Unmarshal([]byte(s), &nm)
		if err != nil {
			return nil, fmt.Errorf("value is not JSON: %w", err)
		}

		return nm, nil
	}, FN_ParseJSON)
}

const FN_ParseXML = "ParseXML"

func func_ParseXML(rtParams runtimeParams, val any) (any, error) {
	return stringToObjectFunc(rtParams, val, func(s string) (map[string]any, error) {
		xml := strings.NewReader(s)
		jsn, err := xj.Convert(xml)
		if err != nil {
			return nil, fmt.Errorf("input wasn't XML")
		}

		jsnString := jsn.String()

		nm := map[string]any{}
		err = json.Unmarshal([]byte(jsnString), &nm)
		if err != nil {
			return nil, fmt.Errorf("value is not JSON: %w", err)
		}

		return nm, nil
	}, FN_ParseXML)
}

const FN_ParseYAML = "ParseYAML"

func func_ParseYAML(rtParams runtimeParams, val any) (any, error) {
	return stringToObjectFunc(rtParams, val, func(s string) (map[string]any, error) {
		nm := map[string]any{}
		err := yaml.Unmarshal([]byte(s), &nm)
		if err != nil {
			return nil, fmt.Errorf("value is not YAML: %w", err)
		}

		return nm, nil
	}, FN_ParseYAML)
}

const FN_ParseTOML = "ParseTOML"

func func_ParseTOML(rtParams runtimeParams, val any) (any, error) {
	return stringToObjectFunc(rtParams, val, func(s string) (map[string]any, error) {
		nm := map[string]any{}
		err := toml.Unmarshal([]byte(s), &nm)
		if err != nil {
			return nil, fmt.Errorf("value is not TOML: %w", err)
		}

		return nm, nil
	}, FN_ParseTOML)
}

const FN_RemoveKeysByRegex = "RemoveKeysByRegex"

func func_RemoveKeysByRegex(rtParams runtimeParams, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return nil, errNumParams(FN_RemoveKeysByRegex, 1, got)
	}

	param, err := paramsGetFirstOfString(rtParams)
	if err != nil {
		return nil, err
	}

	exp, err := regexp.Compile(param)
	if err != nil {
		return nil, fmt.Errorf("regular expression is invalid")
	}

	doForMapPerKey(val, func(keyAsString string, keyAsValue, mapAsValue reflect.Value) {
		if exp.MatchString(keyAsString) {
			// This deletes the key if it matches the regex
			mapAsValue.SetMapIndex(keyAsValue, reflect.Value{})
		}
	})

	return nil, fmt.Errorf("value is not a map")
}

const FN_RemoveKeysByPrefix = "RemoveKeysByPrefix"

func func_RemoveKeysByPrefix(rtParams runtimeParams, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return nil, errNumParams(FN_RemoveKeysByPrefix, 1, got)
	}

	prefixParam, err := paramsGetFirstOfString(rtParams)
	if err != nil {
		return nil, err
	}

	doForMapPerKey(val, func(keyAsString string, keyAsValue, mapAsValue reflect.Value) {
		if strings.HasPrefix(keyAsString, prefixParam) {
			// This deletes the key if it matches the regex
			mapAsValue.SetMapIndex(keyAsValue, reflect.Value{})
		}
	})

	return nil, fmt.Errorf("value is not a map")
}

const FN_RemoveKeysBySuffix = "RemoveKeysBySuffix"

func func_RemoveKeysBySuffix(rtParams runtimeParams, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return nil, errNumParams(FN_RemoveKeysBySuffix, 1, got)
	}

	prefixParam, err := paramsGetFirstOfString(rtParams)
	if err != nil {
		return nil, err
	}

	doForMapPerKey(val, func(keyAsString string, keyAsValue, mapAsValue reflect.Value) {
		if strings.HasSuffix(keyAsString, prefixParam) {
			// This deletes the key if it matches the regex
			mapAsValue.SetMapIndex(keyAsValue, reflect.Value{})
		}
	})

	return nil, fmt.Errorf("value is not a map")
}

type FT_FunctionType int

const (
	FT_NotSet FT_FunctionType = iota
	FT_Equal
	FT_NotEqual
	FT_Less
	FT_LessOrEqual
	FT_Greater
	FT_GreaterOrEqual
	FT_Contains
	FT_NotContains
	FT_Prefix
	FT_NotPrefix
	FT_Suffix
	FT_NotSuffix
	FT_Any
	FT_AnyOf

	FT_Count
	FT_First
	FT_Last
	FT_Index
	FT_Sum
	FT_Avg
	FT_Max
	FT_Min
	FT_Add
	FT_Sub
	FT_Div
	FT_Mul
	FT_Mod

	FT_TrimRightN
	FT_TrimLeftN
	FT_RightN
	FT_LeftN
	FT_DoesMatchRegex
	FT_ReplaceRegex
	FT_ReplaceAll

	FT_AsJSON
	FT_ParseJSON
	FT_ParseXML
	FT_ParseYAML
	FT_ParseTOML
	FT_RemoveKeysByRegex
	FT_RemoveKeysByPrefix
	FT_RemoveKeysBySuffix
)

func singleParam(name string, typ PT_ParameterType) []ParameterDescriptor {
	return []ParameterDescriptor{
		{
			Name: name,
			Type: typ,
		},
	}
}

var (
	funcMap = map[FT_FunctionType]FunctionDescriptor{
		FT_Equal: {
			Name:        "Equal",
			Description: "Checks whether the value equals the parameter",
			Params:      singleParam("value to match", PT_Any),
			ValidOn:     PT_Any,
			fn:          func_Equal,
		},
		FT_NotEqual: {
			Name:        "NotEqual",
			Description: "Checks whether the value does not equal the parameter",
			Params:      singleParam("value to match", PT_Any),
			ValidOn:     PT_Any,
			fn:          func_Equal,
		},
		FT_Less: {
			Name:        "Less",
			Description: "Checks whether the value is less than the parameter",
			Params:      singleParam("number to compare", PT_Number),
			ValidOn:     PT_Number,
			fn:          func_Less,
		},
		FT_LessOrEqual: {
			Name:        "LessOrEqual",
			Description: "Checks whether the value is less than or equal to the parameter",
			Params:      singleParam("number to compare", PT_Number),
			ValidOn:     PT_Number,
			fn:          func_LessOrEqual,
		},
		FT_Greater: {
			Name:        "Greater",
			Description: "Checks whether the value is greater than the parameter",
			Params:      singleParam("number to compare", PT_Number),
			ValidOn:     PT_Number,
			fn:          func_Greater,
		},
		FT_GreaterOrEqual: {
			Name:        "GreaterOrEqual",
			Description: "Checks whether the value is greater than or equal to the parameter",
			Params:      singleParam("number to compare", PT_Number),
			ValidOn:     PT_Number,
			fn:          func_GreaterOrEqual,
		},
		FT_Contains: {
			Name:        "Contains",
			Description: "Checks whether the value contains the parameter",
			Params:      singleParam("string to match", PT_String),
			ValidOn:     PT_String,
			fn:          func_Contains,
		},
		FT_NotContains: {
			Name:        "NotContains",
			Description: "Checks whether the value does not contain the parameter",
			Params:      singleParam("string to match", PT_String),
			ValidOn:     PT_String,
			fn:          func_NotContains,
		},
		FT_Prefix: {
			Name:        "Prefix",
			Description: "Checks whether the value has the parameter as a prefix",
			Params:      singleParam("prefix to match", PT_String),
			ValidOn:     PT_String,
			fn:          func_Prefix,
		},
		FT_NotPrefix: {
			Name:        "NotPrefix",
			Description: "Checks whether the value does not have the parameter as a prefix",
			Params:      singleParam("prefix to match", PT_String),
			ValidOn:     PT_String,
			fn:          func_NotPrefix,
		},
		FT_Suffix: {
			Name:        "Suffix",
			Description: "Checks whether the value has the parameter as a suffix",
			Params:      singleParam("suffix to match", PT_String),
			ValidOn:     PT_String,
			fn:          func_Suffix,
		},
		FT_NotSuffix: {
			Name:        "NotSuffix",
			Description: "Checks whether the value does not have the parameter as a suffix",
			Params:      singleParam("suffix to match", PT_String),
			ValidOn:     PT_String,
			fn:          func_NotSuffix,
		},
		FT_Count: {
			Name:        "Count",
			Description: "Returns the count of elements in the array",
			Params:      nil,
			ValidOn:     PT_Array,
			fn:          func_Count,
		},
		FT_First: {
			Name:        "First",
			Description: "Returns the first element of the array",
			Params:      nil,
			ValidOn:     PT_Array,
			fn:          func_First,
		},
		FT_Last: {
			Name:        "Last",
			Description: "Returns the last element of the array",
			Params:      nil,
			ValidOn:     PT_Array,
			fn:          func_Last,
		},
		FT_Index: {
			Name:        "Index",
			Description: "Returns the element at the zero based index of the array",
			Params:      singleParam("index", PT_Number),
			ValidOn:     PT_Array,
			fn:          func_Index,
		},
		FT_Any: {
			Name:        "Any",
			Description: "Checks whether there are any elements in the array",
			Params:      nil,
			ValidOn:     PT_Array,
			fn:          func_Any,
		},
		FT_Sum: {
			Name:        "Sum",
			Description: "Sums the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers (not required)", PT_ArrayOfNumbers),
			ValidOn:     PT_NumberOrArrayOfNumbers,
			fn:          func_Sum,
		},
		FT_Avg: {
			Name:        "Avg",
			Description: "Averages the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers (not required)", PT_ArrayOfNumbers),
			ValidOn:     PT_NumberOrArrayOfNumbers,
			fn:          func_Avg,
		},
		FT_Max: {
			Name:        "Max",
			Description: "Returns the maximum of the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers (not required)", PT_ArrayOfNumbers),
			ValidOn:     PT_NumberOrArrayOfNumbers,
			fn:          func_Max,
		},
		FT_Min: {
			Name:        "Min",
			Description: "Returns the minimum of the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers (not required)", PT_ArrayOfNumbers),
			ValidOn:     PT_NumberOrArrayOfNumbers,
			fn:          func_Min,
		},
		FT_Add: {
			Name:        "Add",
			Description: "Adds the parameter to the value",
			Params:      singleParam("number to add", PT_Number),
			ValidOn:     PT_Number,
			fn:          func_Add,
		},
		FT_Sub: {
			Name:        "Sub",
			Description: "Subtracts the parameter from the value",
			Params:      singleParam("number to subtract", PT_Number),
			ValidOn:     PT_Number,
			fn:          func_Sub,
		},
		FT_Div: {
			Name:        "Div",
			Description: "Divides the value by the parameter",
			Params:      singleParam("number to divide by", PT_Number),
			ValidOn:     PT_Number,
			fn:          func_Div,
		},
		FT_Mul: {
			Name:        "Mul",
			Description: "Multiplies the value by the parameter",
			Params:      singleParam("number to multiply by", PT_Number),
			ValidOn:     PT_Number,
			fn:          func_Mul,
		},
		FT_Mod: {
			Name:        "Mod",
			Description: "Returns the remainder of the value after dividing the value by the parameter",
			Params:      singleParam("number to modulo by", PT_Number),
			ValidOn:     PT_Number,
			fn:          func_Mod,
		},
		FT_AnyOf: {
			Name:        "AnyOf",
			Description: "Checks whether the value matches any of the parameters",
			Params:      singleParam("the values to match against", PT_Array),
			ValidOn:     PT_Any,
			fn:          func_AnyOf,
		},
		FT_TrimRightN: {
			Name:        "TrimRightN",
			Description: "Removes the 'n' most characters of the value from the right, where 'n' is the parameter",
			Params:      singleParam("number of characters", PT_Number),
			ValidOn:     PT_String,
			fn:          func_TrimRightN,
		},
		FT_TrimLeftN: {
			Name:        "TrimLeftN",
			Description: "Removes the 'n' most characters of the value from the left, where 'n' is the parameter",
			Params:      singleParam("number of characters", PT_Number),
			ValidOn:     PT_String,
			fn:          func_TrimLeftN,
		},
		FT_RightN: {
			Name:        "RightN",
			Description: "Returns the 'n' most characters of the value from the right, where 'n' is the parameter'",
			Params:      singleParam("number of characters", PT_Number),
			ValidOn:     PT_String,
			fn:          func_RightN,
		},
		FT_LeftN: {
			Name:        "LeftN",
			Description: "Returns the 'n' most characters of the value from the left, where 'n' is the parameter",
			Params:      singleParam("number of characters", PT_Number),
			ValidOn:     PT_String,
			fn:          func_LeftN,
		},
		FT_DoesMatchRegex: {
			Name:        "DoesMatchRegex",
			Description: "Checks whether the value matches the regular expression in the parameter",
			Params:      singleParam("regular expression to match", PT_String),
			ValidOn:     PT_String,
			fn:          func_DoesMatchRegex,
		},
		FT_ReplaceRegex: {
			Name:        "ReplaceRegex",
			Description: "Replaces any matches of the regular expression parameter in the value with the replacement parameter",
			Params: []ParameterDescriptor{
				{
					Name: "regular expression",
					Type: PT_String,
				},
				{
					Name: "replacement",
					Type: PT_String,
				},
			},
			ValidOn: PT_String,
			fn:      func_ReplaceRegex,
		},
		FT_ReplaceAll: {
			Name:        "ReplaceAll",
			Description: "Replaces any matches of the string to match parameter in the value with the replacement parameter",
			Params: []ParameterDescriptor{
				{
					Name: "string to match",
					Type: PT_String,
				},
				{
					Name: "replacement",
					Type: PT_String,
				},
			},
			ValidOn: PT_String,
			fn:      func_ReplaceAll,
		},
		FT_AsJSON: {
			Name:        "AsJSON",
			Description: "Returns the value represented as JSON",
			Params:      nil,
			ValidOn:     PT_Any,
			fn:          func_AsJSON,
		},
		FT_ParseJSON: {
			Name:        "ParseJSON",
			Description: "Parses the value as JSON and returns an object or array",
			Params:      nil,
			ValidOn:     PT_String,
			fn:          func_ParseJSON,
		},
		FT_ParseXML: {
			Name:        "ParseXML",
			Description: "Parses the value as XML and returns an object or array",
			Params:      nil,
			ValidOn:     PT_String,
			fn:          func_ParseXML,
		},
		FT_ParseYAML: {
			Name:        "ParseYAML",
			Description: "Parses the value as YAML and returns an object or array",
			Params:      nil,
			ValidOn:     PT_String,
			fn:          func_ParseYAML,
		},
		FT_ParseTOML: {
			Name:        "ParseTOML",
			Description: "Parses the value as TOML and returns an object or array",
			Params:      nil,
			ValidOn:     PT_String,
			fn:          func_ParseTOML,
		},
		FT_RemoveKeysByRegex: {
			Name:        "RemoveKeysByRegex",
			Description: "Removes any keys that match the regular expression in the parameter",
			Params:      singleParam("regular expression to match", PT_String),
			ValidOn:     PT_MapStringOfAny,
			fn:          func_RemoveKeysByRegex,
		},
		FT_RemoveKeysByPrefix: {
			Name:        "RemoveKeysByPrefix",
			Description: "Removes any keys that have a prefix as defined by the parameter",
			Params:      singleParam("prefix to match", PT_String),
			ValidOn:     PT_MapStringOfAny,
			fn:          func_RemoveKeysByPrefix,
		},
		FT_RemoveKeysBySuffix: {
			Name:        "RemoveKeysBySuffix",
			Description: "Removes any keys that have a suffix as defined by the parameter",
			Params:      singleParam("suffix to match", PT_String),
			ValidOn:     PT_MapStringOfAny,
			fn:          func_RemoveKeysBySuffix,
		},
	}
)

func ft_GetByName(name string) (ft FT_FunctionType, err error) {
	switch name {
	case "Equal":
		ft = FT_Equal
	case "NotEqual":
		ft = FT_NotEqual
	case "Less":
		ft = FT_Less
	case "LessOrEqual":
		ft = FT_LessOrEqual
	case "Greater":
		ft = FT_Greater
	case "GreaterOrEqual":
		ft = FT_GreaterOrEqual
	case "Contains":
		ft = FT_Contains
	case "NotContains":
		ft = FT_NotContains
	case "Prefix":
		ft = FT_Prefix
	case "NotPrefix":
		ft = FT_NotPrefix
	case "Suffix":
		ft = FT_Suffix
	case "NotSuffix":
		ft = FT_NotSuffix
	case "Count":
		ft = FT_Count
	case "First":
		ft = FT_First
	case "Last":
		ft = FT_Last
	case "Index":
		ft = FT_Index
	case "Any":
		ft = FT_Any
	case "Sum":
		ft = FT_Sum
	case "Avg":
		ft = FT_Avg
	case "Max":
		ft = FT_Max
	case "Min":
		ft = FT_Min
	case "Add":
		ft = FT_Add
	case "Sub":
		ft = FT_Sub
	case "Div":
		ft = FT_Div
	case "Mul":
		ft = FT_Mul
	case "Mod":
		ft = FT_Mod
	case "AnyOf":
		ft = FT_AnyOf

	case "TrimRightN":
		ft = FT_TrimRightN
	case "TrimLeftN":
		ft = FT_TrimLeftN
	case "RightN":
		ft = FT_RightN
	case "LeftN":
		ft = FT_LeftN
	case "DoesMatchRegex":
		ft = FT_DoesMatchRegex
	case "ReplaceRegex":
		ft = FT_ReplaceRegex
	case "ReplaceAll":
		ft = FT_ReplaceAll

	case "AsJSON":
		ft = FT_AsJSON
	case "ParseJSON":
		ft = FT_ParseJSON
	case "ParseXML":
		ft = FT_ParseXML
	case "ParseYAML":
		ft = FT_ParseYAML
	case "ParseTOML":
		ft = FT_ParseTOML

	case "RemoveKeysByRegex":
		ft = FT_RemoveKeysByRegex
	case "RemoveKeysByPrefix":
		ft = FT_RemoveKeysByPrefix
	case "RemoveKeysBySuffix":
		ft = FT_RemoveKeysBySuffix

	default:
		return 0, fmt.Errorf("unknown function name '%s'", name)
	}

	return
}

func ft_GetName(ft FT_FunctionType) (name string) {
	fm, ok := funcMap[ft]
	if !ok {
		return "unknown function"
	}

	return fm.Name
}

func ft_ShouldContinueForPath(ft FT_FunctionType) bool {
	switch ft {
	case FT_First, FT_Last, FT_Index:
		return true
	}

	return false
}

func ft_IsBoolFunc(ft FT_FunctionType) bool {
	switch ft {
	case FT_Equal,
		FT_NotEqual,
		FT_Less,
		FT_LessOrEqual,
		FT_Greater,
		FT_GreaterOrEqual,
		FT_Contains,
		FT_NotContains,
		FT_Prefix,
		FT_NotPrefix,
		FT_Suffix,
		FT_NotSuffix,
		FT_AnyOf,
		FT_Any:
		return true
	}

	return false
}
