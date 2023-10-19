package mpath

import (
	"fmt"
	"reflect"
	"strings"

	"cuelang.org/go/cue"
)

type opPathIdent struct {
	IdentName string
	opCommon
}

func (x *opPathIdent) Validate(inputValue cue.Value) (part *PathIdent, nextValue cue.Value, returnedType InputOrOutput, err error) {
	part = &PathIdent{
		pathIdentFields: pathIdentFields{
			Available: &Available{},
		},
	}

	// find the cue value for this ident
	part.String = x.UserString()
	nextValue, err = findValuePath(inputValue, x.IdentName)
	if err != nil {
		errMessage := err.Error()
		part.Error = &errMessage
		err = nil
	}

	k := nextValue.IncompleteKind()
	wasList := false
loop:
	switch k {
	// Primative Kinds:
	case cue.BoolKind:
		if wasList {
			returnedType = inputOrOutput(PT_Boolean, IOOT_Array)
		} else {
			returnedType = inputOrOutput(PT_Boolean, IOOT_Single)
		}
	case cue.StringKind:
		if wasList {
			returnedType = inputOrOutput(PT_String, IOOT_Array)
		} else {
			returnedType = inputOrOutput(PT_String, IOOT_Single)
		}
	case cue.NumberKind, cue.IntKind, cue.FloatKind:
		if wasList {
			returnedType = inputOrOutput(PT_Number, IOOT_Array)
		} else {
			returnedType = inputOrOutput(PT_Number, IOOT_Single)
		}
	case cue.StructKind:
		if wasList {
			returnedType = inputOrOutput(PT_Object, IOOT_Array)
		} else {
			returnedType = inputOrOutput(PT_Object, IOOT_Single)
		}

		// Get the fields for the next value:
		availableFields, err := getAvailableFieldsForValue(nextValue)
		if err != nil {
			return nil, nextValue, returnedType, fmt.Errorf("couldn't get fields for struct type to build filters: %w", err)
		}

		if !wasList {
			part.Available.Fields = availableFields
		}

		if wasList {
			for _, af := range availableFields {
				part.Available.Filters = append(part.Available.Filters, "@."+af)
			}
		}

	case cue.ListKind:
		if wasList {
			returnedType = inputOrOutput(PT_Any, IOOT_Single)
			return
		}

		wasList = true
		// Check what kind of array
		k, err = getUnderlyingKind(nextValue)
		if err != nil {
			return nil, nextValue, returnedType, fmt.Errorf("couldn't ascertain underlying kind of list for field '%s': %w", part.String, err)
		}
		goto loop

	case cue.BottomKind:
		errMessage := "unable to find field"
		part.Error = &errMessage
		return

	default:
		sels := nextValue.IncompleteKind()
		fmt.Println(sels)

		return nil, nextValue, returnedType, fmt.Errorf("encountered unknown cue kind %v", k)
	}

	part.Available.Functions = getAvailableFunctionsForKind(returnedType)
	part.Type = returnedType

	return
}

func (x *opPathIdent) Type() OT_OpType { return OT_PathIdent }

func (x *opPathIdent) Sprint(depth int) (out string) {
	return x.IdentName
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
	x.userString = x.IdentName

	return s.Scan(), nil
}
