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

func (x *opPathIdent) Validate(rootValue cue.Value, cuePath CuePath, blockedRootFields []string) (part *PathIdent, returnedType InputOrOutput) {
	errFunc := func(e error) (*PathIdent, InputOrOutput) {
		if part == nil {
			part = &PathIdent{}
		}

		part.String = x.UserString()
		part.Error = strPtr(e.Error())

		return part, returnedType
	}

	part = &PathIdent{
		pathIdentFields: pathIdentFields{
			Available: &Available{},
			String:    x.UserString(),
		},
	}

	cuePathValue, err := findValueAtPath(rootValue, cuePath)
	if err != nil {
		return errFunc(err)
	}

	k := cuePathValue.IncompleteKind()

	if k == cue.BottomKind {
		cuePathValue = cuePathValue.LookupPath(cue.MakePath(cue.AnyIndex))
		k = cuePathValue.IncompleteKind()
	}

	wasList := false
	seenBottomKind := false
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
		availableFields, err := getAvailableFieldsForValue(cuePathValue, blockedRootFields)
		if err != nil {
			return errFunc(fmt.Errorf("couldn't get fields for struct type to build filters: %w", err))
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
			returnedType.CueExpr = getExpr(cuePathValue)
			return
		}

		wasList = true
		// Check what kind of array
		k, err = getUnderlyingKind(cuePathValue)
		if err != nil {
			return errFunc(fmt.Errorf("couldn't ascertain underlying kind of list for field '%s': %w", part.String, err))
		}
		goto loop

	case cue.BottomKind:
		if !seenBottomKind {
			seenBottomKind = true
			cuePathValue = cuePathValue.LookupPath(cue.MakePath(cue.AnyIndex))
			k = cuePathValue.IncompleteKind()
			goto loop
		}

		errMessage := "unable to find field"
		thisValue := cuePathValue.LookupPath(cue.MakePath(cue.AnyIndex))
		errMessage += fmt.Sprint(thisValue.Err())
		errMessage += thisValue.IncompleteKind().String()

		part.Error = &errMessage
		return

	default:
		// sels := cuePathValue.IncompleteKind()
		// fmt.Println(sels)

		return errFunc(fmt.Errorf("encountered unknown cue kind %v", k))
	}

	part.Available.Functions = getAvailableFunctionsForKind(returnedType)
	part.Type = returnedType
	returnedType.CueExpr = getExpr(cuePathValue)

	return
}

func (x *opPathIdent) ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool) {
	return append(current, x.IdentName), nil, false
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
