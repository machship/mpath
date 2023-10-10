package mpath

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"cuelang.org/go/cue"
)

type opPathIdent struct {
	IdentName string
}

func (x *opPathIdent) Validate(inputValue cue.Value) (part *TypeaheadPart, nextValue cue.Value, returnedType PT_ParameterType, err error) {
	part = &TypeaheadPart{}

	// find the cue value for this ident
	part.String = x.IdentName
	part.Type = PT_Object
	nextValue, err = findValuePath(inputValue, x.IdentName)
	if err != nil {
		errMessage := err.Error()
		part.Error = &errMessage
	}

	k := nextValue.Kind()
	wasList := false
loop:
	switch k {
	// Primative Kinds:
	case cue.BoolKind:
		returnedType = PT_Boolean
		part.Available.Functions = getAvailableFunctionsForKind(PT_Boolean, false)
	case cue.StringKind:
		returnedType = PT_String
		part.Available.Functions = getAvailableFunctionsForKind(PT_String, false)
	case cue.NumberKind, cue.IntKind, cue.FloatKind:
		if wasList {
			returnedType = PT_ArrayOfNumbers
			part.Available.Functions = getAvailableFunctionsForKind(PT_ArrayOfNumbers, false)
		} else {
			returnedType = PT_Number
			part.Available.Functions = getAvailableFunctionsForKind(PT_Number, false)
		}
		extraFuncs := getAvailableFunctionsForKind(PT_NumberOrArrayOfNumbers, true)
		part.Available.Functions = append(part.Available.Functions, extraFuncs...)
	case cue.StructKind:
		returnedType = PT_Object
		part.Available.Functions = getAvailableFunctionsForKind(PT_Object, false)

		// Get the fields for the next value:
		availableFields, err := getAvailableFieldsForValue(nextValue)
		if err != nil {
			return nil, nextValue, returnedType, fmt.Errorf("couldn't get fields for struct type to build filters: %w", err)
		}

		for _, af := range availableFields {
			part.Available.Filters = append(part.Available.Filters, "@."+af)
		}

	case cue.ListKind:
		if wasList {
			returnedType = PT_Array
			part.Available.Functions = getAvailableFunctionsForKind(PT_Any, true)
			return
		}

		wasList = true
		// Check what kind of array
		k, err = getUnderlyingKind(nextValue)
		if err != nil {
			return nil, nextValue, returnedType, fmt.Errorf("couldn't ascertain underlying kind of list for field '%s': %w", part.String, err)
		}
		goto loop

	default:
		return nil, nextValue, returnedType, fmt.Errorf("encountered unknown cue kind %v", k)
	}

	return
}

func (x *opPathIdent) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type      string `json:"_type"`
		IdentName string
	}{
		Type:      "PathIdent",
		IdentName: x.IdentName,
	})
}

func (x *opPathIdent) Type() OT_OpType { return OT_PathIdent }

func (x *opPathIdent) Sprint(depth int) (out string) {
	return x.IdentName
}

func (x *opPathIdent) ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool) {
	return append(current, x.IdentName), nil, false
}

func (x *opPathIdent) Do(currentData, _ any) (dataToUse any, err error) {
	// Ident paths require that the data is a struct or map[string]any

	// Deal with maps
	// if m, ok := currentData.(map[string]any); ok {
	v := reflect.ValueOf(currentData)
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

			if !strings.EqualFold(mks, x.IdentName) {
				continue
			}

			dataToUse = v.MapIndex(e).Interface()
			if _, ok := dataToUse.(string); !ok {
				dataToUse = convertToDecimalIfNumber(dataToUse)
			}
			return
		}

		return nil, nil
	}

	// If we get here, the data must be a struct
	// and we will look for the field by name
	return getValuesByName(x.IdentName, currentData), nil
}
func (x *opPathIdent) Parse(s *scanner, r rune) (nextR rune, err error) {
	x.IdentName = s.TokenText()

	return s.Scan(), nil
}
