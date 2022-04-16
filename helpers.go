package mpath

import (
	"reflect"
	"strings"

	"github.com/shopspring/decimal"
)

func repeatTabs(numTabs int) string {
	return strings.Repeat("\t", numTabs)
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	}
	return false
}

func convertToDecimalIfNumber(val any) (out any) {
	out = val
	v := reflect.ValueOf(val)

	if !isEmptyValue(v) {
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			v = v.Elem()
		}
	}

	if !isNumberKind(v.Kind()) {
		return
	}

	switch outType := out.(type) {
	case int:
		out = decimal.NewFromInt(int64(outType))
	case int8:
		out = decimal.NewFromInt(int64(outType))
	case int16:
		out = decimal.NewFromInt(int64(outType))
	case int32:
		out = decimal.NewFromInt(int64(outType))
	case int64:
		out = decimal.NewFromInt(int64(outType))
	case uint:
		out = decimal.NewFromInt(int64(outType))
	case uint8:
		out = decimal.NewFromInt(int64(outType))
	case uint16:
		out = decimal.NewFromInt(int64(outType))
	case uint32:
		out = decimal.NewFromInt(int64(outType))
	case uint64:
		out = decimal.NewFromInt(int64(outType))
	case float32:
		out = decimal.NewFromFloat(float64(outType))
	case float64:
		out = decimal.NewFromFloat(outType)
	}

	return out
}

func isNumberKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func getValuesByName(identName string, data any) (out any) {
	v := reflect.ValueOf(data)

	if !isEmptyValue(v) {
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			v = v.Elem()
		}

		switch v.Kind() {
		case reflect.Struct:
			out, _ = getFieldValueByNameFromStruct(identName, v)
			return
		case reflect.Array, reflect.Slice:
			if v.Len() == 0 {
				return nil
			}

			fev := v.Index(0)
			switch fev.Kind() {
			case reflect.Pointer, reflect.Interface:
				fev = fev.Elem()
			}

			if k := fev.Kind(); !(k == reflect.Struct || k == reflect.Map) {
				return nil
			}

			// if fev.Kind() == reflect.Map {
			// 	for _, e := range fev.MapKeys() {
			// 		if mks, ok := e.Interface().(string); !ok || strings.ToLower(mks) != strings.ToLower(identName) {
			// 			continue
			// 		}

			// 		slc = append(slc, convertToDecimalIfNumber(fev.MapIndex(e).Interface()))
			// 	}
			// 	return slc
			// }

			var slc []any
			var found bool
			for i := 0; i < v.Len(); i++ {
				if out, found = getFieldValueByNameFromStruct(identName, v.Index(i)); found {
					slc = append(slc, out)
				}
			}
			if len(slc) > 0 {
				return slc
			}
		}
	}

	return nil
}

func getAsStructOrSlice(data any) (out any, ok, wasStruct bool) {
	if m, ok := data.(map[string]any); ok {
		// this is the JSON version of a struct
		return m, true, true
	}

	v := reflect.ValueOf(data)

	// if !isEmptyValue(v) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		return v.Interface(), true, true
	case reflect.Array, reflect.Slice:
		if v.Len() == 0 {
			return []any{}, true, false
		}

		if slc, ok := v.Interface().([]any); ok {
			return slc, true, false
		}

		var slc []any
		for i := 0; i < v.Len(); i++ {
			slc = append(slc, v.Index(i).Interface())
		}
		return slc, true, false
	}
	// }

	return nil, false, false
}

func getFieldValueByNameFromStruct(identName string, structValue reflect.Value) (out any, found bool) {
	if isEmptyValue(structValue) {
		return nil, false
	}

	switch structValue.Kind() {
	case reflect.Pointer, reflect.Interface:
		structValue = structValue.Elem()
	}

	svk := structValue.Kind()

	if svk == reflect.Map {
		for _, e := range structValue.MapKeys() {
			if mks, ok := e.Interface().(string); !ok || strings.ToLower(mks) != strings.ToLower(identName) {
				continue
			}

			return convertToDecimalIfNumber(structValue.MapIndex(e).Interface()), true
		}
		return nil, false
	}

	if svk != reflect.Struct {
		return nil, false
	}

	st := structValue.Type()

	for fn := 0; fn < structValue.NumField(); fn++ {
		if strings.ToLower(st.Field(fn).Name) == strings.ToLower(identName) {
			out = structValue.Field(fn).Interface()

			switch outType := out.(type) {
			case float64:
				out = decimal.NewFromFloat(outType)
			case float32:
				out = decimal.NewFromFloat(float64(outType))
			case int:
				out = decimal.NewFromInt(int64(outType))
			case int8:
				out = decimal.NewFromInt(int64(outType))
			case int16:
				out = decimal.NewFromInt(int64(outType))
			case int32:
				out = decimal.NewFromInt(int64(outType))
			case int64:
				out = decimal.NewFromInt(int64(outType))
			case uint:
				out = decimal.NewFromInt(int64(outType))
			case uint8:
				out = decimal.NewFromInt(int64(outType))
			case uint16:
				out = decimal.NewFromInt(int64(outType))
			case uint32:
				out = decimal.NewFromInt(int64(outType))
			case uint64:
				out = decimal.NewFromInt(int64(outType))
			}

			return out, true
		}
	}

	return nil, false
}
