package mpath

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	sc "text/scanner"

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
	DisallowRoot      bool
	MustEndInFunction bool
	Operations        []Operation
}

func (x *opPath) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type              string `json:"_type"`
		StartAtRoot       bool
		DisallowRoot      bool
		MustEndInFunction bool
		Operations        []Operation
	}{
		Type:              "Path",
		StartAtRoot:       x.StartAtRoot,
		DisallowRoot:      x.DisallowRoot,
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
	if x.StartAtRoot && x.DisallowRoot {
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
		if x.DisallowRoot {
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
	x.LogicalOperation.DisallowRoot = true
	return x.LogicalOperation.Parse(s, r)
}

////////////////////////////////////////////////////////////////////////////////////

type opLogicalOperation struct {
	DisallowRoot         bool
	LogicalOperationType LOT_LogicalOperationType
	Operations           []Operation
}

func (x *opLogicalOperation) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type                 string `json:"_type"`
		DisallowRoot         bool
		LogicalOperationType LOT_LogicalOperationType
		Operations           []Operation
	}{
		Type:                 "LogicalOperation",
		DisallowRoot:         x.DisallowRoot,
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
	if x.DisallowRoot {
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
			op = &opPath{MustEndInFunction: true, DisallowRoot: x.DisallowRoot}
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

type LOT_LogicalOperationType int

const (
	LOT_And LOT_LogicalOperationType = iota
	LOT_Or
)

////////////////////////////////////////////////////////////////////////////////////

// Functions can only be part of an opPath
type opFunction struct {
	FunctionType FT_FunctionType
	ParamsNumber []decimal.Decimal
	ParamsString []string
	ParamsBool   []bool
	ParamsPath   []*opPath
}

func (x *opFunction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type         string `json:"_type"`
		FunctionName string `json:"_functionName"`
		FunctionType FT_FunctionType
		ParamsNumber []decimal.Decimal
		ParamsString []string
		ParamsBool   []bool
		ParamsPath   []*opPath
	}{
		Type:         "Function",
		FunctionName: ft_GetName(x.FunctionType),
		FunctionType: x.FunctionType,
		ParamsNumber: x.ParamsNumber,
		ParamsString: x.ParamsString,
		ParamsBool:   x.ParamsBool,
		ParamsPath:   x.ParamsPath,
	})
}

type runtimeParams struct {
	paramsNumber []decimal.Decimal
	paramsString []string
	paramsBool   []bool
}

func (x *opFunction) Type() OT_OpType { return OT_Function }

func (x *opFunction) Sprint(depth int) (out string) {
	paramsAsStrings := []string{}
	for _, p := range x.ParamsNumber {
		paramsAsStrings = append(paramsAsStrings, fmt.Sprint(p))
	}
	for _, p := range x.ParamsString {
		paramsAsStrings = append(paramsAsStrings, fmt.Sprintf(`"%s"`, p))
	}
	for _, p := range x.ParamsBool {
		paramsAsStrings = append(paramsAsStrings, fmt.Sprint(p))
	}
	for _, p := range x.ParamsPath {
		paramsAsStrings = append(paramsAsStrings, strings.TrimLeft(p.Sprint(depth), "\t"))
	}

	return fmt.Sprintf("%s(%s)", ft_GetName(x.FunctionType), strings.Join(paramsAsStrings, ","))
}

func (x *opFunction) ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool) {
	if !ft_ShouldContinueForPath(x.FunctionType) {
		shouldStopLoop = true
		return
	}
	outCurrent = current

	for _, p := range x.ParamsPath {
		pp, a, _ := p.ForPath(current)
		additional = append(additional, pp)
		additional = append(additional, a...)
	}

	return
}

func (x *opFunction) Do(currentData, originalData any) (dataToUse any, err error) {
	rtParams := runtimeParams{}

	rtParams.paramsBool = append(rtParams.paramsBool, x.ParamsBool...)
	rtParams.paramsNumber = append(rtParams.paramsNumber, x.ParamsNumber...)
	rtParams.paramsString = append(rtParams.paramsString, x.ParamsString...)

	// get the pathParams and put them in the appropriate bucket
	for _, ppOp := range x.ParamsPath {
		res, err := ppOp.Do(currentData, originalData)
		if err != nil {
			return nil, fmt.Errorf("issue with path parameter: %w", err)
		}
		switch resType := res.(type) {
		case decimal.Decimal:
			rtParams.paramsNumber = append(rtParams.paramsNumber, resType)
		case string:
			rtParams.paramsString = append(rtParams.paramsString, resType)
		case bool:
			rtParams.paramsBool = append(rtParams.paramsBool, resType)
		case []decimal.Decimal:
			rtParams.paramsNumber = append(rtParams.paramsNumber, resType...)
		case []string:
			rtParams.paramsString = append(rtParams.paramsString, resType...)
		case []bool:
			rtParams.paramsBool = append(rtParams.paramsBool, resType...)
		case []float64:
			for _, asFloat := range resType {
				rtParams.paramsNumber = append(rtParams.paramsNumber, decimal.NewFromFloat(asFloat))
			}
		case []int:
			for _, asInt := range resType {
				rtParams.paramsNumber = append(rtParams.paramsNumber, decimal.NewFromInt(int64(asInt)))
			}
		case []any:
			for _, pv := range resType {
				switch pvType := pv.(type) {
				case float64:
					rtParams.paramsNumber = append(rtParams.paramsNumber, decimal.NewFromFloat(pvType))
				case int:
					rtParams.paramsNumber = append(rtParams.paramsNumber, decimal.NewFromInt(int64(pvType)))
				case decimal.Decimal:
					rtParams.paramsNumber = append(rtParams.paramsNumber, pvType)
				case string:
					rtParams.paramsString = append(rtParams.paramsString, pvType)
				case bool:
					rtParams.paramsBool = append(rtParams.paramsBool, pvType)
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
			x.ParamsString = append(x.ParamsString, tt)
		case sc.Float, sc.Int:
			f, err := strconv.ParseFloat(s.TokenText(), 64)
			if err != nil {
				// This should not be possible, but handle it just in case
				return r, erAt(s, "couldn't convert number as string '%s' to number", s.TokenText())
			}
			x.ParamsNumber = append(x.ParamsNumber, decimal.NewFromFloat(f))
		case sc.Ident:
			//must be bool
			switch s.TokenText() {
			case "true":
				x.ParamsBool = append(x.ParamsBool, true)
			case "false":
				x.ParamsBool = append(x.ParamsBool, false)
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
	x.ParamsPath = append(x.ParamsPath, op)
	return op.Parse(s, r)
}
