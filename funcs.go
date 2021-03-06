package mpath

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/shopspring/decimal"
)

func (op *opFunction) paramsGetFirstOfAny(rtParams runtimeParams) (val any, err error) {
	if got, ok := rtParams.checkNumParams(1); !ok {
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
	if got, ok := rtParams.checkNumParams(1); !ok {
		return val, fmt.Errorf("expected %d params, got %d", 1, got)
	}

	for _, p := range rtParams.paramsNumber {
		return p, nil
	}

	return val, fmt.Errorf("no number parameter found")
}

func (op *opFunction) paramsGetFirstOfString(rtParams runtimeParams) (val string, err error) {
	if got, ok := rtParams.checkNumParams(1); !ok {
		return val, fmt.Errorf("expected %d params, got %d", 1, got)
	}

	for _, p := range rtParams.paramsString {
		return p, nil
	}

	return val, fmt.Errorf("no stromg parameter found")
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

func (rtParams *runtimeParams) checkNumParams(allowed int) (got int, ok bool) {
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
	if got, ok := rtParams.checkNumParams(0); !ok {
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
	if got, ok := rtParams.checkNumParams(0); !ok {
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
	if got, ok := rtParams.checkNumParams(0); !ok {
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
	if got, ok := rtParams.checkNumParams(0); !ok {
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

func (op *opFunction) func_decimalSlice(rtParams runtimeParams, val any, decSlcFunc func(decimal.Decimal, ...decimal.Decimal) decimal.Decimal) (any, error) {
	if vd, ok := val.(decimal.Decimal); ok {
		val = []decimal.Decimal{vd}
	}

	if valIfc, ok := val.([]decimal.Decimal); ok {
		newSlc := append([]decimal.Decimal{}, valIfc...)
		newSlc = append(newSlc, rtParams.paramsNumber...)

		if len(newSlc) == 0 {
			return decimal.Zero, nil
		}

		if len(newSlc) == 1 {
			return newSlc[0], nil
		}

		return decSlcFunc(newSlc[0], newSlc[1:]...), nil
	}

	if valIfc, ok := val.([]any); ok {
		newSlc := append([]decimal.Decimal{}, rtParams.paramsNumber...)
		for _, vs := range valIfc {
			if vd, ok := vs.(decimal.Decimal); ok {
				newSlc = append(newSlc, vd)
			} else {
				goto notArrayOfNumbers
			}
		}

		if len(newSlc) == 0 {
			return decimal.Zero, nil
		}

		if len(newSlc) == 1 {
			return newSlc[0], nil
		}

		return decSlcFunc(newSlc[0], newSlc[1:]...), nil
	}

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
