package mpath

import (
	"bufio"
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
	// First, read the substring pattern - we need this to know what to search for
	// This is unavoidable since we need to know the complete pattern to search
	var pattern []byte
	subBuf := make([]byte, 1024)

	for {
		n, err := substr.Read(subBuf)
		if n > 0 {
			pattern = append(pattern, subBuf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
	}

	// Empty substring always matches
	if len(pattern) == 0 {
		return true, nil
	}

	buf := bufio.NewReader(r)
	bufSize := 4096
	overlap := make([]byte, 0, len(pattern)-1)

	for {
		chunk := make([]byte, bufSize)
		n, err := buf.Read(chunk)
		if n > 0 {
			data := append(overlap, chunk[:n]...)
			if bytes.Contains(data, pattern) {
				return true, nil
			}

			// Keep last `len(pattern)-1` bytes as overlap
			if len(data) >= len(pattern)-1 {
				overlap = data[len(data)-(len(pattern)-1):]
			} else {
				overlap = data
			}
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
	// Read the suffix pattern in chunks
	var pattern []byte
	suffixBuf := make([]byte, 1024)

	for {
		n, err := suffix.Read(suffixBuf)
		if n > 0 {
			pattern = append(pattern, suffixBuf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
	}

	// Empty suffix always matches
	if len(pattern) == 0 {
		return true, nil
	}

	// Keep only the most recent bytes equal to the pattern length
	buf := bufio.NewReader(r)
	bufSize := 4096
	queue := make([]byte, 0, len(pattern))

	for {
		chunk := make([]byte, bufSize)
		n, err := buf.Read(chunk)
		if n > 0 {
			queue = append(queue, chunk[:n]...)
			if len(queue) > len(pattern) {
				queue = queue[len(queue)-len(pattern):] // Keep only last `pattern` bytes
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
	}

	return bytes.Equal(queue, pattern), nil
}

func readerHasPrefix(r io.Reader, prefix io.Reader) (bool, error) {
	// Read the prefix pattern in chunks
	var pattern []byte
	prefixBuf := make([]byte, 1024)

	for {
		n, err := prefix.Read(prefixBuf)
		if n > 0 {
			pattern = append(pattern, prefixBuf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
	}

	// Empty prefix always matches
	if len(pattern) == 0 {
		return true, nil
	}

	// Read exactly the same number of bytes from the reader as the pattern length
	buf := bufio.NewReader(r)
	readBytes := make([]byte, len(pattern))
	n, err := io.ReadFull(buf, readBytes)

	// Handle errors
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, err
	}

	// If we couldn't read enough bytes, reader doesn't have this prefix
	if n < len(pattern) {
		return false, nil
	}

	// Check if what we read equals the pattern
	return bytes.Equal(readBytes, pattern), nil
}

// This method performs the replacement while streaming, ensuring minimal memory usage
func streamingReplaceAll(r io.ReadSeeker, find, replace string) (string, error) {
	var result strings.Builder
	buf := bufio.NewReader(r)
	findLen := len(find)
	window := make([]byte, 0, findLen)

	// Ensure we start reading from the beginning
	_, err := r.Seek(0, io.SeekStart)
	if err != nil {
		return "", fmt.Errorf("error resetting reader: %w", err)
	}

	for {
		b, err := buf.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("error reading from input: %w", err)
		}

		window = append(window, b)

		// Maintain a rolling window
		if len(window) > findLen {
			// Write first byte in window and shift
			result.WriteByte(window[0])
			window = window[1:]
		}

		// Match found → Replace
		if len(window) == findLen && string(window) == find {
			result.WriteString(replace) // Write replacement
			window = window[:0]         // Reset window
		}
	}

	// Flush remaining bytes
	if len(window) > 0 {
		result.Write(window)
	}

	return result.String(), nil
}

func trimRight(r io.Reader, n int) (string, error) {
	if n == 0 { // Nothing to trim
		var result bytes.Buffer
		_, err := io.Copy(&result, r)
		if err != nil {
			return "", err
		}
		return result.String(), nil
	}

	buf := bufio.NewReader(r)
	window := make([]byte, 0, n) // Sliding window for last `n` bytes
	var result bytes.Buffer
	count := 0

	for {
		b, err := buf.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if count >= n {
			result.WriteByte(window[0]) // Only write out bytes before last `n`
			window = window[1:]         // Shift window left
		}

		window = append(window, b)
		count++
	}

	// If input size < n, return empty string
	if count < n {
		return "", nil
	}

	return result.String(), nil
}

func trimLeft(r io.Reader, n int) (string, error) {
	buf := bufio.NewReader(r)

	// Skip the first `n` bytes
	for i := 0; i < n; i++ {
		_, err := buf.ReadByte()
		if err == io.EOF {
			return "", nil // If we reach EOF, return empty string
		}
		if err != nil {
			return "", err
		}
	}

	var result bytes.Buffer
	_, err := io.Copy(&result, buf)
	if err != nil {
		return "", err
	}

	return result.String(), nil
}

func right(r io.Reader, n int) (string, error) {
	buf := bufio.NewReader(r)
	window := make([]byte, 0, n) // Dynamic sliding window
	count := 0

	for {
		b, err := buf.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if count < n {
			window = append(window, b) // Fill up to `n`
		} else {
			copy(window, window[1:])
			window[len(window)-1] = b
		}
		count++
	}

	// If `n > len(input)`, return the full string
	if count < n {
		return string(window), nil
	}

	return string(window), nil
}

func left(r io.Reader, n int) (string, error) {
	buf := bufio.NewReader(r)
	var result bytes.Buffer

	for i := 0; i < n; i++ {
		b, err := buf.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		result.WriteByte(b)
	}

	return result.String(), nil
}

func streamEquals(r1 io.Reader, r2 io.Reader) (bool, error) {
	bufSize := 4096
	buf1 := make([]byte, bufSize)
	buf2 := make([]byte, bufSize)

	for {
		n1, err1 := r1.Read(buf1)
		n2, err2 := r2.Read(buf2)

		if n1 != n2 || !bytes.Equal(buf1[:n1], buf2[:n2]) {
			return false, nil // Mismatch found, return immediately
		}

		if err1 == io.EOF && err2 == io.EOF {
			return true, nil // Both reached EOF at the same time
		}

		if err1 != nil && err1 != io.EOF {
			return false, fmt.Errorf("error reading first stream: %w", err1)
		}

		if err2 != nil && err2 != io.EOF {
			return false, fmt.Errorf("error reading second stream: %w", err2)
		}
	}
}
