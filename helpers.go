package mpath

import (
	"bytes"
	"fmt"
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

func readerContains(r io.Reader, substr io.Reader) (bool, error) {
	substrBytes, err := io.ReadAll(substr)
	if err != nil {
		return false, err
	}

	bufSize := 4096
	buf := make([]byte, bufSize+len(substrBytes)) // Keep extra space for substring match across buffers
	offset := 0

	for {
		n, err := r.Read(buf[offset:])
		if n > 0 {
			if strings.Contains(string(buf[:offset+n]), string(substrBytes)) {
				return true, nil
			}
			copy(buf, buf[n:offset+n]) // Shift buffer left
			offset = len(substrBytes) - 1
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

func readerHasSuffix(r io.Reader, suffix io.Reader) (bool, error) {
	suffixBytes, err := io.ReadAll(suffix)
	if err != nil {
		return false, fmt.Errorf("error reading suffix: %w", err)
	}

	suffixLen := len(suffixBytes)
	if suffixLen == 0 {
		return true, nil
	}

	buf := make([]byte, suffixLen)
	_, err = io.ReadFull(r, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, err
	}

	// Read the stream while maintaining a sliding buffer of last `suffixLen` bytes
	tmp := make([]byte, 1)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			// Shift buffer left and append new byte
			copy(buf, buf[1:])
			buf[len(buf)-1] = tmp[0]
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
	}

	// Final suffix check
	return bytes.HasSuffix(buf, suffixBytes), nil
}

func readerHasPrefix(r io.Reader, prefix io.Reader) (bool, error) {
	prefixBytes, err := io.ReadAll(prefix)
	if err != nil {
		return false, err
	}

	buf := make([]byte, len(prefixBytes))
	_, err = io.ReadFull(r, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, err
	}

	return strings.HasPrefix(string(buf), string(prefixBytes)), nil
}

func bufferReader(r io.Reader) (io.Reader, error) {
	var buf bytes.Buffer
	tee := io.TeeReader(r, &buf)
	data, err := io.ReadAll(tee) // Read the full stream and store it
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil // Return a new reader with stored data
}

func readAll(r io.ReadSeeker) (string, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
