package mpath

import (
	"io"
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

func convertToDecimalIfNumberAndCheck(val any) (wasNumber bool, out decimal.Decimal) {
	v := reflect.ValueOf(val)

	if !isEmptyValue(v) {
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			v = v.Elem()
		}
	}

	if !(isNumberKind(v.Kind()) || v.Kind() == reflect.String) {
		return
	}

	switch outType := val.(type) {
	case string:
		var err error
		out, err = decimal.NewFromString(outType)
		if err != nil {
			return false, decimal.Zero
		}
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

	wasNumber = true

	return
}

func convertToDecimalIfNumber(val any) (out any) {
	if wasNumber, number := convertToDecimalIfNumberAndCheck(val); wasNumber {
		return number
	}

	return val
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

func getValuesByName(identName string, data any) (out any, err error) {
	v := reflect.ValueOf(data)

	if !isEmptyValue(v) {
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			v = v.Elem()
		}

		switch v.Kind() {
		case reflect.Struct:
			var wasFound bool
			out, wasFound = getFieldValueByNameFromStruct(identName, v)
			if wasFound {
				return
			}

			return nil, ErrKeyNotFound
		case reflect.Array, reflect.Slice:
			if v.Len() == 0 {
				return nil, ErrKeyNotFound
			}

			fev := v.Index(0)
			switch fev.Kind() {
			case reflect.Pointer, reflect.Interface:
				fev = fev.Elem()
			}

			if k := fev.Kind(); !(k == reflect.Struct || k == reflect.Map) {
				return nil, ErrKeyNotFound
			}

			var slc []any
			var found bool
			for i := 0; i < v.Len(); i++ {
				if out, found = getFieldValueByNameFromStruct(identName, v.Index(i)); found {
					slc = append(slc, out)
				}
			}
			if len(slc) > 0 {
				return slc, nil
			}
		}
	}

	return nil, ErrKeyNotFound
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
			mks, ok := e.Interface().(string)
			if !ok {
				if reflect.TypeOf(e.Interface()).ConvertibleTo(reflect.TypeOf("")) {
					mksTemp := reflect.ValueOf(e.Interface()).Convert(reflect.TypeOf("")).Interface()
					mks, ok = mksTemp.(string)
					if !ok || mks == "" {
						continue
					}
				} else {
					continue
				}
			}

			if !strings.EqualFold(mks, identName) {
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
		structFieldName := st.Field(fn).Name
		if strings.EqualFold(structFieldName, identName) {
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

func doForMapPerKey(valueThatShouldBeMap any, doFunc func(keyAsString string, keyAsValue, mapAsValue reflect.Value)) {
	v := reflect.ValueOf(valueThatShouldBeMap)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		v = v.Elem()
	}

	if v.Kind() == reflect.Map {
		for _, e := range v.MapKeys() {
			mks, ok := e.Interface().(string)
			if !ok {
				if reflect.TypeOf(e.Interface()).ConvertibleTo(reflect.TypeOf("")) {
					mksTemp := reflect.ValueOf(e.Interface()).Convert(reflect.TypeOf("")).Interface()
					mks, ok = mksTemp.(string)
					if !ok || mks == "" {
						continue
					}
				} else {
					continue
				}
			}

			doFunc(mks, e, v)
		}
	}
}

func readerContains(r io.Reader, substr string) (bool, error) {
	bufSize := 4096
	substrLen := len(substr)
	buf := make([]byte, bufSize+substrLen-1) // Ensure space to handle boundaries

	offset := 0

	for {
		n, err := r.Read(buf[offset:]) // Read into available space in the buffer
		if n > 0 {
			data := string(buf[:offset+n])
			if strings.Contains(data, substr) {
				return true, nil
			}

			// Preserve only the last (substrLen - 1) bytes in the buffer for the next read
			if offset+n >= substrLen-1 {
				copy(buf, buf[offset+n-substrLen+1:offset+n])
			}
			offset = substrLen - 1
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
	}

	return false, nil
}

func readerHasSuffix(r io.Reader, suffix string) (bool, error) {
	if suffix == "" {
		return true, nil // Empty suffix should always match
	}

	suffixLen := len(suffix)
	buf := make([]byte, 0, suffixLen) // Buffer to store last suffixLen bytes

	tmp := make([]byte, 1) // Read one byte at a time
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			if len(buf) < suffixLen {
				buf = append(buf, tmp[0]) // Build up buffer
			} else {
				copy(buf, buf[1:])        // Shift left
				buf[suffixLen-1] = tmp[0] // Append new byte
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
	}

	// Check if the final buffer matches suffix
	return string(buf) == suffix, nil
}

func readerHasPrefix(r io.Reader, prefix string) (bool, error) {
	if prefix == "" {
		return true, nil // Empty prefix should always match
	}

	buf := make([]byte, len(prefix))
	n, err := io.ReadFull(r, buf)

	if err == io.EOF {
		return false, nil // Input is empty, cannot have a prefix
	}
	if err != nil && err != io.ErrUnexpectedEOF {
		return false, err
	}

	return string(buf[:n]) == prefix, nil
}
