package mpath

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"

	xj "github.com/basgys/goxml2json"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pelletier/go-toml/v2"
	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v2"
)

func paramsGetFirstOfAny(rtParams FunctionParameterTypes) (val any, err error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return nil, fmt.Errorf("expected %d params, got %d", 1, got)
	}

	for _, p := range rtParams {
		return p.GetValue(), nil
	}

	return nil, fmt.Errorf("no parameters found")
}

func paramsGetFirstOfNumber(rtParams FunctionParameterTypes) (val decimal.Decimal, err error) {
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

func paramsGetFirstOfString(rtParams FunctionParameterTypes) (val string, err error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return val, fmt.Errorf("expected %d params, got %d", 1, got)
	}

	for _, p := range rtParams.Strings() {
		return p.Value, nil
	}

	return val, fmt.Errorf("no string parameter found")
}

func paramsGetAll(rtParams FunctionParameterTypes) (val []any, err error) {
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

func (rtParams *FunctionParameterTypes) checkLengthOfParams(allowed int) (got int, ok bool) {
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

func func_Equal(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_NotEqual(rtParams FunctionParameterTypes, val any) (any, error) {
	res, err := func_Equal(rtParams, val)
	if err != nil {
		return false, err
	}

	resBool, ok := res.(bool)
	if !ok {
		return false, fmt.Errorf("result was not a boolean")
	}

	return !resBool, nil
}

func decimalBoolFunc(rtParams FunctionParameterTypes, val any, fn func(d1, d2 decimal.Decimal) bool, fnName FT_FunctionType) (bool, error) {
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

func func_Less(rtParams FunctionParameterTypes, val any) (any, error) {
	return decimalBoolFunc(rtParams, val, func(d1, d2 decimal.Decimal) bool {
		return d1.LessThan(d2)
	}, FT_Less)
}

const FT_LessOrEqual FT_FunctionType = "LessOrEqual"

func func_LessOrEqual(rtParams FunctionParameterTypes, val any) (any, error) {
	return decimalBoolFunc(rtParams, val, func(d1, d2 decimal.Decimal) bool {
		return d1.LessThanOrEqual(d2)
	}, FT_LessOrEqual)
}

const FT_Greater FT_FunctionType = "Greater"

func func_Greater(rtParams FunctionParameterTypes, val any) (any, error) {
	return decimalBoolFunc(rtParams, val, func(d1, d2 decimal.Decimal) bool {
		return d1.GreaterThan(d2)
	}, FT_Greater)
}

const FT_GreaterOrEqual FT_FunctionType = "GreaterOrEqual"

func func_GreaterOrEqual(rtParams FunctionParameterTypes, val any) (any, error) {
	return decimalBoolFunc(rtParams, val, func(d1, d2 decimal.Decimal) bool {
		return d1.GreaterThanOrEqual(d2)
	}, FT_GreaterOrEqual)
}

const FT_Invert FT_FunctionType = "Invert"

func func_Invert(rtParams FunctionParameterTypes, val any) (any, error) {
	switch t := val.(type) {
	case bool:
		return !t, nil
	case *bool:
		return !(*t), nil
	}

	return false, fmt.Errorf("input was not boolean")
}

func stringBoolFunc(rtParams FunctionParameterTypes, val any, fn func(string, string) bool, invert bool, fnName FT_FunctionType) (bool, error) {
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

	if reader, ok := val.(io.Reader); ok {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			if fn(line, param) {
				if invert {
					return false, nil
				}
				return true, nil
			}
		}
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("error reading stream: %w", err)
		}
		return invert, nil
	}

	return false, fmt.Errorf("parameter wasn't string")
}

const FT_Contains FT_FunctionType = "Contains"

func func_Contains(rtParams FunctionParameterTypes, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.Contains, false, FT_Contains)
}

const FT_NotContains FT_FunctionType = "NotContains"

func func_NotContains(rtParams FunctionParameterTypes, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.Contains, true, FT_NotContains)
}

const FT_Prefix FT_FunctionType = "Prefix"

func func_Prefix(rtParams FunctionParameterTypes, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.HasPrefix, false, FT_Prefix)
}

const FT_NotPrefix FT_FunctionType = "NotPrefix"

func func_NotPrefix(rtParams FunctionParameterTypes, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.HasPrefix, true, FT_NotPrefix)
}

const FT_Suffix FT_FunctionType = "Suffix"

func func_Suffix(rtParams FunctionParameterTypes, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.HasSuffix, false, FT_Suffix)
}

const FT_NotSuffix FT_FunctionType = "NotSuffix"

func func_NotSuffix(rtParams FunctionParameterTypes, val any) (any, error) {
	return stringBoolFunc(rtParams, val, strings.HasSuffix, true, FT_NotSuffix)
}

const FT_Sprintf FT_FunctionType = "Sprintf"

func func_Sprintf(rtParams FunctionParameterTypes, val any) (any, error) {
	valStr, ok := val.(string)
	if !ok {
		return errString(FT_Sprintf, fmt.Errorf("input was not a string"))
	}

	allparams, err := paramsGetAll(rtParams)
	if err != nil {
		return errString(FT_Sprintf, err)
	}

	pLen := len(allparams)
	switch pLen {
	case 0:
		return valStr, nil
	default:
		return fmt.Sprintf(valStr, allparams[1:]...), nil
	}
}

const FT_Count FT_FunctionType = "Count"

func func_Count(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_Any(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_First(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_Last(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_Index(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_decimalSlice(rtParams FunctionParameterTypes, val any, decimalSliceFunction func(decimal.Decimal, ...decimal.Decimal) decimal.Decimal) (any, error) {
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

	if isMap(val) {
		var err error
		val, err = getMapValues(val)
		if err != nil {
			return false, fmt.Errorf("not an array of numbers")
		}
	}

	var newSlc []decimal.Decimal
	switch valueInstance := val.(type) {
	case []decimal.Decimal:
		newSlc = append([]decimal.Decimal{}, valueInstance...)
		newSlc = append(newSlc, paramNumbers...)
	case []any:
		newSlc = append([]decimal.Decimal{}, paramNumbers...)
		for _, vs := range valueInstance {
			switch t := vs.(type) {
			case decimal.Decimal:
				newSlc = append(newSlc, t)
			case string:
				wasNumber, number := convertToDecimalIfNumberAndCheck(t)
				if wasNumber {
					newSlc = append(newSlc, number)
					continue
				}
				goto notArrayOfNumbers
			case float64:
				newSlc = append(newSlc, decimal.NewFromFloat(t))
			default:
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
	return false, fmt.Errorf("not an array of numbers")
}

const FT_Sum FT_FunctionType = "Sum"

func func_Sum(rtParams FunctionParameterTypes, val any) (any, error) {
	return func_decimalSlice(rtParams, val, decimal.Sum)
}

const FT_Average FT_FunctionType = "Average"

func func_Average(rtParams FunctionParameterTypes, val any) (any, error) {
	return func_decimalSlice(rtParams, val, decimal.Avg)
}

const FT_Minimum FT_FunctionType = "Minimum"

func func_Minimum(rtParams FunctionParameterTypes, val any) (any, error) {
	return func_decimalSlice(rtParams, val, decimal.Min)
}

const FT_Maximum FT_FunctionType = "Maximum"

func func_Maximum(rtParams FunctionParameterTypes, val any) (any, error) {
	return func_decimalSlice(rtParams, val, decimal.Max)
}

func func_decimal(rtParams FunctionParameterTypes, val any, decSlcFunc func(decimal.Decimal, decimal.Decimal) decimal.Decimal, name FT_FunctionType) (any, error) {
	param, err := paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return errBool(name, err)
	}

	if valIfc, ok := val.(decimal.Decimal); ok {
		return decSlcFunc(valIfc, param), nil
	}

	return false, fmt.Errorf("not a number")
}

const FT_AsArray FT_FunctionType = "AsArray"

func func_AsArray(_ FunctionParameterTypes, val any) (any, error) {
	return []any{val}, nil
}

const FT_Add FT_FunctionType = "Add"

func func_Add(rtParams FunctionParameterTypes, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Add, FT_Add)
}

const FT_Subtract FT_FunctionType = "Subtract"

func func_Subtract(rtParams FunctionParameterTypes, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Sub, FT_Subtract)
}

const FT_Divide FT_FunctionType = "Divide"

func func_Divide(rtParams FunctionParameterTypes, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Div, FT_Divide)
}

const FT_Multiply FT_FunctionType = "Multiply"

func func_Multiply(rtParams FunctionParameterTypes, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Mul, FT_Multiply)
}

const FT_Modulo FT_FunctionType = "Modulo"

func func_Modulo(rtParams FunctionParameterTypes, val any) (any, error) {
	return func_decimal(rtParams, val, decimal.Decimal.Mod, FT_Modulo)
}

const FT_AnyOf FT_FunctionType = "AnyOf"

func func_AnyOf(rtParams FunctionParameterTypes, val any) (any, error) {
	params, err := paramsGetAll(rtParams)
	if err != nil {
		return errBool(FT_AnyOf, err)
	}

	if len(params) == 0 {
		return false, nil
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

func stringPartFunc(rtParams FunctionParameterTypes, val any, fn func(string, int) (string, error), fnName FT_FunctionType) (string, error) {
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

func func_TrimRight(rtParams FunctionParameterTypes, val any) (any, error) {
	return stringPartFunc(rtParams, val, func(s string, i int) (string, error) {
		if len(s) <= i {
			return "", nil
		}

		return s[:len(s)-i], nil
	}, FT_TrimRight)
}

const FT_TrimLeft FT_FunctionType = "TrimLeft"

func func_TrimLeft(rtParams FunctionParameterTypes, val any) (any, error) {
	return stringPartFunc(rtParams, val, func(s string, i int) (string, error) {
		if len(s) <= i {
			return "", nil
		}

		return s[i:], nil
	}, FT_TrimLeft)
}

const FT_Right FT_FunctionType = "Right"

func func_Right(rtParams FunctionParameterTypes, val any) (any, error) {
	return stringPartFunc(rtParams, val, func(s string, i int) (string, error) {
		if len(s) < i {
			return s, nil
		}

		return s[len(s)-i:], nil
	}, FT_Right)
}

const FT_Left FT_FunctionType = "Left"

func func_Left(rtParams FunctionParameterTypes, val any) (any, error) {
	return stringPartFunc(rtParams, val, func(s string, i int) (string, error) {
		if len(s) < i {
			return s, nil
		}

		return s[:i], nil
	}, FT_Left)
}

const FT_DoesMatchRegex FT_FunctionType = "DoesMatchRegex"

func func_DoesMatchRegex(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_ReplaceRegex(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_ReplaceAll(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_AsJSON(rtParams FunctionParameterTypes, val any) (any, error) {
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

func stringToObjectFunc(rtParams FunctionParameterTypes, val any, fn func(s string) (map[string]any, error), _ FT_FunctionType) (map[string]any, error) {
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

func func_ParseJSON(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_ParseXML(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_ParseYAML(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_ParseTOML(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_RemoveKeysByRegex(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_RemoveKeysByPrefix(rtParams FunctionParameterTypes, val any) (any, error) {
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

func func_RemoveKeysBySuffix(rtParams FunctionParameterTypes, val any) (any, error) {
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

const FT_Not FT_FunctionType = "Not"

func func_Not(rtParams FunctionParameterTypes, val any) (any, error) {
	if vb, ok := val.(bool); ok {
		return !vb, nil
	}

	return false, fmt.Errorf("value is not a boolean")
}

const FT_IsNull FT_FunctionType = "IsNull"

func func_IsNull(rtParams FunctionParameterTypes, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return false, errNumParams(FT_IsNull, 0, got)
	}

	return isNil(val), nil
}

const FT_IsNotNull FT_FunctionType = "IsNotNull"

func func_IsNotNull(rtParams FunctionParameterTypes, val any) (any, error) {
	res, err := func_IsNull(rtParams, val)
	if err != nil {
		return false, err
	}

	resBool, ok := res.(bool)
	if !ok {
		return false, fmt.Errorf("result was not a boolean")
	}

	return !resBool, nil
}

const FT_IsEmpty FT_FunctionType = "IsEmpty"

func func_IsEmpty(rtParams FunctionParameterTypes, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return false, errNumParams(FT_IsEmpty, 0, got)
	}

	value := reflect.ValueOf(val)

	// Get the zero value for the type of val
	zeroValue := reflect.Zero(value.Type())
	zeroValueAsInterface := zeroValue.Interface()

	// Compare the value with the zero value
	isEmpty := cmp.Equal(val, zeroValueAsInterface, cmpopts.EquateEmpty())

	return isEmpty, nil
}

const FT_IsNotEmpty FT_FunctionType = "IsNotEmpty"

func func_IsNotEmpty(rtParams FunctionParameterTypes, val any) (any, error) {
	res, err := func_IsEmpty(rtParams, val)
	if err != nil {
		return false, err
	}

	resBool, ok := res.(bool)
	if !ok {
		return false, fmt.Errorf("result was not a boolean")
	}

	return !resBool, nil
}

const FT_IsNullOrEmpty FT_FunctionType = "IsNullOrEmpty"

func func_IsNullOrEmpty(rtParams FunctionParameterTypes, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return false, errNumParams(FT_IsNull, 0, got)
	}

	res, err := func_IsNull(rtParams, val)
	if err != nil {
		return false, err
	}

	if resBool, ok := res.(bool); ok && resBool {
		return true, nil
	}

	// Check if the value is the zero value of its type
	res, err = func_IsEmpty(rtParams, val)
	if err != nil {
		return false, err
	}

	if resBool, ok := res.(bool); ok && resBool {
		return true, nil
	}

	return false, nil
}

const FT_IsNotNullOrEmpty FT_FunctionType = "IsNotNullOrEmpty"

func func_IsNotNullOrEmpty(rtParams FunctionParameterTypes, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return false, errNumParams(FT_IsNull, 0, got)
	}

	res, err := func_IsNullOrEmpty(rtParams, val)
	if err != nil {
		return false, err
	}

	resBool, ok := res.(bool)
	if !ok {
		return false, fmt.Errorf("result was not a boolean")
	}

	return !resBool, nil
}

const FT_Select FT_FunctionType = "Select"

func func_Select(rtParams FunctionParameterTypes, val any) (any, error) {
	// Expect exactly one parameter: the query string.
	query, err := paramsGetFirstOfString(rtParams)
	if err != nil {
		return errString(FT_Select, err)
	}

	// Parse the query string (e.g., "$.IntField") into an operation.
	op, err := ParseString(query)
	if err != nil {
		return nil, fmt.Errorf("func %s: error parsing query: %w", FT_Select, err)
	}

	// Prepare to iterate over the input using reflection.
	v := reflect.ValueOf(val)
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	var results []any

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		// Iterate over each element in the slice/array.
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i).Interface()
			res, err := op.Do(elem, elem)
			if err != nil {
				return nil, fmt.Errorf("func %s: error selecting field: %w", FT_Select, err)
			}
			// If the result is itself a slice/array, flatten it.
			resVal := reflect.ValueOf(res)
			if resVal.Kind() == reflect.Slice || resVal.Kind() == reflect.Array {
				for j := 0; j < resVal.Len(); j++ {
					results = append(results, resVal.Index(j).Interface())
				}
			} else {
				results = append(results, res)
			}
		}
	case reflect.Map:
		// Iterate over map values.
		orderedMapValues := v.MapKeys()
		// Sort the keys to ensure deterministic output.
		sort.Slice(orderedMapValues, func(i, j int) bool {
			return orderedMapValues[i].String() < orderedMapValues[j].String()
		})

		for _, key := range orderedMapValues {
			elem := v.MapIndex(key).Interface()
			res, err := op.Do(elem, elem)
			if err != nil {
				return nil, fmt.Errorf("func %s: error selecting field: %w", FT_Select, err)
			}
			resVal := reflect.ValueOf(res)
			if resVal.Kind() == reflect.Slice || resVal.Kind() == reflect.Array {
				for j := 0; j < resVal.Len(); j++ {
					results = append(results, resVal.Index(j).Interface())
				}
			} else {
				results = append(results, res)
			}
		}
	default:
		return nil, fmt.Errorf("func %s: unsupported type %T; expected array or map", FT_Select, val)
	}
	return results, nil
}

func isNil(val any) bool {
	value := reflect.ValueOf(val)

	switch vk := value.Kind(); vk {
	case reflect.Ptr,
		reflect.Interface,
		reflect.Slice,
		reflect.Map,
		reflect.Chan,
		reflect.Func:
		isNil := value.IsNil()
		return isNil
	case reflect.Invalid:
		return true
	}

	// any value that cannot be null is by definition, not null
	return false
}

type FT_FunctionType string

func singleParam(name string, typ PT_ParameterType, ioType IOOT_InputOrOutputType) []ParameterDescriptor {
	return []ParameterDescriptor{
		{
			InputOrOutput: inputOrOutput(typ, ioType),
			Name:          name,
		},
	}
}

func inputOrOutput(typ PT_ParameterType, ioType IOOT_InputOrOutputType) InputOrOutput {
	return InputOrOutput{
		IOType: ioType,
		Type:   typ,
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
	PT_String      PT_ParameterType = "String"
	PT_Bytes       PT_ParameterType = "Bytes"
	PT_Boolean     PT_ParameterType = "Boolean"
	PT_Number      PT_ParameterType = "Number"
	PT_Any         PT_ParameterType = "Any"
	PT_Object      PT_ParameterType = "Object"
	PT_Root        PT_ParameterType = "Root"
	PT_ElementRoot PT_ParameterType = "ElementRoot"
)

func (pt PT_ParameterType) IsPrimitive() bool {
	switch pt {
	case PT_String, PT_Boolean, PT_Number:
		return true
	}

	return false
}

func (pt PT_ParameterType) CueExpr() *string {
	out := ""
	switch pt {
	case PT_String:
		out = "string"
	case PT_Boolean:
		out = "bool"
	case PT_Number:
		out = "number"
	case PT_Any:
		out = "_"
	case PT_Object:
		out = "{...}"
	case PT_Bytes:
		out = "bytes"
	}

	return &out
}

type FunctionDescriptor struct {
	Name               FT_FunctionType       `json:"name"`
	Description        string                `json:"description"`
	Params             []ParameterDescriptor `json:"params"`
	ValidOn            InputOrOutput         `json:"validOn"`
	Returns            InputOrOutput         `json:"returns"`
	ReturnsKnownValues bool                  `json:"returnsKnownValues"`

	fn              FuncFunction
	explanationFunc func(tf Function) string
}

type FuncFunction func(rtParams FunctionParameterTypes, val any) (any, error)

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
		ValidOn      InputOrOutput         `json:"validOn"`
		Returns      InputOrOutput         `json:"returns"`
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

func getAvailableFunctionsForKind(iot InputOrOutput) (names []string) {
	for _, fd := range funcMap {
		if iot == fd.ValidOn || (fd.ValidOn.Type == PT_Any && (fd.ValidOn.IOType == IOOT_Variadic || fd.ValidOn.IOType == iot.IOType)) {
			names = append(names, string(fd.Name))
		}
	}

	sort.Strings(names)

	return
}

type IOOT_InputOrOutputType string

const (
	IOOT_Single   IOOT_InputOrOutputType = "Single"
	IOOT_Array    IOOT_InputOrOutputType = "Array"
	IOOT_Variadic IOOT_InputOrOutputType = "Variadic"
)

type InputOrOutput struct {
	Type    PT_ParameterType       `json:"type"`
	IOType  IOOT_InputOrOutputType `json:"ioType"`
	CueExpr *string                `json:"cueExpr,omitempty"`
}

type ParameterDescriptor struct {
	InputOrOutput
	Name string `json:"name"`
}

var (
	funcMap = map[FT_FunctionType]FunctionDescriptor{
		FT_Not: {
			Name:        FT_Not,
			Description: "Inverts a boolean result",
			Params:      nil,
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Boolean, IOOT_Single),
			fn:          func_Not,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return "is not"
			},
		},
		FT_IsNull: {
			Name:        FT_IsNull,
			Description: "Checks whether the value is null",
			Params:      nil,
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Any, IOOT_Variadic),
			fn:          func_IsNull,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return "is equal to null"
			},
		},
		FT_IsNotNull: {
			Name:        FT_IsNotNull,
			Description: "Checks whether the value is not null",
			Params:      nil,
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Any, IOOT_Variadic),
			fn:          func_IsNotNull,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return "is not equal to null"
			},
		},
		FT_IsNullOrEmpty: {
			Name:        FT_IsNullOrEmpty,
			Description: "Checks whether the value is null or empty",
			Params:      nil,
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Any, IOOT_Variadic),
			fn:          func_IsNullOrEmpty,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return "is equal to null or is empty"
			},
		},
		FT_IsNotNullOrEmpty: {
			Name:        FT_IsNotNullOrEmpty,
			Description: "Checks whether the value is not null or is not empty",
			Params:      nil,
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Any, IOOT_Variadic),
			fn:          func_IsNotNullOrEmpty,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return "is not equal to null or is not empty"
			},
		},
		FT_IsEmpty: {
			Name:        FT_IsEmpty,
			Description: "Checks whether the value is empty",
			Params:      nil,
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Any, IOOT_Variadic),
			fn:          func_IsEmpty,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return "is empty"
			},
		},
		FT_IsNotEmpty: {
			Name:        FT_IsNotEmpty,
			Description: "Checks whether the value is not empty",
			Params:      nil,
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Any, IOOT_Variadic),
			fn:          func_IsNotEmpty,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return "is not empty"
			},
		},
		FT_Select: {
			Name:        FT_Select,
			Description: "Selects a field from each object in a map or array",
			Params:      singleParam("mpath query to run against each element", PT_String, IOOT_Single),
			Returns:     inputOrOutput(PT_Any, IOOT_Array),
			ValidOn:     inputOrOutput(PT_Any, IOOT_Array),
			fn:          func_Select,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("selects the field {{%s}}", tf.FunctionParameters[0].String)
			},
		},

		FT_Equal: {
			Name:        FT_Equal,
			Description: "Checks whether the value equals the parameter",
			Params:      singleParam("value to match", PT_Any, IOOT_Single),
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Any, IOOT_Variadic),
			fn:          func_Equal,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("is equal to {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_NotEqual: {
			Name:        FT_NotEqual,
			Description: "Checks whether the value does not equal the parameter",
			Params:      singleParam("value to match", PT_Any, IOOT_Single),
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Any, IOOT_Variadic),
			fn:          func_NotEqual,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("is not equal to {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_Less: {
			Name:        FT_Less,
			Description: "Checks whether the value is less than the parameter",
			Params:      singleParam("number to compare", PT_Number, IOOT_Single),
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Number, IOOT_Single),
			fn:          func_Less,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("is less than {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_LessOrEqual: {
			Name:        FT_LessOrEqual,
			Description: "Checks whether the value is less than or equal to the parameter",
			Params:      singleParam("number to compare", PT_Number, IOOT_Single),
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Number, IOOT_Single),
			fn:          func_LessOrEqual,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("is less than or equal to {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_Greater: {
			Name:        FT_Greater,
			Description: "Checks whether the value is greater than the parameter",
			Params:      singleParam("number to compare", PT_Number, IOOT_Single),
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Number, IOOT_Single),
			fn:          func_Greater,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("is greater than {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_GreaterOrEqual: {
			Name:        FT_GreaterOrEqual,
			Description: "Checks whether the value is greater than or equal to the parameter",
			Params:      singleParam("number to compare", PT_Number, IOOT_Single),
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Number, IOOT_Single),
			fn:          func_GreaterOrEqual,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("is greater than or equal to {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_Invert: {
			Name:        FT_Invert,
			Description: "Inverts a boolean (swaps true to false, or false to true)",
			Params:      nil,
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Boolean, IOOT_Single),
			fn:          func_Invert,
			explanationFunc: func(tf Function) string {
				return "inverts the input"
			},
		},
		FT_Contains: {
			Name:        FT_Contains,
			Description: "Checks whether the value contains the parameter",
			Params:      singleParam("string to match", PT_String, IOOT_Single),
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_Contains,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("contains the string {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_NotContains: {
			Name:        FT_NotContains,
			Description: "Checks whether the value does not contain the parameter",
			Params:      singleParam("string to match", PT_String, IOOT_Single),
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_NotContains,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("does not contain the string {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_Prefix: {
			Name:        FT_Prefix,
			Description: "Checks whether the value has the parameter as a prefix",
			Params:      singleParam("prefix to match", PT_String, IOOT_Single),
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_Prefix,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("has the prefix {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_NotPrefix: {
			Name:        FT_NotPrefix,
			Description: "Checks whether the value does not have the parameter as a prefix",
			Params:      singleParam("prefix to match", PT_String, IOOT_Single),
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_NotPrefix,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("does not have the prefix {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_Suffix: {
			Name:        FT_Suffix,
			Description: "Checks whether the value has the parameter as a suffix",
			Params:      singleParam("suffix to match", PT_String, IOOT_Single),
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_Suffix,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("has the suffix {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_NotSuffix: {
			Name:        FT_NotSuffix,
			Description: "Checks whether the value does not have the parameter as a suffix",
			Params:      singleParam("suffix to match", PT_String, IOOT_Single),
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_NotSuffix,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("does not have the suffix {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_Sprintf: {
			Name:        FT_Sprintf,
			Description: "Builds a string based on templated values (ignores input)",
			Params:      singleParam("arguments", PT_Any, IOOT_Variadic),
			Returns:     inputOrOutput(PT_String, IOOT_Single),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_Sprintf,
			explanationFunc: func(tf Function) string {
				return "builds a string from the input as a template" //todo: do better
			},
		},
		FT_Count: {
			Name:        FT_Count,
			Description: "Returns the count of elements in the array",
			Params:      nil,
			Returns:     inputOrOutput(PT_Number, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Any, IOOT_Array),
			fn:          func_Count,
			explanationFunc: func(tf Function) string {
				return "count of the number of elements"
			},
		},
		FT_First: {
			Name:               FT_First,
			Description:        "Returns the first element of the array",
			Params:             nil,
			Returns:            inputOrOutput(PT_Any, IOOT_Single),
			ValidOn:            inputOrOutput(PT_Any, IOOT_Array),
			ReturnsKnownValues: true,
			fn:                 func_First,
			explanationFunc: func(tf Function) string {
				return "the first element"
			},
		},
		FT_Last: {
			Name:               FT_Last,
			Description:        "Returns the last element of the array",
			Params:             nil,
			Returns:            inputOrOutput(PT_Any, IOOT_Single),
			ValidOn:            inputOrOutput(PT_Any, IOOT_Array),
			ReturnsKnownValues: true,
			fn:                 func_Last,
			explanationFunc: func(tf Function) string {
				return "the last element"
			},
		},
		FT_Index: {
			Name:               FT_Index,
			Description:        "Returns the element at the zero based index of the array",
			Params:             singleParam("index", PT_Number, IOOT_Single),
			Returns:            inputOrOutput(PT_Any, IOOT_Single),
			ValidOn:            inputOrOutput(PT_Any, IOOT_Array),
			ReturnsKnownValues: true,
			fn:                 func_Index,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("the element at index {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_Any: {
			Name:        FT_Any,
			Description: "Checks whether there are any elements in the array",
			Params:      nil,
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Any, IOOT_Array),
			fn:          func_Any,
			explanationFunc: func(tf Function) string {
				return "has any elements"
			},
		},
		FT_Sum: {
			Name:        FT_Sum,
			Description: "Sums the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers", PT_Number, IOOT_Variadic),
			Returns:     inputOrOutput(PT_Number, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Number, IOOT_Array),
			fn:          func_Sum,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) == 0 {
					return "the sum of all elements"
				}

				return fmt.Sprintf("the sum of all elements, including %s", tf.FunctionParameters[0].String)
			},
		},
		FT_Average: {
			Name:        FT_Average,
			Description: "Averages the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers", PT_Number, IOOT_Variadic),
			Returns:     inputOrOutput(PT_Number, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Number, IOOT_Array),
			fn:          func_Average,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) == 0 {
					return "the average of all elements"
				}

				return fmt.Sprintf("the average of all elements, including %s", tf.FunctionParameters[0].String)
			},
		},
		FT_Maximum: {
			Name:        FT_Maximum,
			Description: "Returns the maximum of the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers", PT_Number, IOOT_Variadic),
			Returns:     inputOrOutput(PT_Number, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Number, IOOT_Array),
			fn:          func_Maximum,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) == 0 {
					return "the maximum of all elements"
				}

				return fmt.Sprintf("the maximum of all elements, including %s", tf.FunctionParameters[0].String)
			},
		},
		FT_Minimum: {
			Name:        FT_Minimum,
			Description: "Returns the minimum of the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers", PT_Number, IOOT_Variadic),
			Returns:     inputOrOutput(PT_Number, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Number, IOOT_Array),
			fn:          func_Minimum,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) == 0 {
					return "the minimum of all elements"
				}

				return fmt.Sprintf("the minimum of all elements, including %s", tf.FunctionParameters[0].String)
			},
		},
		FT_AsArray: {
			Name:               FT_AsArray,
			Description:        "Returns an array of length 1 that contains the value",
			Params:             nil,
			Returns:            inputOrOutput(PT_Any, IOOT_Array),
			ValidOn:            inputOrOutput(PT_Any, IOOT_Array),
			ReturnsKnownValues: true,
			fn:                 func_AsArray,
			explanationFunc: func(tf Function) string {
				return "creates an array that contains the value"
			},
		},
		FT_Add: {
			Name:        FT_Add,
			Description: "Adds the parameter to the value",
			Params:      singleParam("number to add", PT_Number, IOOT_Single),
			Returns:     inputOrOutput(PT_Number, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Number, IOOT_Single),
			fn:          func_Add,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("adds {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_Subtract: {
			Name:        FT_Subtract,
			Description: "Subtracts the parameter from the value",
			Params:      singleParam("number to subtract", PT_Number, IOOT_Single),
			Returns:     inputOrOutput(PT_Number, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Number, IOOT_Single),
			fn:          func_Subtract,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("subtracts {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_Divide: {
			Name:        FT_Divide,
			Description: "Divides the value by the parameter",
			Params:      singleParam("number to divide by", PT_Number, IOOT_Single),
			Returns:     inputOrOutput(PT_Number, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Number, IOOT_Single),
			fn:          func_Divide,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("divides by {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_Multiply: {
			Name:        FT_Multiply,
			Description: "Multiplies the value by the parameter",
			Params:      singleParam("number to multiply by", PT_Number, IOOT_Single),
			Returns:     inputOrOutput(PT_Number, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Number, IOOT_Single),
			fn:          func_Multiply,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("multiplies by {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_Modulo: {
			Name:        FT_Modulo,
			Description: "Returns the remainder of the value after dividing the value by the parameter",
			Params:      singleParam("number to modulo by", PT_Number, IOOT_Single),
			Returns:     inputOrOutput(PT_Number, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Number, IOOT_Single),
			fn:          func_Modulo,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("the remainder after dividing by {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_AnyOf: {
			Name:        FT_AnyOf,
			Description: "Checks whether the value matches any of the parameters",
			Params:      singleParam("the values to match against", PT_Any, IOOT_Variadic),
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Any, IOOT_Single),
			fn:          func_AnyOf,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) == 0 {
					return "will return false as there are no parameters to compare against"
				}

				paramStrs := []string{}

				for _, ps := range tf.FunctionParameters {
					paramStrs = append(paramStrs, fmt.Sprintf("{{%s}}", ps.String))
				}

				return fmt.Sprintf("checks whether the value is any of %s", strings.Join(paramStrs, ", "))
			},
		},
		FT_TrimRight: {
			Name:        FT_TrimRight,
			Description: "Removes the 'n' most characters of the value from the right, where 'n' is the parameter",
			Params:      singleParam("number of characters", PT_Number, IOOT_Single),
			Returns:     inputOrOutput(PT_String, IOOT_Single),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_TrimRight,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("trims the right {{%s}} characters", tf.FunctionParameters[0].String)
			},
		},
		FT_TrimLeft: {
			Name:        FT_TrimLeft,
			Description: "Removes the 'n' most characters of the value from the left, where 'n' is the parameter",
			Params:      singleParam("number of characters", PT_Number, IOOT_Single),
			Returns:     inputOrOutput(PT_String, IOOT_Single),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_TrimLeft,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("trims the left {{%s}} characters", tf.FunctionParameters[0].String)
			},
		},
		FT_Right: {
			Name:        FT_Right,
			Description: "Returns the 'n' most characters of the value from the right, where 'n' is the parameter'",
			Params:      singleParam("number of characters", PT_Number, IOOT_Single),
			Returns:     inputOrOutput(PT_String, IOOT_Single),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_Right,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("keeps only the right {{%s}} characters", tf.FunctionParameters[0].String)
			},
		},
		FT_Left: {
			Name:        FT_Left,
			Description: "Returns the 'n' most characters of the value from the left, where 'n' is the parameter",
			Params:      singleParam("number of characters", PT_Number, IOOT_Single),
			Returns:     inputOrOutput(PT_String, IOOT_Single),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_Left,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("keeps only the left {{%s}} characters", tf.FunctionParameters[0].String)
			},
		},
		FT_DoesMatchRegex: {
			Name:        FT_DoesMatchRegex,
			Description: "Checks whether the value matches the regular expression in the parameter",
			Params:      singleParam("regular expression to match", PT_String, IOOT_Single),
			Returns:     inputOrOutput(PT_Boolean, IOOT_Single),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_DoesMatchRegex,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("checks whether matches the regex {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_ReplaceRegex: {
			Name:        FT_ReplaceRegex,
			Description: "Replaces any matches of the regular expression parameter in the value with the replacement parameter",
			Params: []ParameterDescriptor{
				{
					Name:          "regular expression",
					InputOrOutput: inputOrOutput(PT_String, IOOT_Single),
				},
				{
					Name:          "replacement",
					InputOrOutput: inputOrOutput(PT_String, IOOT_Single),
				},
			},
			Returns: inputOrOutput(PT_String, IOOT_Single),
			ValidOn: inputOrOutput(PT_String, IOOT_Single),
			fn:      func_ReplaceRegex,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 2 {
					return ""
				}

				return fmt.Sprintf("replaces any matches for the regex {{%s}} with {{%s}}", tf.FunctionParameters[0].String, tf.FunctionParameters[1].String)
			},
		},
		FT_ReplaceAll: {
			Name:        FT_ReplaceAll,
			Description: "Replaces any matches of the string to match parameter in the value with the replacement parameter",
			Params: []ParameterDescriptor{
				{
					Name:          "string to match",
					InputOrOutput: inputOrOutput(PT_String, IOOT_Single),
				},
				{
					Name:          "replacement",
					InputOrOutput: inputOrOutput(PT_String, IOOT_Single),
				},
			},
			Returns: inputOrOutput(PT_String, IOOT_Single),
			ValidOn: inputOrOutput(PT_String, IOOT_Single),
			fn:      func_ReplaceAll,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 2 {
					return ""
				}

				return fmt.Sprintf("replaces any matches of {{%s}} with {{%s}}", tf.FunctionParameters[0].String, tf.FunctionParameters[1].String)
			},
		},
		FT_AsJSON: {
			Name:        FT_AsJSON,
			Description: "Returns the value represented as JSON",
			Params:      nil,
			Returns:     inputOrOutput(PT_String, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Any, IOOT_Variadic),
			fn:          func_AsJSON,
			explanationFunc: func(tf Function) string {
				return "converts to a JSON string"
			},
		},
		FT_ParseJSON: {
			Name:        FT_ParseJSON,
			Description: "Parses the value as JSON and returns an object or array",
			Params:      nil,
			Returns:     inputOrOutput(PT_Object, IOOT_Variadic),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_ParseJSON,
			explanationFunc: func(tf Function) string {
				return "parses as JSON to become an object"
			},
		},
		FT_ParseXML: {
			Name:        FT_ParseXML,
			Description: "Parses the value as XML and returns an object or array",
			Params:      nil,
			Returns:     inputOrOutput(PT_Object, IOOT_Variadic),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_ParseXML,
			explanationFunc: func(tf Function) string {
				return "parses as XML to become an object"
			},
		},
		FT_ParseYAML: {
			Name:        FT_ParseYAML,
			Description: "Parses the value as YAML and returns an object or array",
			Params:      nil,
			Returns:     inputOrOutput(PT_Object, IOOT_Variadic),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_ParseYAML,
			explanationFunc: func(tf Function) string {
				return "parses as YAML to become an object"
			},
		},
		FT_ParseTOML: {
			Name:        FT_ParseTOML,
			Description: "Parses the value as TOML and returns an object or array",
			Params:      nil,
			Returns:     inputOrOutput(PT_Object, IOOT_Variadic),
			ValidOn:     inputOrOutput(PT_String, IOOT_Single),
			fn:          func_ParseTOML,
			explanationFunc: func(tf Function) string {
				return "parses as TOML to become an object"
			},
		},
		FT_RemoveKeysByRegex: {
			Name:        FT_RemoveKeysByRegex,
			Description: "Removes any keys that match the regular expression in the parameter",
			Params:      singleParam("regular expression to match", PT_String, IOOT_Single),
			Returns:     inputOrOutput(PT_Object, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Object, IOOT_Single),
			fn:          func_RemoveKeysByRegex,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("removes keys matching regex {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_RemoveKeysByPrefix: {
			Name:        FT_RemoveKeysByPrefix,
			Description: "Removes any keys that have a prefix as defined by the parameter",
			Params:      singleParam("prefix to match", PT_String, IOOT_Single),
			Returns:     inputOrOutput(PT_Object, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Object, IOOT_Single),
			fn:          func_RemoveKeysByPrefix,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("removes any keys with prefix {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		FT_RemoveKeysBySuffix: {
			Name:        FT_RemoveKeysBySuffix,
			Description: "Removes any keys that have a suffix as defined by the parameter",
			Params:      singleParam("suffix to match", PT_String, IOOT_Single),
			Returns:     inputOrOutput(PT_Object, IOOT_Single),
			ValidOn:     inputOrOutput(PT_Object, IOOT_Single),
			fn:          func_RemoveKeysBySuffix,
			explanationFunc: func(tf Function) string {
				if len(tf.FunctionParameters) != 1 {
					return ""
				}

				return fmt.Sprintf("removes any keys with suffix {{%s}}", tf.FunctionParameters[0].String)
			},
		},
		/*
			- Functions to add:
				-	Select(fieldName string)
					This will return an array of [whatever the field type is] by selecting
					only that field from an array of objects
		*/
	}
)

func ft_IsBoolFunc(ft FT_FunctionType) bool {
	fn, ok := funcMap[ft]
	if !ok {
		return false
	}

	return fn.Returns.Type == PT_Boolean && fn.Returns.IOType == IOOT_Single
}

func isMap(val any) bool {
	if val == nil {
		return false
	}
	return reflect.TypeOf(val).Kind() == reflect.Map
}
func getMapValues(input any) ([]any, error) {
	// Use reflection to check that the input is a map.
	v := reflect.ValueOf(input)
	if v.Kind() != reflect.Map {
		return nil, fmt.Errorf("input is not a map")
	}

	// Create a new map for the result.
	result := make([]any, v.Len())

	// Iterate over all keys.
	for i, key := range v.MapKeys() {
		// Use the string key and the corresponding value.
		result[i] = v.MapIndex(key).Interface()
	}
	return result, nil
}
