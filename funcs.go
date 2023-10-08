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
	PT_NumberOrArrayOfNumbers PT_ParameterType = "ArrayOfNumbers"
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
}

type ParameterDescriptor struct {
	Name string
	Type PT_ParameterType
}

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

func (ft FT_FunctionType) errBool(err error) (bool, error) {
	return false, fmt.Errorf("func %s: %w", ft_GetName(ft), err)
}

func (ft FT_FunctionType) errString(err error) (string, error) {
	return "", fmt.Errorf("func %s: %w", ft_GetName(ft), err)
}

func (op *opFunction) func_Equal(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfAny(rtParams)
	if err != nil {
		return op.FunctionType.errBool(err)
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
		return op.FunctionType.errBool(err)
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
		return op.FunctionType.errBool(err)
	}

	if valIfc, ok := val.(decimal.Decimal); ok {
		return valIfc.LessThan(param), nil
	}

	return false, fmt.Errorf("parameter wasn't number")
}

func (op *opFunction) func_LessOrEqual(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return op.FunctionType.errBool(err)
	}

	if valIfc, ok := val.(decimal.Decimal); ok {
		return valIfc.LessThanOrEqual(param), nil
	}

	return false, fmt.Errorf("parameter wasn't number")
}

func (op *opFunction) func_Greater(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return op.FunctionType.errBool(err)
	}

	if valIfc, ok := val.(decimal.Decimal); ok {
		return valIfc.GreaterThan(param), nil
	}

	return false, fmt.Errorf("parameter wasn't number")
}

func (op *opFunction) func_GreaterOrEqual(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfNumber(rtParams)
	if err != nil {
		return op.FunctionType.errBool(err)
	}

	if valDec, ok := val.(decimal.Decimal); ok {
		return valDec.GreaterThanOrEqual(param), nil
	}

	return false, fmt.Errorf("parameter wasn't number")
}

func (op *opFunction) func_Contains(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfString(rtParams)
	if err != nil {
		return op.FunctionType.errBool(err)
	}

	if valIfc, ok := val.(string); ok {
		return strings.Contains(valIfc, param), nil
	}

	return false, fmt.Errorf("parameter wasn't string")
}

func (op *opFunction) func_NotContains(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfString(rtParams)
	if err != nil {
		return op.FunctionType.errBool(err)
	}

	if valIfc, ok := val.(string); ok {
		return !strings.Contains(valIfc, param), nil
	}

	return false, fmt.Errorf("parameter wasn't string")
}

func (op *opFunction) func_Prefix(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfString(rtParams)
	if err != nil {
		return op.FunctionType.errBool(err)
	}

	if valIfc, ok := val.(string); ok {
		return strings.HasPrefix(valIfc, param), nil
	}

	return false, fmt.Errorf("parameter wasn't string")
}

func (op *opFunction) func_NotPrefix(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfString(rtParams)
	if err != nil {
		return op.FunctionType.errBool(err)
	}

	if valIfc, ok := val.(string); ok {
		return !strings.HasPrefix(valIfc, param), nil
	}

	return false, fmt.Errorf("parameter wasn't string")
}

func (op *opFunction) func_Suffix(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfString(rtParams)
	if err != nil {
		return op.FunctionType.errBool(err)
	}

	if valIfc, ok := val.(string); ok {
		return strings.HasSuffix(valIfc, param), nil
	}

	return false, fmt.Errorf("parameter wasn't string")
}

func (op *opFunction) func_NotSuffix(rtParams runtimeParams, val any) (bool, error) {
	param, err := op.paramsGetFirstOfString(rtParams)
	if err != nil {
		return op.FunctionType.errBool(err)
	}

	if valIfc, ok := val.(string); ok {
		return !strings.HasSuffix(valIfc, param), nil
	}

	return false, fmt.Errorf("parameter wasn't string")
}

func (op *opFunction) func_Count(rtParams runtimeParams, val any) (decimal.Decimal, error) {
	if got, ok := rtParams.checkLengthOfParams(0); !ok {
		return decimal.Zero, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.FunctionType), 0, got)
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
		return false, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.FunctionType), 0, got)
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
		return 0, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.FunctionType), 0, got)
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
		return false, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.FunctionType), 0, got)
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
		return op.FunctionType.errBool(err)
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
		return op.FunctionType.errBool(err)
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
		return op.FunctionType.errBool(err)
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
		return "", fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.FunctionType), 0, got)
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
		return op.FunctionType.errString(err)
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
		return op.FunctionType.errString(err)
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
		return op.FunctionType.errString(err)
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
		return op.FunctionType.errString(err)
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
		return op.FunctionType.errBool(err)
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
		return nil, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.FunctionType), 0, got)
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
		return nil, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.FunctionType), 0, got)
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
		return nil, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.FunctionType), 0, got)
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
		return nil, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.FunctionType), 0, got)
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
		return nil, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.FunctionType), 1, got)
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
		return nil, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.FunctionType), 1, got)
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
		return nil, fmt.Errorf("(%s) expected %d params, got %d", ft_GetName(op.FunctionType), 1, got)
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
		},
		FT_NotEqual: {
			Name:        "NotEqual",
			Description: "Checks whether the value does not equal the parameter",
			Params:      singleParam("value to match", PT_Any),
			ValidOn:     PT_Any,
		},
		FT_Less: {
			Name:        "Less",
			Description: "Checks whether the value is less than the parameter",
			Params:      singleParam("number to compare", PT_Number),
			ValidOn:     PT_Number,
		},
		FT_LessOrEqual: {
			Name:        "LessOrEqual",
			Description: "Checks whether the value is less than or equal to the parameter",
			Params:      singleParam("number to compare", PT_Number),
			ValidOn:     PT_Number,
		},
		FT_Greater: {
			Name:        "Greater",
			Description: "Checks whether the value is greater than the parameter",
			Params:      singleParam("number to compare", PT_Number),
			ValidOn:     PT_Number,
		},
		FT_GreaterOrEqual: {
			Name:        "GreaterOrEqual",
			Description: "Checks whether the value is greater than or equal to the parameter",
			Params:      singleParam("number to compare", PT_Number),
			ValidOn:     PT_Number,
		},
		FT_Contains: {
			Name:        "Contains",
			Description: "Checks whether the value contains the parameter",
			Params:      singleParam("string to match", PT_String),
			ValidOn:     PT_String,
		},
		FT_NotContains: {
			Name:        "NotContains",
			Description: "Checks whether the value does not contain the parameter",
			Params:      singleParam("string to match", PT_String),
			ValidOn:     PT_String,
		},
		FT_Prefix: {
			Name:        "Prefix",
			Description: "Checks whether the value has the parameter as a prefix",
			Params:      singleParam("prefix to match", PT_String),
			ValidOn:     PT_String,
		},
		FT_NotPrefix: {
			Name:        "NotPrefix",
			Description: "Checks whether the value does not have the parameter as a prefix",
			Params:      singleParam("prefix to match", PT_String),
			ValidOn:     PT_String,
		},
		FT_Suffix: {
			Name:        "Suffix",
			Description: "Checks whether the value has the parameter as a suffix",
			Params:      singleParam("suffix to match", PT_String),
			ValidOn:     PT_String,
		},
		FT_NotSuffix: {
			Name:        "NotSuffix",
			Description: "Checks whether the value does not have the parameter as a suffix",
			Params:      singleParam("suffix to match", PT_String),
			ValidOn:     PT_String,
		},
		FT_Count: {
			Name:        "Count",
			Description: "Returns the count of elements in the array",
			Params:      nil,
			ValidOn:     PT_Array,
		},
		FT_First: {
			Name:        "First",
			Description: "Returns the first element of the array",
			Params:      nil,
			ValidOn:     PT_Array,
		},
		FT_Last: {
			Name:        "Last",
			Description: "Returns the last element of the array",
			Params:      nil,
			ValidOn:     PT_Array,
		},
		FT_Index: {
			Name:        "Index",
			Description: "Returns the element at the zero based index of the array",
			Params:      singleParam("index", PT_Number),
			ValidOn:     PT_Array,
		},
		FT_Any: {
			Name:        "Any",
			Description: "Checks whether there are any elements in the array",
			Params:      nil,
			ValidOn:     PT_Array,
		},
		FT_Sum: {
			Name:        "Sum",
			Description: "Sums the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers (not required)", PT_ArrayOfNumbers),
			ValidOn:     PT_NumberOrArrayOfNumbers,
		},
		FT_Avg: {
			Name:        "Avg",
			Description: "Averages the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers (not required)", PT_ArrayOfNumbers),
			ValidOn:     PT_NumberOrArrayOfNumbers,
		},
		FT_Max: {
			Name:        "Max",
			Description: "Returns the maximum of the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers (not required)", PT_ArrayOfNumbers),
			ValidOn:     PT_NumberOrArrayOfNumbers,
		},
		FT_Min: {
			Name:        "Min",
			Description: "Returns the minimum of the value along with any extra numbers in the parameters",
			Params:      singleParam("extra numbers (not required)", PT_ArrayOfNumbers),
			ValidOn:     PT_NumberOrArrayOfNumbers,
		},
		FT_Add: {
			Name:        "Add",
			Description: "Adds the parameter to the value",
			Params:      singleParam("number to add", PT_Number),
			ValidOn:     PT_Number,
		},
		FT_Sub: {
			Name:        "Sub",
			Description: "Subtracts the parameter from the value",
			Params:      singleParam("number to subtract", PT_Number),
			ValidOn:     PT_Number,
		},
		FT_Div: {
			Name:        "Div",
			Description: "Divides the value by the parameter",
			Params:      singleParam("number to divide by", PT_Number),
			ValidOn:     PT_Number,
		},
		FT_Mul: {
			Name:        "Mul",
			Description: "Multiplies the value by the parameter",
			Params:      singleParam("number to multiply by", PT_Number),
			ValidOn:     PT_Number,
		},
		FT_Mod: {
			Name:        "Mod",
			Description: "Returns the remainder of the value after dividing the value by the parameter",
			Params:      singleParam("number to modulo by", PT_Number),
			ValidOn:     PT_Number,
		},
		FT_AnyOf: {
			Name:        "AnyOf",
			Description: "Checks whether the value matches any of the parameters",
			Params:      singleParam("the values to match against", PT_Array),
			ValidOn:     PT_Any,
		},
		FT_TrimRightN: {
			Name:        "TrimRightN",
			Description: "Removes the 'n' most characters of the value from the right, where 'n' is the parameter",
			Params:      singleParam("number of characters", PT_Number),
			ValidOn:     PT_String,
		},
		FT_TrimLeftN: {
			Name:        "TrimLeftN",
			Description: "Removes the 'n' most characters of the value from the left, where 'n' is the parameter",
			Params:      singleParam("number of characters", PT_Number),
			ValidOn:     PT_String,
		},
		FT_RightN: {
			Name:        "RightN",
			Description: "Returns the 'n' most characters of the value from the right, where 'n' is the parameter'",
			Params:      singleParam("number of characters", PT_Number),
			ValidOn:     PT_String,
		},
		FT_LeftN: {
			Name:        "LeftN",
			Description: "Returns the 'n' most characters of the value from the left, where 'n' is the parameter",
			Params:      singleParam("number of characters", PT_Number),
			ValidOn:     PT_String,
		},
		FT_DoesMatchRegex: {
			Name:        "DoesMatchRegex",
			Description: "Checks whether the value matches the regular expression in the parameter",
			Params:      singleParam("regular expression to match", PT_String),
			ValidOn:     PT_String,
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
		},
		FT_AsJSON: {
			Name:        "AsJSON",
			Description: "Returns the value represented as JSON",
			Params:      nil,
			ValidOn:     PT_Any,
		},
		FT_ParseJSON: {
			Name:        "ParseJSON",
			Description: "Parses the value as JSON and returns an object or array",
			Params:      nil,
			ValidOn:     PT_String,
		},
		FT_ParseXML: {
			Name:        "ParseXML",
			Description: "Parses the value as XML and returns an object or array",
			Params:      nil,
			ValidOn:     PT_String,
		},
		FT_ParseYAML: {
			Name:        "ParseYAML",
			Description: "Parses the value as YAML and returns an object or array",
			Params:      nil,
			ValidOn:     PT_String,
		},
		FT_ParseTOML: {
			Name:        "ParseTOML",
			Description: "Parses the value as TOML and returns an object or array",
			Params:      nil,
			ValidOn:     PT_String,
		},
		FT_RemoveKeysByRegex: {
			Name:        "RemoveKeysByRegex",
			Description: "Removes any keys that match the regular expression in the parameter",
			Params:      singleParam("regular expression to match", PT_String),
			ValidOn:     PT_MapStringOfAny,
		},
		FT_RemoveKeysByPrefix: {
			Name:        "RemoveKeysByPrefix",
			Description: "Removes any keys that have a prefix as defined by the parameter",
			Params:      singleParam("prefix to match", PT_String),
			ValidOn:     PT_MapStringOfAny,
		},
		FT_RemoveKeysBySuffix: {
			Name:        "RemoveKeysBySuffix",
			Description: "Removes any keys that have a suffix as defined by the parameter",
			Params:      singleParam("suffix to match", PT_String),
			ValidOn:     PT_MapStringOfAny,
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
