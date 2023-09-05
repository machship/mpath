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

func (op *opFunction) paramsGetFirstOfAny(rtParams runtimeParams) (val any, err error) {
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

func (op *opFunction) paramsGetFirstOfNumber(rtParams runtimeParams) (val decimal.Decimal, err error) {
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

func (op *opFunction) paramsGetFirstOfString(rtParams runtimeParams) (val string, err error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return val, fmt.Errorf("expected %d params, got %d", 1, got)
	}

	for _, p := range rtParams.paramsString {
		return p, nil
	}

	return val, fmt.Errorf("no string parameter found")
}

func (op *opFunction) paramsGetAll(rtParams runtimeParams) (val []any, err error) {
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

func (ft ft_FunctionType) errBool(err error) (bool, error) {
	return false, fmt.Errorf("func %s: %w", ft_GetName(ft), err)
}

func (ft ft_FunctionType) errString(err error) (string, error) {
	return "", fmt.Errorf("func %s: %w", ft_GetName(ft), err)
}

func (op *opFunction) func_Equal(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfAny(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
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

func (op *opFunction) func_NotEqual(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfAny(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
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

func (op *opFunction) func_Less(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
	}

	if valIfc, ok := val.(decimal.Decimal); ok {
		return valIfc.LessThan(param), nil
	}

	return false, fmt.Errorf("parameter wasn't number")
}

func (op *opFunction) func_LessOrEqual(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
	}

	if valIfc, ok := val.(decimal.Decimal); ok {
		return valIfc.LessThanOrEqual(param), nil
	}

	return false, fmt.Errorf("parameter wasn't number")
}

func (op *opFunction) func_Greater(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
	}

	if valIfc, ok := val.(decimal.Decimal); ok {
		return valIfc.GreaterThan(param), nil
	}

	return false, fmt.Errorf("parameter wasn't number")
}

func (op *opFunction) func_GreaterOrEqual(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
	}

	if valDec, ok := val.(decimal.Decimal); ok {
		return valDec.GreaterThanOrEqual(param), nil
	}

	return false, fmt.Errorf("parameter wasn't number")
}

func (op *opFunction) func_Contains(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfString(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
	}

	if valIfc, ok := val.(string); ok {
		return strings.Contains(valIfc, param), nil
	}

	return false, fmt.Errorf("parameter wasn't string")
}

func (op *opFunction) func_NotContains(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfString(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
	}

	if valIfc, ok := val.(string); ok {
		return !strings.Contains(valIfc, param), nil
	}

	return false, fmt.Errorf("parameter wasn't string")
}

func (op *opFunction) func_Prefix(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfString(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
	}

	if valIfc, ok := val.(string); ok {
		return strings.HasPrefix(valIfc, param), nil
	}

	return false, fmt.Errorf("parameter wasn't string")
}

func (op *opFunction) func_NotPrefix(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfString(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
	}

	if valIfc, ok := val.(string); ok {
		return !strings.HasPrefix(valIfc, param), nil
	}

	return false, fmt.Errorf("parameter wasn't string")
}

func (op *opFunction) func_Suffix(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfString(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
	}

	if valIfc, ok := val.(string); ok {
		return strings.HasSuffix(valIfc, param), nil
	}

	return false, fmt.Errorf("parameter wasn't string")
}

func (op *opFunction) func_NotSuffix(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfString(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
	}

	if valIfc, ok := val.(string); ok {
		return !strings.HasSuffix(valIfc, param), nil
	}

	return false, fmt.Errorf("parameter wasn't string")
}

func (op *opFunction) func_Count(rtParams runtimeParams, val any) (decimal.Decimal, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return decimal.Zero, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.functionType), 0, got)
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

func (op *opFunction) func_Any(rtParams runtimeParams, val any) (bool, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return false, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.functionType), 0, got)
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

func (op *opFunction) func_First(rtParams runtimeParams, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return 0, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.functionType), 0, got)
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

func (op *opFunction) func_Last(rtParams runtimeParams, val any) (any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return false, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.functionType), 0, got)
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

func (op *opFunction) func_Index(rtParams runtimeParams, val any) (any, error) {
	param, err := op.paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
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

func (op *opFunction) func_decimalSlice(rtParams runtimeParams, val any, decimalSliceFunction func(decimal.Decimal, ...decimal.Decimal) decimal.Decimal) (any, error) {
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

func (op *opFunction) func_Sum(rtParams runtimeParams, val any) (any, error) {
	return op.func_decimalSlice(rtParams, val, decimal.Sum)
}

func (op *opFunction) func_Avg(rtParams runtimeParams, val any) (any, error) {
	return op.func_decimalSlice(rtParams, val, decimal.Avg)
}

func (op *opFunction) func_Min(rtParams runtimeParams, val any) (any, error) {
	return op.func_decimalSlice(rtParams, val, decimal.Min)
}

func (op *opFunction) func_Max(rtParams runtimeParams, val any) (any, error) {
	return op.func_decimalSlice(rtParams, val, decimal.Max)
}

func (op *opFunction) func_decimal(rtParams runtimeParams, val any, decSlcFunc func(decimal.Decimal, decimal.Decimal) decimal.Decimal) (any, error) {
	param, err := op.paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
	}

	if valIfc, ok := val.(decimal.Decimal); ok {
		return decSlcFunc(valIfc, param), nil
	}

	return false, fmt.Errorf("not a number")
}

func (op *opFunction) func_Add(rtParams runtimeParams, val any) (any, error) {
	return op.func_decimal(rtParams, val, decimal.Decimal.Add)
}

func (op *opFunction) func_Sub(rtParams runtimeParams, val any) (any, error) {
	return op.func_decimal(rtParams, val, decimal.Decimal.Sub)
}

func (op *opFunction) func_Div(rtParams runtimeParams, val any) (any, error) {
	return op.func_decimal(rtParams, val, decimal.Decimal.Div)
}

func (op *opFunction) func_Mul(rtParams runtimeParams, val any) (any, error) {
	return op.func_decimal(rtParams, val, decimal.Decimal.Mul)
}

func (op *opFunction) func_Mod(rtParams runtimeParams, val any) (any, error) {
	return op.func_decimal(rtParams, val, decimal.Decimal.Mod)
}

func (op *opFunction) func_AnyOf(rtParams runtimeParams, val any) (bool, error) {
	params, err := op.paramsGetAll(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
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

func (op *opFunction) func_AsJSON(rtParams runtimeParams, val any) (string, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return "", fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.functionType), 0, got)
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

func (op *opFunction) func_TrimRightN(rtParams runtimeParams, val any) (string, error) {
	param, err := op.paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return op.functionType.errString(err)
	}

	if !param.IsInteger() {
		return "", fmt.Errorf("parameter must be an integer")
	}

	paramAsInt := int(param.IntPart())

	if valIfc, ok := val.(string); ok {
		if len(valIfc) <= paramAsInt {
			return "", nil
		}

		return valIfc[:len(valIfc)-paramAsInt], nil
	}

	return "", fmt.Errorf("value wasn't string")
}

func (op *opFunction) func_TrimLeftN(rtParams runtimeParams, val any) (string, error) {
	param, err := op.paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return op.functionType.errString(err)
	}

	if !param.IsInteger() {
		return "", fmt.Errorf("parameter must be an integer")
	}

	paramAsInt := int(param.IntPart())

	if valIfc, ok := val.(string); ok {
		if len(valIfc) <= paramAsInt {
			return "", nil
		}

		return valIfc[paramAsInt:], nil
	}

	return "", fmt.Errorf("value wasn't string")
}

func (op *opFunction) func_RightN(rtParams runtimeParams, val any) (string, error) {
	param, err := op.paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return op.functionType.errString(err)
	}

	if !param.IsInteger() {
		return "", fmt.Errorf("parameter must be an integer")
	}

	paramAsInt := int(param.IntPart())

	if valIfc, ok := val.(string); ok {
		if len(valIfc) < paramAsInt {
			return valIfc, nil
		}

		return valIfc[len(valIfc)-paramAsInt:], nil
	}

	return "", fmt.Errorf("value wasn't string")
}

func (op *opFunction) func_LeftN(rtParams runtimeParams, val any) (string, error) {
	param, err := op.paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return op.functionType.errString(err)
	}

	if !param.IsInteger() {
		return "", fmt.Errorf("parameter must be an integer")
	}

	paramAsInt := int(param.IntPart())

	if valIfc, ok := val.(string); ok {
		if len(valIfc) < paramAsInt {
			return valIfc, nil
		}

		return valIfc[:paramAsInt], nil
	}

	return "", fmt.Errorf("value wasn't string")
}

func (op *opFunction) func_DoesMatchRegex(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfString(rtParams)
	if err != nil {
		return op.functionType.errBool(err)
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

func (op *opFunction) func_ReplaceRegex(rtParams runtimeParams, val any) (string, error) {
	if got, ok := rtParams.checkLengthOfParams(2); !ok {
		return "", fmt.Errorf("expected %d params, got %d", 1, got)
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

func (op *opFunction) func_ReplaceAll(rtParams runtimeParams, val any) (string, error) {
	if got, ok := rtParams.checkLengthOfParams(2); !ok {
		return "", fmt.Errorf("expected %d params, got %d", 1, got)
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

func (op *opFunction) func_ParseJSON(rtParams runtimeParams, val any) (map[string]any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return nil, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.functionType), 0, got)
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
			nm := map[string]any{}
			err := json.Unmarshal([]byte(cdString), &nm)
			if err != nil {
				return nil, fmt.Errorf("value is not JSON: %w", err)
			}

			return nm, nil
		}
	}

	return nil, fmt.Errorf("value is not a string")
}

func (op *opFunction) func_ParseXML(rtParams runtimeParams, val any) (map[string]any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return nil, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.functionType), 0, got)
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
			xml := strings.NewReader(cdString)
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
		}
	}

	return nil, fmt.Errorf("value is not a string")
}

func (op *opFunction) func_ParseYAML(rtParams runtimeParams, val any) (map[string]any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return nil, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.functionType), 0, got)
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
			nm := map[string]any{}
			err := yaml.Unmarshal([]byte(cdString), &nm)
			if err != nil {
				return nil, fmt.Errorf("value is not YAML: %w", err)
			}

			return nm, nil
		}
	}

	return nil, fmt.Errorf("value is not a string")
}

func (op *opFunction) func_ParseTOML(rtParams runtimeParams, val any) (map[string]any, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return nil, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.functionType), 0, got)
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
			nm := map[string]any{}
			err := toml.Unmarshal([]byte(cdString), &nm)
			if err != nil {
				return nil, fmt.Errorf("value is not TOML: %w", err)
			}

			return nm, nil
		}
	}

	return nil, fmt.Errorf("value is not a string")
}

func (op *opFunction) func_RemoveKeysByRegex(rtParams runtimeParams, val any) (map[string]any, error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return nil, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.functionType), 1, got)
	}

	param, err := op.paramsGetFirstOfString(rtParams)
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

func (op *opFunction) func_RemoveKeysByPrefix(rtParams runtimeParams, val any) (map[string]any, error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return nil, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.functionType), 1, got)
	}

	prefixParam, err := op.paramsGetFirstOfString(rtParams)
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

func (op *opFunction) func_RemoveKeysBySuffix(rtParams runtimeParams, val any) (map[string]any, error) {
	if got, ok := rtParams.checkLengthOfParams(1); !ok {
		return nil, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.functionType), 1, got)
	}

	prefixParam, err := op.paramsGetFirstOfString(rtParams)
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

type ft_FunctionType int

const (
	ft_NotSet ft_FunctionType = iota
	ft_Equal
	ft_NotEqual
	ft_Less
	ft_LessOrEqual
	ft_Greater
	ft_GreaterOrEqual
	ft_Contains
	ft_NotContains
	ft_Prefix
	ft_NotPrefix
	ft_Suffix
	ft_NotSuffix
	ft_Any
	ft_AnyOf

	ft_Count
	ft_First
	ft_Last
	ft_Index
	ft_Sum
	ft_Avg
	ft_Max
	ft_Min
	ft_Add
	ft_Sub
	ft_Div
	ft_Mul
	ft_Mod

	ft_TrimRightN
	ft_TrimLeftN
	ft_RightN
	ft_LeftN
	ft_DoesMatchRegex
	ft_ReplaceRegex
	ft_ReplaceAll

	ft_AsJSON
	ft_ParseJSON
	ft_ParseXML
	ft_ParseYAML
	ft_ParseTOML
	ft_RemoveKeysByRegex
	ft_RemoveKeysByPrefix
	ft_RemoveKeysBySuffix
)

func ft_GetByName(name string) (ft ft_FunctionType, err error) {
	switch name {
	case "Equal":
		ft = ft_Equal
	case "NotEqual":
		ft = ft_NotEqual
	case "Less":
		ft = ft_Less
	case "LessOrEqual":
		ft = ft_LessOrEqual
	case "Greater":
		ft = ft_Greater
	case "GreaterOrEqual":
		ft = ft_GreaterOrEqual
	case "Contains":
		ft = ft_Contains
	case "NotContains":
		ft = ft_NotContains
	case "Prefix":
		ft = ft_Prefix
	case "NotPrefix":
		ft = ft_NotPrefix
	case "Suffix":
		ft = ft_Suffix
	case "NotSuffix":
		ft = ft_NotSuffix
	case "Count":
		ft = ft_Count
	case "First":
		ft = ft_First
	case "Last":
		ft = ft_Last
	case "Index":
		ft = ft_Index
	case "Any":
		ft = ft_Any
	case "Sum":
		ft = ft_Sum
	case "Avg":
		ft = ft_Avg
	case "Max":
		ft = ft_Max
	case "Min":
		ft = ft_Min
	case "Add":
		ft = ft_Add
	case "Sub":
		ft = ft_Sub
	case "Div":
		ft = ft_Div
	case "Mul":
		ft = ft_Mul
	case "Mod":
		ft = ft_Mod
	case "AnyOf":
		ft = ft_AnyOf

	case "TrimRightN":
		ft = ft_TrimRightN
	case "TrimLeftN":
		ft = ft_TrimLeftN
	case "RightN":
		ft = ft_RightN
	case "LeftN":
		ft = ft_LeftN
	case "DoesMatchRegex":
		ft = ft_DoesMatchRegex
	case "ReplaceRegex":
		ft = ft_ReplaceRegex
	case "ReplaceAll":
		ft = ft_ReplaceAll

	case "AsJSON":
		ft = ft_AsJSON
	case "ParseJSON":
		ft = ft_ParseJSON
	case "ParseXML":
		ft = ft_ParseXML
	case "ParseYAML":
		ft = ft_ParseYAML
	case "ParseTOML":
		ft = ft_ParseTOML

	case "RemoveKeysByRegex":
		ft = ft_RemoveKeysByRegex
	case "RemoveKeysByPrefix":
		ft = ft_RemoveKeysByPrefix
	case "RemoveKeysBySuffix":
		ft = ft_RemoveKeysBySuffix

	default:
		return 0, fmt.Errorf("unknown function name '%s'", name)
	}

	return
}

func ft_GetName(ft ft_FunctionType) (name string) {
	switch ft {
	case ft_Equal:
		name = "Equal"
	case ft_NotEqual:
		name = "NotEqual"
	case ft_Less:
		name = "Less"
	case ft_LessOrEqual:
		name = "LessOrEqual"
	case ft_Greater:
		name = "Greater"
	case ft_GreaterOrEqual:
		name = "GreaterOrEqual"
	case ft_Contains:
		name = "Contains"
	case ft_NotContains:
		name = "NotContains"
	case ft_Prefix:
		name = "Prefix"
	case ft_NotPrefix:
		name = "NotPrefix"
	case ft_Suffix:
		name = "Suffix"
	case ft_NotSuffix:
		name = "NotSuffix"
	case ft_Count:
		name = "Count"
	case ft_First:
		name = "First"
	case ft_Last:
		name = "Last"
	case ft_Index:
		name = "Index"
	case ft_Any:
		name = "Any"
	case ft_Sum:
		name = "Sum"
	case ft_Avg:
		name = "Avg"
	case ft_Max:
		name = "Max"
	case ft_Min:
		name = "Min"
	case ft_Add:
		name = "Add"
	case ft_Sub:
		name = "Sub"
	case ft_Div:
		name = "Div"
	case ft_Mul:
		name = "Mul"
	case ft_Mod:
		name = "Mod"
	case ft_AnyOf:
		name = "AnyOf"

	case ft_TrimRightN:
		name = "TrimRightN"
	case ft_TrimLeftN:
		name = "TrimLeftN"
	case ft_RightN:
		name = "RightN"
	case ft_LeftN:
		name = "LeftN"
	case ft_DoesMatchRegex:
		name = "DoesMatchRegex"
	case ft_ReplaceRegex:
		name = "ReplaceRegex"
	case ft_ReplaceAll:
		name = "ReplaceAll"

	case ft_ParseJSON:
		name = "ParseJSON"
	case ft_ParseXML:
		name = "ParseXML"
	case ft_ParseYAML:
		name = "ParseYAML"
	case ft_ParseTOML:
		name = "ParseTOML"
	case ft_RemoveKeysByRegex:
		name = "RemoveKeysByRegex"
	case ft_RemoveKeysByPrefix:
		name = "RemoveKeysByPrefix"
	case ft_RemoveKeysBySuffix:
		name = "RemoveKeysBySuffix"
	}

	return
}

func ft_ShouldContinueForPath(ft ft_FunctionType) bool {
	switch ft {
	case ft_First, ft_Last, ft_Index:
		return true
	}

	return false
}

func ft_IsBoolFunc(ft ft_FunctionType) bool {
	switch ft {
	case ft_Equal,
		ft_NotEqual,
		ft_Less,
		ft_LessOrEqual,
		ft_Greater,
		ft_GreaterOrEqual,
		ft_Contains,
		ft_NotContains,
		ft_Prefix,
		ft_NotPrefix,
		ft_Suffix,
		ft_NotSuffix,
		ft_AnyOf,
		ft_Any:
		return true
	}

	return false
}
