package mpath

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	sc "text/scanner"

	"cuelang.org/go/cue"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////

type Operation interface {
	Do(currentData, originalData any) (dataToUse any, err error)
	Parse(s *scanner, r rune) (nextR rune, err error)
	Sprint(depth int) (out string)
	ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool)
	Type() OT_OpType
	MarshalJSON() ([]byte, error)
}

////////////////////////////////////////////////////////////////////////////////////

type OT_OpType int

const (
	OT_Path OT_OpType = iota
	OT_PathIdent
	OT_Filter
	OT_LogicalOperation
	OT_Function
)

type opPath struct {
	StartAtRoot       bool
	IsFilter          bool
	MustEndInFunction bool
	Operations        []Operation
}

func (x *opPath) Validate(rootValue, nextValue cue.Value) (parts []*TypeaheadPart, returnedType PT_ParameterType, err error) {
	rootPart := &TypeaheadPart{
		Type: PT_Root,
	}

	parts = []*TypeaheadPart{rootPart}

	switch x.StartAtRoot {
	case true:
		rootPart.String = "$"
	case false:
		rootPart.String = "@"
	}

	availableFields, err := getAvailableFieldsForValue(nextValue)
	if err != nil {
		return nil, returnedType, fmt.Errorf("failed to list available fields from cue: %w", err)
	}

	if len(availableFields) > 0 {
		rootPart.Available = &TypeaheadAvailable{
			Fields: availableFields,
		}
	}

	var shouldErrorRemaining bool
	var part *TypeaheadPart
	for _, op := range x.Operations {
		if shouldErrorRemaining {
			var str string
			switch t := op.(type) {
			case *opPathIdent:
				str = t.IdentName
			case *opFilter:
				str = t.Sprint(0) // todo: is this correct?
			default:
				continue
			}
			errMessage := "cannot continue due to previous error"
			part = &TypeaheadPart{
				String: str,
				Error:  &errMessage,
			}

			continue
		}

		switch t := op.(type) {
		case *opPathIdent:
			if returnedType.IsPrimitive() {
				shouldErrorRemaining = true
				errMessage := "cannot address into primitive value"
				part = &TypeaheadPart{
					String: t.IdentName,
					Error:  &errMessage,
				}
			}

			// opPathIdent Validate advances the next value
			part, nextValue, returnedType, err = t.Validate(nextValue)
			if err != nil {
				return nil, returnedType, err
			}
			parts = append(parts, part)

		case *opFilter:
			// opFilter Validate does not advance the next value
			part.Filter, err = t.Validate(rootValue, nextValue)
			if err != nil {
				return nil, returnedType, err
			}

		case *opFunction:
			part, nextValue, returnedType, err = t.Validate(rootValue, nextValue)
			if err != nil {
				shouldErrorRemaining = true
			}
			parts = append(parts, part)
		}
	}

	return
}

func (x *opPath) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type              string `json:"_type"`
		StartAtRoot       bool
		IsFilter          bool
		MustEndInFunction bool
		Operations        []Operation
	}{
		Type:              "Path",
		StartAtRoot:       x.StartAtRoot,
		IsFilter:          x.IsFilter,
		MustEndInFunction: x.MustEndInFunction,
		Operations:        x.Operations,
	})
}

func (x *opPath) addOpToOperationsAndParse(op Operation, s *scanner, r rune) (nextR rune, err error) {
	x.Operations = append(x.Operations, op)
	return op.Parse(s, r)
}

func (x *opPath) Type() OT_OpType { return OT_Path }

func (x *opPath) Sprint(depth int) (out string) {

	out += repeatTabs(depth)

	switch x.StartAtRoot {
	case true:
		out += "$"
	case false:
		out += "@"
	}

	opStrings := []string{}

	for _, op := range x.Operations {
		opStrings = append(opStrings, op.Sprint(depth))
	}

	if len(opStrings) > 0 {
		out += "." + strings.Join(opStrings, ".")
	}

	return
}

func (x *opPath) ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool) {
	outCurrent = current

	for _, op := range x.Operations {
		pass := outCurrent
		// if op.Type() != ot_Filter {
		// 	pass = nil
		// }

		oc, a, shouldStopLoop := op.ForPath(pass)
		if shouldStopLoop {
			break
		}

		outCurrent = oc
		if len(a) > 0 {
			additional = append(additional, a...)
		}
	}

	return
}

func (x *opPath) Do(currentData, originalData any) (dataToUse any, err error) {
	if x.StartAtRoot && x.IsFilter {
		return nil, fmt.Errorf("cannot access root data in filter")
	}

	if x.StartAtRoot {
		dataToUse = originalData
	} else {
		dataToUse = currentData
	}

	if len(x.Operations) == 0 {
		// This is a special case where the root is being returned

		// As we always guarantee numbers are returned as the decimal type, we do this check
		if _, ok := dataToUse.(string); !ok {
			dataToUse = convertToDecimalIfNumber(dataToUse)
		}
	}

	// Now we know which data to use, we can apply the path parts
	for _, op := range x.Operations {
		dataToUse, err = op.Do(dataToUse, originalData)
		if err != nil {
			return nil, fmt.Errorf("path op failed: %w", err)
		}
		if dataToUse == nil {
			return
		}
	}

	return
}

func (x *opPath) Parse(s *scanner, r rune) (nextR rune, err error) {
	switch r {
	case '$':
		if x.IsFilter {
			return r, errors.Wrap(erInvalid(s, '@'), "cannot use '$' (root) inside filter")
		}
		x.StartAtRoot = true
	case '@':
		// do nothing, this is the default
	default:
		return r, erInvalid(s, '$', '@')
	}

	r = s.Scan()

	var op Operation
	for { //i := 1; i > 0; i++ {
		if r == sc.EOF {
			break
		}

		switch r {
		case '.':
			// This is the separator, we can move on
			r = s.Scan()
			continue

		case ',', ')', ']', '}':
			// This should mean we are finished the path
			if x.MustEndInFunction {
				if len(x.Operations) > 0 && x.Operations[len(x.Operations)-1].Type() == OT_Function {
					if pf, ok := x.Operations[len(x.Operations)-1].(*opFunction); ok {
						if ft_IsBoolFunc(pf.FunctionType) {
							return r, nil
						}
					}
				}

				return r, erAt(s, "paths that are part of a logical operation must end in a boolean function")
			}

			return r, nil

		case sc.Ident:
			// Need to check if this is the name of a function
			p := s.sx.Peek()
			if p == '(' {
				op = &opFunction{}
			} else {
				// This should be a field name
				op = &opPathIdent{}
			}

		case '[':
			// This is a filter
			op = &opFilter{}

		default:
			// log.Printf("got %s (%d) [%t] (%d) \n", string(r), r, unicode.IsPrint(r), '\x00')
			return r, erInvalid(s)
		}

		if r, err = x.addOpToOperationsAndParse(op, s, r); err != nil {
			return r, err
		}
	}

	return
}

////////////////////////////////////////////////////////////////////////////////////

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

////////////////////////////////////////////////////////////////////////////////////

type opFilter struct {
	LogicalOperation *opLogicalOperation
}

func (x *opFilter) Validate(rootValue, inputValue cue.Value) (filter *TypeaheadFilter, err error) {
	filter = &TypeaheadFilter{
		String: x.Sprint(0), // todo: is this right?
	}

	if inputValue.Kind() != cue.ListKind {
		errMessage := "not a list; only lists can be filtered"
		filter.Error = &errMessage
	}

	it, err := inputValue.List()
	if err != nil {
		return nil, fmt.Errorf("couldn't get list iterator for list kind")
	}

	it.Next()
	nextValue := it.Value()

	filter.LogicalOperator, filter.LogicalOperations, err = x.LogicalOperation.Validate(rootValue, nextValue)
	if err != nil {
		errMessage := err.Error()
		filter.Error = &errMessage
	}

	return
}

func (x *opFilter) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type             string `json:"_type"`
		LogicalOperation *opLogicalOperation
	}{
		Type:             "Filter",
		LogicalOperation: x.LogicalOperation,
	})
}

func (x *opFilter) Type() OT_OpType { return OT_Filter }

func (x *opFilter) Sprint(depth int) (out string) {
	return x.LogicalOperation.Sprint(depth)
}

func (x *opFilter) ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool) {
	oc, additional, _ := x.LogicalOperation.ForPath(current)
	outCurrent = current
	a := []string{}
	a = append(a, oc...)
	if len(a) > 0 {
		additional = append(additional, a)
	}
	return
}

func (x *opFilter) Do(currentData, originalData any) (dataToUse any, err error) {

	val, ok, wasStruct := getAsStructOrSlice(currentData)
	if !ok {
		return nil, fmt.Errorf("value was not object or array and cannot be filtered")
	}

	if wasStruct {
		res, err := x.LogicalOperation.Do(val, originalData)
		if err != nil {
			return nil, err
		}

		if _, ok := res.(bool); ok {
			return val, nil
		}
		return nil, nil
	}

	newOut := []any{}
	for _, v := range val.([]any) {
		res, err := x.LogicalOperation.Do(v, originalData)
		if err != nil {
			return nil, err
		}

		if res.(bool) {
			newOut = append(newOut, v)
		}

	}
	return newOut, nil
}

func (x *opFilter) Parse(s *scanner, r rune) (nextR rune, err error) {
	if r != '[' {
		return r, erInvalid(s, '[')
	}

	x.LogicalOperation = &opLogicalOperation{}
	x.LogicalOperation.IsFilter = true
	return x.LogicalOperation.Parse(s, r)
}

////////////////////////////////////////////////////////////////////////////////////

type opLogicalOperation struct {
	IsFilter             bool
	LogicalOperationType LOT_LogicalOperationType
	Operations           []Operation
}

func (x *opLogicalOperation) Validate(rootValue, nextValue cue.Value) (operator *LOT_LogicalOperationType, operations []*TypeaheadConfig, err error) {
	operator = &x.LogicalOperationType

	for _, op := range x.Operations {
		switch t := op.(type) {
		case *opPath:
			operation := &TypeaheadConfig{
				String: op.Sprint(0), // todo: is this correct?
			}
			operations = append(operations, operation)

			operation.Parts, operation.Type, err = t.Validate(rootValue, nextValue)
			if err != nil {
				errMessage := err.Error()
				operation.Error = &errMessage
			}

		case *opLogicalOperation:
			operation := &TypeaheadConfig{
				String: op.Sprint(0), // todo: is this correct?
			}
			subOperator, subOperations, err := t.Validate(rootValue, nextValue)
			if err != nil {
				errMessage := err.Error()
				operation.Error = &errMessage
				continue
			}

			operation.Parts = append(operation.Parts, &TypeaheadPart{
				String:            op.Sprint(0),
				Type:              PT_Boolean,
				LogicalOperator:   subOperator,
				LogicalOperations: subOperations,
			})
		}
	}

	return
}

func (x *opLogicalOperation) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type                 string `json:"_type"`
		IsFilter             bool
		LogicalOperationType LOT_LogicalOperationType
		Operations           []Operation
	}{
		Type:                 "LogicalOperation",
		IsFilter:             x.IsFilter,
		LogicalOperationType: x.LogicalOperationType,
		Operations:           x.Operations,
	})
}

func (x *opLogicalOperation) addOpToOperationsAndParse(op Operation, s *scanner, r rune) (nextR rune, err error) {
	x.Operations = append(x.Operations, op)
	return op.Parse(s, r)
}

func (x *opLogicalOperation) Type() OT_OpType { return OT_LogicalOperation }

func (x *opLogicalOperation) Sprint(depth int) (out string) {
	startChar := "{"
	endChar := "}"
	if x.IsFilter {
		startChar = "["
		endChar = "]"
	}

	switch startChar {
	case "{":
		out += repeatTabs(depth) + startChar
	case "[":
		out += startChar
	}

	out += "\n" + repeatTabs(depth+1)

	switch x.LogicalOperationType {
	case LOT_And:
		out += "AND,"
	case LOT_Or:
		out += "OR,"
	}

	for _, op := range x.Operations {
		out += "\n" + op.Sprint(depth+1) + ","
	}

	out += "\n" + repeatTabs(depth) + endChar

	return
}

func (x *opLogicalOperation) ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool) {
	// outCurrent = current

	for _, op := range x.Operations {
		oc, a, _ := op.ForPath(nil)
		nc := []string{}
		nc = append(nc, current...)
		nc = append(nc, oc...)
		if len(nc) > 0 {
			additional = append(additional, nc)
		}
		if len(a) > 0 {
			additional = append(additional, a...)
		}
	}

	return
}

func (x *opLogicalOperation) Do(currentData, originalData any) (dataToUse any, err error) {
	for _, op := range x.Operations {
		res, err := op.Do(currentData, originalData)
		if err != nil {
			return nil, err
		}
		if b, ok := res.(bool); ok {
			switch x.LogicalOperationType {
			case LOT_And:
				if !b {
					return false, nil
				}
			case LOT_Or:
				if b {
					return true, nil
				}
			}
			continue
		}

		// todo: I have hidden this error, but it should perhaps still be present
		return false, nil //fmt.Errorf("op %T didn't return a boolean (returned %T)", op, res)
	}

	switch x.LogicalOperationType {
	case LOT_And:
		return true, nil
	case LOT_Or:
		return false, nil
	}

	return nil, fmt.Errorf("didn't parse result correctly")
}

func (x *opLogicalOperation) Parse(s *scanner, r rune) (nextR rune, err error) {
	if !(r == '{' || r == '[') {
		return r, erInvalid(s, '{', '[')
	}
	r = s.Scan()

	tokenText := s.TokenText()
	if r == sc.Ident && (tokenText == "AND" || tokenText == "OR") {
		switch tokenText {
		case "AND":
			x.LogicalOperationType = LOT_And
		case "OR":
			x.LogicalOperationType = LOT_Or
		}
		r = s.Scan()
	} else {
		// We assume that a group without the logical operation defined is
		// an AND operation
		x.LogicalOperationType = LOT_And
	}

	var op Operation
	for i := 1; i > 0; i++ {
		if r == sc.EOF {
			break
		}

		switch r {
		case ',':
			// This is the separator, we can move on
			r = s.Scan()
			continue

		case '$', '@':
			// This is an opPath
			op = &opPath{MustEndInFunction: true, IsFilter: x.IsFilter}
		case '{':
			// This is an opLogicalOperation
			op = &opLogicalOperation{}
		case '}', ']':
			// This is the end of this logical operation
			return s.Scan(), nil
		default:
			return r, erInvalid(s)
		}

		if r, err = x.addOpToOperationsAndParse(op, s, r); err != nil {
			return r, err
		}
	}

	return
}

type LOT_LogicalOperationType string

const (
	LOT_And LOT_LogicalOperationType = "And"
	LOT_Or  LOT_LogicalOperationType = "Or"
)

////////////////////////////////////////////////////////////////////////////////////

type FunctionParameters []FunctionParameter

func (x FunctionParameters) Numbers() (out []*FP_Number) {
	for _, fp := range x {
		switch t := fp.(type) {
		case *FP_Number:
			out = append(out, t)
		}
	}
	return
}

func (x FunctionParameters) Strings() (out []*FP_String) {
	for _, fp := range x {
		switch t := fp.(type) {
		case *FP_String:
			out = append(out, t)
		}
	}
	return
}

func (x FunctionParameters) Bools() (out []*FP_Bool) {
	for _, fp := range x {
		switch t := fp.(type) {
		case *FP_Bool:
			out = append(out, t)
		}
	}
	return
}

func (x FunctionParameters) Paths() (out []*FP_Path) {
	for _, fp := range x {
		switch t := fp.(type) {
		case *FP_Path:
			out = append(out, t)
		}
	}
	return
}

type FunctionParameter interface {
	IsFuncParam()
	String() string
	GetValue() any
}

func functionParameterMarshalJSON(value any, typeName string) ([]byte, error) {
	return json.Marshal(struct {
		Type  string `json:"_type"`
		Value any    `json:"Value"`
	}{
		Type:  typeName,
		Value: value,
	})
}

type FP_Number struct {
	Value decimal.Decimal
}

func (p FP_Number) String() string {
	return p.Value.String()
}

func (x *FP_Number) IsFuncParam() {}

func (x *FP_Number) GetValue() any { return x.Value }

func (x *FP_Number) MarshalJSON() ([]byte, error) {
	return functionParameterMarshalJSON(x.Value, "Number")
}

type FP_String struct {
	Value string
}

func (p FP_String) String() string {
	return fmt.Sprintf(`"%s"`, p.Value)
}

func (x *FP_String) IsFuncParam() {}

func (x *FP_String) GetValue() any { return x.Value }

func (x *FP_String) MarshalJSON() ([]byte, error) {
	return functionParameterMarshalJSON(x.Value, "String")
}

type FP_Bool struct {
	Value bool
}

func (p FP_Bool) String() string {
	return fmt.Sprint(p.Value)
}

func (x *FP_Bool) IsFuncParam() {}

func (x *FP_Bool) GetValue() any { return x.Value }

func (x *FP_Bool) MarshalJSON() ([]byte, error) {
	return functionParameterMarshalJSON(x.Value, "Bool")
}

type FP_Path struct {
	Value *opPath
}

func (p FP_Path) String() string {
	//return strings.TrimLeft(p.Value.Sprint(0), "\t")
	return p.Value.Sprint(0) // todo: is this correct?
}

func (x *FP_Path) IsFuncParam() {}

func (x *FP_Path) GetValue() any { return x.Value }

func (x *FP_Path) MarshalJSON() ([]byte, error) {
	return functionParameterMarshalJSON(x.Value, "Path")
}

func (x *FP_Path) ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool) {
	return x.Value.ForPath(current)
}

// Functions can only be part of an opPath
type opFunction struct {
	FunctionType FT_FunctionType

	Params FunctionParameters
}

func (x *opFunction) Validate(rootValue, inputValue cue.Value) (part *TypeaheadPart, nextValue cue.Value, returnedType PT_ParameterType, err error) {
	part = &TypeaheadPart{
		String:       x.Sprint(0), //todo: is this correct?
		FunctionName: (*string)(&x.FunctionType),
	}

	// Find the function descriptor
	fd, ok := funcMap[x.FunctionType]
	if !ok {
		errMessage := "unknown function"
		part.Error = &errMessage
		return
	}

	returnedType = fd.Returns

	part.Parameters = []*TypeaheadParameter{}
	for i, p := range x.Params {
		param := &TypeaheadParameter{
			String: p.String(),
		}
		part.Parameters = append(part.Parameters, param)

		//get the parameter at this position
		pd, err := fd.GetParamAtPosition(i)
		if err != nil {
			errMessage := err.Error()
			param.Error = &errMessage
			continue
		}

		switch t := p.(type) {
		case *FP_Path:
			param.Parts, returnedType, err = t.Value.Validate(rootValue, nextValue)
			if err != nil {
				errMessage := err.Error()
				param.Error = &errMessage
				continue
			}

		case *FP_Bool:
			returnedType = PT_Boolean
		case *FP_Number:
			returnedType = PT_Number
		case *FP_String:
			returnedType = PT_String
		}

		// Check that the returned type is appropriate
		if !(pd.Type == PT_Any || returnedType == pd.Type) {
			errMessage := fmt.Sprintf("incorrect parameter type: wanted '%s'; got '%s'", pd.Type, returnedType)
			param.Error = &errMessage
		}
	}

	return
}

func (x *opFunction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type         string `json:"_type"`
		FunctionName string `json:"_functionName"`
		FunctionType FT_FunctionType
		Params       []FunctionParameter
	}{
		Type:         "Function",
		FunctionName: ft_GetName(x.FunctionType),
		FunctionType: x.FunctionType,
		Params:       x.Params,
	})
}

func (x *opFunction) Type() OT_OpType { return OT_Function }

func (x *opFunction) Sprint(depth int) (out string) {
	paramsAsStrings := []string{}

	for _, p := range x.Params {
		paramsAsStrings = append(paramsAsStrings, p.String())
	}

	return fmt.Sprintf("%s(%s)", ft_GetName(x.FunctionType), strings.Join(paramsAsStrings, ","))
}

func (x *opFunction) ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool) {
	if !ft_ShouldContinueForPath(x.FunctionType) {
		shouldStopLoop = true
		return
	}
	outCurrent = current

	for _, p := range x.Params.Paths() {
		pp, a, _ := p.ForPath(current)
		additional = append(additional, pp)
		additional = append(additional, a...)
	}

	return
}

func (x *opFunction) Do(currentData, originalData any) (dataToUse any, err error) {
	var rtParams FunctionParameters

	// get the pathParams and put them in the appropriate bucket
	for _, param := range x.Params {
		var ppOp *opPath
		switch t := param.(type) {
		case *FP_Number, *FP_String, *FP_Bool:
			rtParams = append(rtParams, t)
			continue
		case *FP_Path:
			ppOp = t.Value
		}

		res, err := ppOp.Do(currentData, originalData)
		if err != nil {
			return nil, fmt.Errorf("issue with path parameter: %w", err)
		}
		switch resType := res.(type) {
		case decimal.Decimal:
			rtParams = append(rtParams, &FP_Number{resType})
		case string:
			rtParams = append(rtParams, &FP_String{resType})
		case bool:
			rtParams = append(rtParams, &FP_Bool{resType})
		case []decimal.Decimal:
			for _, rt := range resType {
				rtParams = append(rtParams, &FP_Number{rt})
			}
		case []string:
			for _, rt := range resType {
				rtParams = append(rtParams, &FP_String{rt})
			}
		case []bool:
			for _, rt := range resType {
				rtParams = append(rtParams, &FP_Bool{rt})
			}
		case []float64:
			for _, asFloat := range resType {
				rtParams = append(rtParams, &FP_Number{decimal.NewFromFloat(asFloat)})
			}
		case []int:
			for _, asInt := range resType {
				rtParams = append(rtParams, &FP_Number{decimal.NewFromInt(int64(asInt))})
			}
		case []any:
			for _, pv := range resType {
				switch pvType := pv.(type) {
				case float64:
					rtParams = append(rtParams, &FP_Number{decimal.NewFromFloat(pvType)})
				case int:
					rtParams = append(rtParams, &FP_Number{decimal.NewFromInt(int64(pvType))})
				case decimal.Decimal:
					rtParams = append(rtParams, &FP_Number{pvType})
				case string:
					rtParams = append(rtParams, &FP_String{pvType})
				case bool:
					rtParams = append(rtParams, &FP_Bool{pvType})
				default:
					return nil, fmt.Errorf("unhandled param path type: %T", pv)
				}
			}
		default:
			return nil, fmt.Errorf("unhandled param path type: %T", resType)
		}
	}

	currentData = convertToDecimalIfNumber(currentData)

	funcToRun, ok := funcMap[x.FunctionType]
	if !ok {
		return nil, fmt.Errorf("unrecognised function")
	}

	return funcToRun.fn(rtParams, currentData)
}

func (x *opFunction) Parse(s *scanner, r rune) (nextR rune, err error) {
	if s.sx.Peek() != '(' {
		return r, erInvalid(s, '(')
	}

	x.FunctionType, err = ft_GetByName(s.TokenText())
	if err != nil {
		return r, erAt(s, err.Error())
	}

	r = s.Scan()

	for {
		if r == sc.EOF {
			break
		}

		switch r {
		case ',':
			// This is the separator, we can move on
			r = s.Scan()
			continue
		case ')':
			// This is the end of the function
			return s.Scan(), nil
		case '$', '@':
			// This is a path
			if r, err = x.addOpToParamsAndParse(s, r); err != nil {
				return r, err
			}
			continue
		case sc.String, sc.RawString, sc.Char:
			tt := s.TokenText()
			if len(tt) >= 2 && strings.HasPrefix(tt, `"`) && strings.HasSuffix(tt, `"`) {
				tt = tt[1 : len(tt)-1]
			}
			x.Params = append(x.Params, &FP_String{tt})
		case sc.Float, sc.Int:
			f, err := strconv.ParseFloat(s.TokenText(), 64)
			if err != nil {
				// This should not be possible, but handle it just in case
				return r, erAt(s, "couldn't convert number as string '%s' to number", s.TokenText())
			}
			x.Params = append(x.Params, &FP_Number{decimal.NewFromFloat(f)})
		case sc.Ident:
			//must be bool
			switch s.TokenText() {
			case "true":
				x.Params = append(x.Params, &FP_Bool{true})
			case "false":
				x.Params = append(x.Params, &FP_Bool{false})
			default:
				return r, erInvalid(s)
			}
		}
		r = s.Scan()
	}

	return
}

func (x *opFunction) addOpToParamsAndParse(s *scanner, r rune) (nextR rune, err error) {
	op := &opPath{}
	x.Params = append(x.Params, &FP_Path{op})
	return op.Parse(s, r)
}
