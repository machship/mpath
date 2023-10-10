package mpath

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"

	xj "github.com/basgys/goxml2json"
	"github.com/pelletier/go-toml/v2"
	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v2"
)

func paramsGetFirstOfAny(rtParams FunctionParameters) (val any, err error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return nil, fmt.Errorf("expected %d params, got %d", 1, got)
	}

	for _, p := range rtParams {
		return p.GetValue(), nil
	}

	return nil, fmt.Errorf("no parameters found")
}

func paramsGetFirstOfNumber(rtParams FunctionParameters) (val decimal.Decimal, err error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return val, fmt.Errorf("expected %d params, got %d", 1, got)
	}

	for _, p := range rtParams.Numbers() {
		return p.Value, nil
	}

	for _, p := range rtParams.Strings() {
		if wasNumber, number := convertToDecimalIfNumberAndCheck(p.Value); wasNumber {
			return number, nil
		}
	}

	return val, fmt.Errorf("no number parameter found")
}

func paramsGetFirstOfString(rtParams FunctionParameters) (val string, err error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return val, fmt.Errorf("expected %d params, got %d", 1, got)
	}

	for _, p := range rtParams.Strings() {
		return p.Value, nil
	}

	return val, fmt.Errorf("no string parameter found")
}

func paramsGetAll(rtParams FunctionParameters) (val []any, err error) {
	for _, p := range rtParams.Numbers() {
		val = append(val, p.Value)
	}

	for _, p := range rtParams.Strings() {
		val = append(val, p.Value)
	}

	for _, p := range rtParams.Bools() {
		val = append(val, p.Value)
	}

	return
}

func (rtParams *FunctionParameters) checkLengthOfParams(allowed int) (got int, ok bool) {
	for _, p := range *rtParams {
		switch p.(type) {
		case *FP_Path:
			continue
		}
		got++
	}

	if allowed == -1 {
		return got, true
	}

	return got, allowed == got
}

func errBool(name FT_FunctionType, err error) (bool, error) {
	return false, fmt.Errorf("func %s: %w", name, err)
}

func errString(name FT_FunctionType, err error) (string, error) {
	return "", fmt.Errorf("func %s: %w", name, err)
}

func errNumParams(name FT_FunctionType, expected, got int) error {
	return fmt.Errorf("(%s) expected %d params, got %d", name, expected, got)
}

const FT_Equal FT_FunctionType = "Equal"

func func_Equal(rtParams FunctionParameters, val any) (any, error) {
	param, err := paramsGetFirstOfAny(rtParams)
	if err != nil {
		return errBool(FT_Equal, err)
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

const FT_NotEqual FT_FunctionType = "NotEqual"

func func_NotEqual(rtParams FunctionParameters, val any) (any, error) {
	param, err := paramsGetFirstOfAny(rtParams)
	if err != nil {
		return errBool(FT_NotEqual, err)
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

func decimalBoolFunc(rtParams FunctionParameters, val any, fn func(d1, d2 decimal.Decimal) bool, fnName FT_FunctionType) (bool, error) {
	param, err := paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return errBool(fnName, err)
	}

	if valIfc, ok := val.(decimal.Decimal); ok {
		return fn(valIfc, param), nil
	}

	return false, fmt.Errorf("parameter wasn't number")
}

const FT_Less FT_FunctionType = "Less"

func func_Less(rtParams FunctionParameters, val any) (any, error) {
	return decimalBoolFunc(rtParams, val, func(d1, d2 decimal.Decimal) bool {
		return d1.LessThan(d2)
	}, FT_Less)
}

const FT_LessOrEqual FT_FunctionType = "LessOrEqual"

func func_LessOrEqual(rtParams FunctionParameters, val any) (any, error) {
	return decimalBoolFunc(rtParams, val, func(d1, d2 decimal.Decimal) bool {
		return d1.LessThanOrEqual(d2)
	}, FT_LessOrEqual)
}

const FT_Greater FT_FunctionType = "Greater"

func func_Greater(rtParams FunctionParameters, val any) (any, error) {
	return decimalBoolFunc(rtParams, val, func(d1, d2 decimal.Decimal) bool {
		return d1.GreaterThan(d2)
	}, FT_Greater)
}

const FT_GreaterOrEqual FT_FunctionType = "GreaterOrEqual"

func func_GreaterOrEqual(rtParams FunctionParameters, val any) (any, error) {
	return decimalBoolFunc(rtParams, val, func(d1, d2 decimal.Decimal) bool {
		return d1.GreaterThanOrEqual(d2)
	}, FT_GreaterOrEqual)
}

func stringBoolFunc(rtParams FunctionParameters, val any, fn func(string, string) bool, invert bool, fnName FT_FunctionType) (bool, error) {
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

const FT_Contains FT_FunctionType = "Contains"

func func_Contains(rtParams FunctionParameters, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.Contains, false, FT_Contains)
}

const FT_NotContains FT_FunctionType = "NotContains"

func func_NotContains(rtParams FunctionParameters, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.Contains, true, FT_NotContains)
}

const FT_Prefix FT_FunctionType = "Prefix"

func func_Prefix(rtParams FunctionParameters, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.HasPrefix, false, FT_Prefix)
}

const FT_NotPrefix FT_FunctionType = "NotPrefix"

func func_NotPrefix(rtParams FunctionParameters, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.HasPrefix, true, FT_NotPrefix)
}

const FT_Suffix FT_FunctionType = "Suffix"

func func_Suffix(rtParams FunctionParameters, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.HasSuffix, false, FT_Suffix)
}

const FT_NotSuffix FT_FunctionType = "NotSuffix"

func func_NotSuffix(rtParams FunctionParameters, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.HasSuffix, true, FT_NotSuffix)
}

const FT_Count FT_FunctionType = "Count"

func func_Count(rtParams FunctionParameters, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return decimal.Zero, fmt.Errorf("(%s) expected %d params, got %d", FT_Count, 0, got)
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

const FT_Any FT_FunctionType = "Any"

func func_Any(rtParams FunctionParameters, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return false, errNumParams(FT_Any, 0, got)
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

const FT_First FT_FunctionType = "First"

func func_First(rtParams FunctionParameters, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return false, errNumParams(FT_First, 0, got)
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

const FT_Last FT_FunctionType = "Last"

func func_Last(rtParams FunctionParameters, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return false, errNumParams(FT_Last, 0, got)
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

const FT_Index FT_FunctionType = "Index"

func func_Index(rtParams FunctionParameters, val any) (any, error) {
	param, err := paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return errBool(FT_Index, err)
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

func func_decimalSlice(rtParams FunctionParameters, val any, decimalSliceFunction func(decimal.Decimal, ...decimal.Decimal) decimal.Decimal) (any, error) {
	if vd, ok := val.(decimal.Decimal); ok {
		val = []decimal.Decimal{vd}
	}

	var paramNumbers []decimal.Decimal
	for _, pn := range rtParams.Numbers() {
		paramNumbers = append(paramNumbers, pn.Value)
	}

	for _, ps := range rtParams.Strings() {
		if wasNumber, number := convertToDecimalIfNumberAndCheck(ps.Value); wasNumber {
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

const FT_Sum FT_FunctionType = "Sum"

func func_Sum(rtParams FunctionParameters, val any) (any, error) {
	return func_decimalSlice(rtParams, val, decimal.Sum)
}

const FT_Average FT_FunctionType = "Average"

func func_Average(rtParams FunctionParameters, val any) (any, error) {
	return func_decimalSlice(rtParams, val, decimal.Avg)
}

const FT_Minimum FT_FunctionType = "Minimum"

func func_Minimum(rtParams FunctionParameters, val any) (any, error) {
	return func_decimalSlice(rtParams, val, decimal.Min)
}

const FT_Maximum FT_FunctionType = "Maximum"

func func_Maximum(rtParams FunctionParameters, val any) (any, error) {
	return func_decimalSlice(rtParams, val, decimal.Max)
}

func func_decimal(rtParams FunctionParameters, val any, decSlcFunc func(decimal.Decimal, decimal.Decimal) decimal.Decimal, name FT_FunctionType) (any, error) {
	param, err := paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return errBool(name, err)
	}

	if valIfc, ok := val.(decimal.Decimal); ok {
		return decSlcFunc(valIfc, param), nil
	}

	return false, fmt.Errorf("not a number")
}

const FT_Add FT_FunctionType = "Add"

func func_Add(rtParams FunctionParameters, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Add, FT_Add)
}

const FT_Subtract FT_FunctionType = "Subtract"

func func_Subtract(rtParams FunctionParameters, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Sub, FT_Subtract)
}

const FT_Divide FT_FunctionType = "Divide"

func func_Divide(rtParams FunctionParameters, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Div, FT_Divide)
}

const FT_Multiply FT_FunctionType = "Multiply"

func func_Multiply(rtParams FunctionParameters, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Mul, FT_Multiply)
}

const FT_Modulo FT_FunctionType = "Modulo"

func func_Modulo(rtParams FunctionParameters, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Mod, FT_Modulo)
}

const FT_AnyOf FT_FunctionType = "AnyOf"

func func_AnyOf(rtParams FunctionParameters, val any) (any, error) {
	params, err := paramsGetAll(rtParams)
	if err != nil {
		return errBool(FT_AnyOf, err)
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

func stringPartFunc(rtParams FunctionParameters, val any, fn func(string, int) (string, error), fnName FT_FunctionType) (string, error) {
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

const FT_TrimRight FT_FunctionType = "TrimRight"

func func_TrimRight(rtParams FunctionParameters, val any) (any, error) {
	return stringPartFunc(rtParams, val, func(s string, i int) (string, error) {
		if len(s) <= i {
			return "", nil
		}

		return s[:len(s)-i], nil
	}, FT_TrimRight)
}

const FT_TrimLeft FT_FunctionType = "TrimLeft"

func func_TrimLeft(rtParams FunctionParameters, val any) (any, error) {
	return stringPartFunc(rtParams, val, func(s string, i int) (string, error) {
		if len(s) <= i {
			return "", nil
		}

		return s[i:], nil
	}, FT_TrimLeft)
}

const FT_Right FT_FunctionType = "Right"

func func_Right(rtParams FunctionParameters, val any) (any, error) {
	return stringPartFunc(rtParams, val, func(s string, i int) (string, error) {
		if len(s) < i {
			return s, nil
		}

		return s[len(s)-i:], nil
	}, FT_Right)
}

const FT_Left FT_FunctionType = "Left"

func func_Left(rtParams FunctionParameters, val any) (any, error) {
	return stringPartFunc(rtParams, val, func(s string, i int) (string, error) {
		if len(s) < i {
			return s, nil
		}

		return s[:i], nil
	}, FT_Left)
}

const FT_DoesMatchRegex FT_FunctionType = "DoesMatchRegex"

func func_DoesMatchRegex(rtParams FunctionParameters, val any) (any, error) {
	param, err := paramsGetFirstOfString(rtParams)
	if err != nil {
		return errBool(FT_DoesMatchRegex, err)
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

const FT_ReplaceRegex FT_FunctionType = "ReplaceRegex"

func func_ReplaceRegex(rtParams FunctionParameters, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(2); !ok {
		return "", errNumParams(FT_ReplaceRegex, 1, got)
	}

	var rgx, replace string
	var foundReplace bool

	for i, ps := range rtParams.Strings() {
		p := ps.Value
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

const FT_ReplaceAll FT_FunctionType = "ReplaceAll"

func func_ReplaceAll(rtParams FunctionParameters, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(2); !ok {
		return "", errNumParams(FT_ReplaceAll, 1, got)
	}

	var find, replace string
	var foundReplace bool

	for i, ps := range rtParams.Strings() {
		p := ps.Value
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

const FT_AsJSON FT_FunctionType = "AsJSON"

func func_AsJSON(rtParams FunctionParameters, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return "", errNumParams(FT_AsJSON, 0, got)
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

func stringToObjectFunc(rtParams FunctionParameters, val any, fn func(s string) (map[string]any, error), name FT_FunctionType) (map[string]any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return nil, errNumParams(FT_ParseJSON, 0, got)
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

const FT_ParseJSON FT_FunctionType = "ParseJSON"

func func_ParseJSON(rtParams FunctionParameters, val any) (any, error) {
	return stringToObjectFunc(rtParams, val, func(s string) (map[string]any, error) {
		nm := map[string]any{}
		err := json.Unmarshal([]byte(s), &nm)
		if err != nil {
			return nil, fmt.Errorf("value is not JSON: %w", err)
		}

		return nm, nil
	}, FT_ParseJSON)
}

const FT_ParseXML FT_FunctionType = "ParseXML"

func func_ParseXML(rtParams FunctionParameters, val any) (any, error) {
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
	}, FT_ParseXML)
}

const FT_ParseYAML FT_FunctionType = "ParseYAML"

func func_ParseYAML(rtParams FunctionParameters, val any) (any, error) {
	return stringToObjectFunc(rtParams, val, func(s string) (map[string]any, error) {
		nm := map[string]any{}
		err := yaml.Unmarshal([]byte(s), &nm)
		if err != nil {
			return nil, fmt.Errorf("value is not YAML: %w", err)
		}

		return nm, nil
	}, FT_ParseYAML)
}

const FT_ParseTOML FT_FunctionType = "ParseTOML"

func func_ParseTOML(rtParams FunctionParameters, val any) (any, error) {
	return stringToObjectFunc(rtParams, val, func(s string) (map[string]any, error) {
		nm := map[string]any{}
		err := toml.Unmarshal([]byte(s), &nm)
		if err != nil {
			return nil, fmt.Errorf("value is not TOML: %w", err)
		}

		return nm, nil
	}, FT_ParseTOML)
}

const FT_RemoveKeysByRegex FT_FunctionType = "RemoveKeysByRegex"

func func_RemoveKeysByRegex(rtParams FunctionParameters, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return nil, errNumParams(FT_RemoveKeysByRegex, 1, got)
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

const FT_RemoveKeysByPrefix FT_FunctionType = "RemoveKeysByPrefix"

func func_RemoveKeysByPrefix(rtParams FunctionParameters, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return nil, errNumParams(FT_RemoveKeysByPrefix, 1, got)
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

const FT_RemoveKeysBySuffix FT_FunctionType = "RemoveKeysBySuffix"

func func_RemoveKeysBySuffix(rtParams FunctionParameters, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return nil, errNumParams(FT_RemoveKeysBySuffix, 1, got)
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

type FT_FunctionType string

func singleParam(name string, typ PT_ParameterType) []ParameterDescriptor {
	return []ParameterDescriptor{
		{
			Name: name,
			Type: typ,
		},
	}
}

var functionTypeByName = map[string]FT_FunctionType{}

func init() {
	for k, v := range funcMap {
		functionTypeByName[string(v.Name)] = k
	}
}

const FT_NotSet FT_FunctionType = "NotSet"

func ft_GetByName(s string) (FT_FunctionType, error) {
	ft, ok := functionTypeByName[s]
	if !ok {
		return FT_NotSet, fmt.Errorf("function '%s' is not a recognised function", s)
	}

	return ft, nil
}

func ft_GetName(x FT_FunctionType) string {
	return string(x)
}

type PT_ParameterType string

const (
	PT_String                 PT_ParameterType = "String"
	PT_Boolean                PT_ParameterType = "Boolean"
	PT_Number                 PT_ParameterType = "Number"
	PT_Array                  PT_ParameterType = "Array"
	PT_ArrayOfNumbers         PT_ParameterType = "ArrayOfNumbers"
	PT_NumberOrArrayOfNumbers PT_ParameterType = "NumberOrArrayOfNumbers"
	// PT_SameAsInput            PT_ParameterType = "SameAsInput"
	PT_Any    PT_ParameterType = "Any"
	PT_Object PT_ParameterType = "Object"
	PT_Root   PT_ParameterType = "Root"
)

func (pt PT_ParameterType) IsPrimitive() bool {
	switch pt {
	case PT_String, PT_Boolean, PT_Number:
		return true
	}

	return false
}

type FunctionDescriptor struct {
	Name        FT_FunctionType       `json:"name"`
	Description string                `json:"description"`
	Params      []ParameterDescriptor `json:"params"`
	ValidOn     PT_ParameterType      `json:"validOn"`
	Returns     PT_ParameterType      `json:"returns"`
	fn          func(rtParams FunctionParameters, val any) (any, error)
}

func (fd FunctionDescriptor) GetParamAtPosition(position int) (pd ParameterDescriptor, err error) {
	if (len(fd.Params) - 1) < position {
		return pd, fmt.Errorf("no parameter at position %d", position)
	}

	return fd.Params[position], nil
}

func (fd FunctionDescriptor) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name         FT_FunctionType       `json:"name"`
		FriendlyName string                `json:"friendlyName"`
		Description  string                `json:"description"`
		Params       []ParameterDescriptor `json:"params"`
		ValidOn      PT_ParameterType      `json:"validOn"`
		Returns      PT_ParameterType      `json:"returns"`
	}{
		Name:         fd.Name,
		FriendlyName: fd.FriendlyName(),
		Description:  fd.Description,
		Params:       fd.Params,
		ValidOn:      fd.ValidOn,
		Returns:      fd.Returns,
	})
}

func (fd FunctionDescriptor) FriendlyName() (friendlyName string) {
	// The rules here are that a name gets split by a space
	// *after* a lowercase character that is followed by an uppercase character

	idxLastChar := len(fd.Name) - 1
	for i, c := range fd.Name {
		friendlyName += string(c)

		if i == idxLastChar {
			friendlyName = strings.ReplaceAll(friendlyName, " Or ", " or ")
			friendlyName = strings.ReplaceAll(friendlyName, " And ", " and ")
			friendlyName = strings.ReplaceAll(friendlyName, " By ", " by ")

			return
		}

		if unicode.IsLower(c) && unicode.IsUpper(rune(fd.Name[i+1])) {
			friendlyName += " "
		}
	}

	return
}

func getAvailableFunctionsForKind(pt PT_ParameterType, exludeAny bool) (names []string) {
	for _, fd := range funcMap {
		if pt == fd.ValidOn {
			names = append(names, string(fd.Name))
		}
		if !exludeAny && pt != PT_Any {
			names = append(names, string(fd.Name))
		}
	}

	sort.Strings(names)

	return
}

type ParameterDescriptor struct {
	Name string           `json:"name"`
	Type PT_ParameterType `json:"type"`
}

var (
	funcMap = map[FT_FunctionType]FunctionDescriptor{
		FT_Equal: {
			Name:        FT_Equal,
			Description: "Checks whether the value equals the parameter",
			Params:      singleParam("value to match", PT_Any),
			Returns:     PT_Boolean,
			ValidOn:     PT_Any,
			fn:          func_Equal,
		},
		FT_NotEqual: {
			Name:        FT_NotEqual,
			Description: "Checks whether the value does not equal the parameter",
			Params:      singleParam("value to match", PT_Any),
			Returns:     PT_Boolean,
			ValidOn:     PT_Any,
			fn:          func_NotEqual,
		},
		FT_Less: {
			Name:        FT_Less,
			Description: "Checks whether the value is less than the parameter",
			Params:      singleParam("number to compare", PT_Number),
			Returns:     PT_Boolean,
			ValidOn:     PT_Number,
			fn:          func_Less,
		},
		FT_LessOrEqual: {
			Name:        FT_LessOrEqual,
			Description: "Checks whether the value is less than or equal to the parameter",
			Params:      singleParam("number to compare", PT_Number),
			Returns:     PT_Boolean,
			ValidOn:     PT_Number,
			fn:          func_LessOrEqual,
		},
		FT_Greater: {
			Name:        FT_Greater,
			Description: "Checks whether the value is greater than the parameter",
			Params:      singleParam("number to compare", PT_Number),
			Returns:     PT_Boolean,
			ValidOn:     PT_Number,
			fn:          func_Greater,
		},
		FT_GreaterOrEqual: {
			Name:        FT_GreaterOrEqual,
			Description: "Checks whether the value is greater than or equal to the parameter",
			Params:      singleParam("number to compare", PT_Number),
			Returns:     PT_Boolean,
			ValidOn:     PT_Number,
			fn:          func_GreaterOrEqual,
		},
		FT_Contains: {
			Name:        FT_Contains,
			Description: "Checks whether the value contains the parameter",
			Params:      singleParam("string to match", PT_String),
			Returns:     PT_Boolean,
			ValidOn:     PT_String,
			fn:          func_Contains,
		},
		FT_NotContains: {
			Name:        FT_NotContains,
			Description: "Checks whether the value does not contain the parameter",
			Params:      singleParam("string to match", PT_String),
			Returns:     PT_Boolean,
			ValidOn:     PT_String,
			fn:          func_NotContains,
		},
		FT_Prefix: {
			Name:        FT_Prefix,
			Description: "Checks whether the value has the parameter as a prefix",
			Params:      singleParam("prefix to match", PT_String),
			Returns:     PT_Boolean,
			ValidOn:     PT_String,
			fn:          func_Prefix,
		},
		FT_NotPrefix: {
			Name:        FT_NotPrefix,
			Description: "Checks whether the value does not have the parameter as a prefix",
			Params:      singleParam("prefix to match", PT_String),
			Returns:     PT_Boolean,
			ValidOn:     PT_String,
			fn:          func_NotPrefix,
		},
		FT_Suffix: {
			Name:        FT_Suffix,
			Description: "Checks whether the value has the parameter as a suffix",
			Params:      singleParam("suffix to match", PT_String),
			Returns:     PT_Boolean,
			ValidOn:     PT_String,
			fn:          func_Suffix,
		},
		FT_NotSuffix: {
			Name:        FT_NotSuffix,
			Description: "Checks whether the value does not have the parameter as a suffix",
			Params:      singleParam("suffix to match", PT_String),
			Returns:     PT_Boolean,
			ValidOn:     PT_String,
			fn:          func_NotSuffix,
		},
		FT_Count: {
			Name:        FT_Count,
			Description: "Returns the count of elements in the array",
			Params:      nil,
			Returns:     PT_Number,
			ValidOn:     PT_Array,
			fn:          func_Count,
		},
		FT_First: {
			Name:        FT_First,
			Description: "Returns the first element of the array",
			Params:      nil,
			Returns:     PT_Any,
			ValidOn:     PT_Array,
			fn:          func_First,
		},
		FT_Last: {
			Name:        FT_Last,
			Description: "Returns the last element of the array",
			Params:      nil,
			Returns:     PT_Any,
			ValidOn:     PT_Array,
			fn:          func_Last,
		},
		FT_Index: {
			Name:        FT_Index,
			Description: "Returns the element at the zero based index of the array",
			Params:      singleParam("index", PT_Number),
			Returns:     PT_Any,
			ValidOn:     PT_Array,
			fn:          func_Index,
		},
		FT_Any: {
			Name:        FT_Any,
			Description: "Checks whether there are any elements in the array",
			Params:      nil,
			Returns:     PT_Boolean,
			ValidOn:     PT_Array,
			fn:          func_Any,
		},
		FT_Sum: {
			Name:        FT_Sum,
			Description: "Sums the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers (not required)", PT_ArrayOfNumbers),
			Returns:     PT_Number,
			ValidOn:     PT_NumberOrArrayOfNumbers,
			fn:          func_Sum,
		},
		FT_Average: {
			Name:        FT_Average,
			Description: "Averages the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers (not required)", PT_ArrayOfNumbers),
			Returns:     PT_Number,
			ValidOn:     PT_NumberOrArrayOfNumbers,
			fn:          func_Average,
		},
		FT_Maximum: {
			Name:        FT_Maximum,
			Description: "Returns the maximum of the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers (not required)", PT_ArrayOfNumbers),
			Returns:     PT_Number,
			ValidOn:     PT_NumberOrArrayOfNumbers,
			fn:          func_Maximum,
		},
		FT_Minimum: {
			Name:        FT_Minimum,
			Description: "Returns the minimum of the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers (not required)", PT_ArrayOfNumbers),
			Returns:     PT_Number,
			ValidOn:     PT_NumberOrArrayOfNumbers,
			fn:          func_Minimum,
		},
		FT_Add: {
			Name:        FT_Add,
			Description: "Adds the parameter to the value",
			Params:      singleParam("number to add", PT_Number),
			Returns:     PT_Number,
			ValidOn:     PT_Number,
			fn:          func_Add,
		},
		FT_Subtract: {
			Name:        FT_Subtract,
			Description: "Subtracts the parameter from the value",
			Params:      singleParam("number to subtract", PT_Number),
			Returns:     PT_Number,
			ValidOn:     PT_Number,
			fn:          func_Subtract,
		},
		FT_Divide: {
			Name:        FT_Divide,
			Description: "Divides the value by the parameter",
			Params:      singleParam("number to divide by", PT_Number),
			Returns:     PT_Number,
			ValidOn:     PT_Number,
			fn:          func_Divide,
		},
		FT_Multiply: {
			Name:        FT_Multiply,
			Description: "Multiplies the value by the parameter",
			Params:      singleParam("number to multiply by", PT_Number),
			Returns:     PT_Number,
			ValidOn:     PT_Number,
			fn:          func_Multiply,
		},
		FT_Modulo: {
			Name:        FT_Modulo,
			Description: "Returns the remainder of the value after dividing the value by the parameter",
			Params:      singleParam("number to modulo by", PT_Number),
			Returns:     PT_Number,
			ValidOn:     PT_Number,
			fn:          func_Modulo,
		},
		FT_AnyOf: {
			Name:        FT_AnyOf,
			Description: "Checks whether the value matches any of the parameters",
			Params:      singleParam("the values to match against", PT_Array),
			Returns:     PT_Boolean,
			ValidOn:     PT_Any,
			fn:          func_AnyOf,
		},
		FT_TrimRight: {
			Name:        FT_TrimRight,
			Description: "Removes the 'n' most characters of the value from the right, where 'n' is the parameter",
			Params:      singleParam("number of characters", PT_Number),
			Returns:     PT_String,
			ValidOn:     PT_String,
			fn:          func_TrimRight,
		},
		FT_TrimLeft: {
			Name:        FT_TrimLeft,
			Description: "Removes the 'n' most characters of the value from the left, where 'n' is the parameter",
			Params:      singleParam("number of characters", PT_Number),
			Returns:     PT_String,
			ValidOn:     PT_String,
			fn:          func_TrimLeft,
		},
		FT_Right: {
			Name:        FT_Right,
			Description: "Returns the 'n' most characters of the value from the right, where 'n' is the parameter'",
			Params:      singleParam("number of characters", PT_Number),
			Returns:     PT_String,
			ValidOn:     PT_String,
			fn:          func_Right,
		},
		FT_Left: {
			Name:        FT_Left,
			Description: "Returns the 'n' most characters of the value from the left, where 'n' is the parameter",
			Params:      singleParam("number of characters", PT_Number),
			Returns:     PT_String,
			ValidOn:     PT_String,
			fn:          func_Left,
		},
		FT_DoesMatchRegex: {
			Name:        FT_DoesMatchRegex,
			Description: "Checks whether the value matches the regular expression in the parameter",
			Params:      singleParam("regular expression to match", PT_String),
			Returns:     PT_Boolean,
			ValidOn:     PT_String,
			fn:          func_DoesMatchRegex,
		},
		FT_ReplaceRegex: {
			Name:        FT_ReplaceRegex,
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
			Returns: PT_String,
			ValidOn: PT_String,
			fn:      func_ReplaceRegex,
		},
		FT_ReplaceAll: {
			Name:        FT_ReplaceAll,
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
			Returns: PT_String,
			ValidOn: PT_String,
			fn:      func_ReplaceAll,
		},
		FT_AsJSON: {
			Name:        FT_AsJSON,
			Description: "Returns the value represented as JSON",
			Params:      nil,
			Returns:     PT_String,
			ValidOn:     PT_Any,
			fn:          func_AsJSON,
		},
		FT_ParseJSON: {
			Name:        FT_ParseJSON,
			Description: "Parses the value as JSON and returns an object or array",
			Params:      nil,
			Returns:     PT_Object,
			ValidOn:     PT_String,
			fn:          func_ParseJSON,
		},
		FT_ParseXML: {
			Name:        FT_ParseXML,
			Description: "Parses the value as XML and returns an object or array",
			Params:      nil,
			Returns:     PT_Object,
			ValidOn:     PT_String,
			fn:          func_ParseXML,
		},
		FT_ParseYAML: {
			Name:        FT_ParseYAML,
			Description: "Parses the value as YAML and returns an object or array",
			Params:      nil,
			Returns:     PT_Object,
			ValidOn:     PT_String,
			fn:          func_ParseYAML,
		},
		FT_ParseTOML: {
			Name:        FT_ParseTOML,
			Description: "Parses the value as TOML and returns an object or array",
			Params:      nil,
			Returns:     PT_Object,
			ValidOn:     PT_String,
			fn:          func_ParseTOML,
		},
		FT_RemoveKeysByRegex: {
			Name:        FT_RemoveKeysByRegex,
			Description: "Removes any keys that match the regular expression in the parameter",
			Params:      singleParam("regular expression to match", PT_String),
			Returns:     PT_Object,
			ValidOn:     PT_Object,
			fn:          func_RemoveKeysByRegex,
		},
		FT_RemoveKeysByPrefix: {
			Name:        FT_RemoveKeysByPrefix,
			Description: "Removes any keys that have a prefix as defined by the parameter",
			Params:      singleParam("prefix to match", PT_String),
			Returns:     PT_Object,
			ValidOn:     PT_Object,
			fn:          func_RemoveKeysByPrefix,
		},
		FT_RemoveKeysBySuffix: {
			Name:        FT_RemoveKeysBySuffix,
			Description: "Removes any keys that have a suffix as defined by the parameter",
			Params:      singleParam("suffix to match", PT_String),
			Returns:     PT_Object,
			ValidOn:     PT_Object,
			fn:          func_RemoveKeysBySuffix,
		},
	}
)

func ft_ShouldContinueForPath(ft FT_FunctionType) bool {
	switch ft {
	case FT_First, FT_Last, FT_Index:
		return true
	}

	return false
}

func ft_IsBoolFunc(ft FT_FunctionType) bool {
	fn, ok := funcMap[ft]
	if !ok {
		return false
	}

	return fn.Returns == PT_Boolean
}
