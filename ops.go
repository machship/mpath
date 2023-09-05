package mpath

import (
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
	Type() ot_OpType
}

////////////////////////////////////////////////////////////////////////////////////

type ot_OpType int

const (
	ot_Path ot_OpType = iota
	ot_PathIdent
	ot_Filter
	ot_LogicalOperation
	ot_Function
)

type opPath struct {
	startAtRoot       bool
	disallowRoot      bool
	mustEndInFunction bool
	operations        []Operation
}

func (x *opPath) addOpToOperationsAndParse(op Operation, s *scanner, r rune) (nextR rune, err error) {
	x.operations = append(x.operations, op)
	return op.Parse(s, r)
}

func (x *opPath) Type() ot_OpType { return ot_Path }

func (x *opPath) Sprint(depth int) (out string) {

	out += repeatTabs(depth)

	switch x.startAtRoot {
	case true:
		out += "$"
	case false:
		out += "@"
	}

	opStrings := []string{}

	for _, op := range x.operations {
		opStrings = append(opStrings, op.Sprint(depth))
	}

	if len(opStrings) > 0 {
		out += "." + strings.Join(opStrings, ".")
	}

	return
}

func (x *opPath) ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool) {
	outCurrent = current

	for _, op := range x.operations {
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
	if x.startAtRoot && x.disallowRoot {
		return nil, fmt.Errorf("cannot access root data in filter")
	}

	if x.startAtRoot {
		dataToUse = originalData
	} else {
		dataToUse = currentData
	}

	if len(x.operations) == 0 {
		// This is a special case where the root is being returned

		// As we always guarantee numbers are returned as the decimal type, we do this check
		if _, ok := dataToUse.(string); !ok {
			dataToUse = convertToDecimalIfNumber(dataToUse)
		}
	}

	// Now we know which data to use, we can apply the path parts
	for _, op := range x.operations {
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
		if x.disallowRoot {
			return r, errors.Wrap(erInvalid(s, '@'), "cannot use '$' (root) inside filter")
		}
		x.startAtRoot = true
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
			if x.mustEndInFunction {
				if len(x.operations) > 0 && x.operations[len(x.operations)-1].Type() == ot_Function {
					if pf, ok := x.operations[len(x.operations)-1].(*opFunction); ok {
						if ft_IsBoolFunc(pf.functionType) {
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
	identName string
}

func (x *opPathIdent) Type() ot_OpType { return ot_PathIdent }

func (x *opPathIdent) Sprint(depth int) (out string) {
	return x.identName
}

func (x *opPathIdent) ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool) {
	return append(current, x.identName), nil, false
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

			if !strings.EqualFold(mks, x.identName) {
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
	return getValuesByName(x.identName, currentData), nil
}
func (x *opPathIdent) Parse(s *scanner, r rune) (nextR rune, err error) {
	x.identName = s.TokenText()

	return s.Scan(), nil
}

////////////////////////////////////////////////////////////////////////////////////

type opFilter struct {
	logicalOperation *opLogicalOperation
}

func (x *opFilter) Type() ot_OpType { return ot_Filter }

func (x *opFilter) Sprint(depth int) (out string) {
	return x.logicalOperation.Sprint(depth)
}

func (x *opFilter) ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool) {
	oc, additional, _ := x.logicalOperation.ForPath(current)
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
		res, err := x.logicalOperation.Do(val, originalData)
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
		res, err := x.logicalOperation.Do(v, originalData)
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

	x.logicalOperation = &opLogicalOperation{}
	x.logicalOperation.disallowRoot = true
	return x.logicalOperation.Parse(s, r)
}

////////////////////////////////////////////////////////////////////////////////////

type opLogicalOperation struct {
	id uint32

	disallowRoot bool

	logicalOperationType lot_LogicalOperationType
	operations           []Operation
}

func (x *opLogicalOperation) addOpToOperationsAndParse(op Operation, s *scanner, r rune) (nextR rune, err error) {
	x.operations = append(x.operations, op)
	return op.Parse(s, r)
}

func (x *opLogicalOperation) Type() ot_OpType { return ot_LogicalOperation }

func (x *opLogicalOperation) Sprint(depth int) (out string) {
	startChar := "{"
	endChar := "}"
	if x.disallowRoot {
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

	switch x.logicalOperationType {
	case lot_And:
		out += "AND,"
	case lot_Or:
		out += "OR,"
	}

	for _, op := range x.operations {
		out += "\n" + op.Sprint(depth+1) + ","
	}

	out += "\n" + repeatTabs(depth) + endChar

	return
}

func (x *opLogicalOperation) ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool) {
	// outCurrent = current

	for _, op := range x.operations {
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
	for _, op := range x.operations {
		res, err := op.Do(currentData, originalData)
		if err != nil {
			return nil, err
		}
		if b, ok := res.(bool); ok {
			switch x.logicalOperationType {
			case lot_And:
				if !b {
					return false, nil
				}
			case lot_Or:
				if b {
					return true, nil
				}
			}
			continue
		}

		// todo: I have hidden this error, but it should perhaps still be present
		return false, nil //fmt.Errorf("op %T didn't return a boolean (returned %T)", op, res)
	}

	switch x.logicalOperationType {
	case lot_And:
		return true, nil
	case lot_Or:
		return false, nil
	}

	return nil, fmt.Errorf("didn't parse result correctly")
}

var (
	globalID = uint32(0)
)

func getNextID() uint32 {
	globalID++
	return globalID
}

func (x *opLogicalOperation) Parse(s *scanner, r rune) (nextR rune, err error) {
	x.id = getNextID()

	if !(r == '{' || r == '[') {
		return r, erInvalid(s, '{', '[')
	}
	r = s.Scan()

	tokenText := s.TokenText()
	if r == sc.Ident && (tokenText == "AND" || tokenText == "OR") {
		switch tokenText {
		case "AND":
			x.logicalOperationType = lot_And
		case "OR":
			x.logicalOperationType = lot_Or
		}
		r = s.Scan()
	} else {
		// We assume that a group without the logical operation defined is
		// an AND operation
		x.logicalOperationType = lot_And
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
			op = &opPath{mustEndInFunction: true, disallowRoot: x.disallowRoot}
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

type lot_LogicalOperationType int

const (
	lot_And lot_LogicalOperationType = iota
	lot_Or
)

////////////////////////////////////////////////////////////////////////////////////

// Functions can only be part of an opPath
type opFunction struct {
	functionType ft_FunctionType
	paramsNumber []decimal.Decimal
	paramsString []string
	paramsBool   []bool
	paramsPath   []*opPath
}

type runtimeParams struct {
	paramsNumber []decimal.Decimal
	paramsString []string
	paramsBool   []bool
}

func (x *opFunction) Type() ot_OpType { return ot_Function }

func (x *opFunction) Sprint(depth int) (out string) {
	paramsAsStrings := []string{}
	for _, p := range x.paramsNumber {
		paramsAsStrings = append(paramsAsStrings, fmt.Sprint(p))
	}
	for _, p := range x.paramsString {
		paramsAsStrings = append(paramsAsStrings, fmt.Sprintf(`"%s"`, p))
	}
	for _, p := range x.paramsBool {
		paramsAsStrings = append(paramsAsStrings, fmt.Sprint(p))
	}
	for _, p := range x.paramsPath {
		paramsAsStrings = append(paramsAsStrings, strings.TrimLeft(p.Sprint(depth), "\t"))
	}

	return fmt.Sprintf("%s(%s)", ft_GetName(x.functionType), strings.Join(paramsAsStrings, ","))
}

func (x *opFunction) ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool) {
	if !ft_ShouldContinueForPath(x.functionType) {
		shouldStopLoop = true
		return
	}
	outCurrent = current

	for _, p := range x.paramsPath {
		pp, a, _ := p.ForPath(current)
		additional = append(additional, pp)
		additional = append(additional, a...)
	}

	return
}

func (x *opFunction) Do(currentData, originalData any) (dataToUse any, err error) {
	rtParams := runtimeParams{}

	rtParams.paramsBool = append(rtParams.paramsBool, x.paramsBool...)
	rtParams.paramsNumber = append(rtParams.paramsNumber, x.paramsNumber...)
	rtParams.paramsString = append(rtParams.paramsString, x.paramsString...)

	// get the pathParams and put them in the appropriate bucket
	for _, ppOp := range x.paramsPath {
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

	switch x.functionType {
	case ft_Equal:
		return x.func_Equal(rtParams, currentData)
	case ft_NotEqual:
		return x.func_NotEqual(rtParams, currentData)

	case ft_Less:
		return x.func_Less(rtParams, currentData)
	case ft_LessOrEqual:
		return x.func_LessOrEqual(rtParams, currentData)
	case ft_Greater:
		return x.func_Greater(rtParams, currentData)
	case ft_GreaterOrEqual:
		return x.func_GreaterOrEqual(rtParams, currentData)

	case ft_Contains:
		return x.func_Contains(rtParams, currentData)
	case ft_NotContains:
		return x.func_NotContains(rtParams, currentData)
	case ft_Prefix:
		return x.func_Prefix(rtParams, currentData)
	case ft_NotPrefix:
		return x.func_NotPrefix(rtParams, currentData)
	case ft_Suffix:
		return x.func_Suffix(rtParams, currentData)
	case ft_NotSuffix:
		return x.func_NotSuffix(rtParams, currentData)

	case ft_Count:
		return x.func_Count(rtParams, currentData)
	case ft_Any:
		return x.func_Any(rtParams, currentData)
	case ft_First:
		return x.func_First(rtParams, currentData)
	case ft_Last:
		return x.func_Last(rtParams, currentData)
	case ft_Index:
		return x.func_Index(rtParams, currentData)

	case ft_Sum:
		return x.func_Sum(rtParams, currentData)
	case ft_Avg:
		return x.func_Avg(rtParams, currentData)
	case ft_Max:
		return x.func_Max(rtParams, currentData)
	case ft_Min:
		return x.func_Min(rtParams, currentData)

	case ft_Add:
		return x.func_Add(rtParams, currentData)
	case ft_Sub:
		return x.func_Sub(rtParams, currentData)
	case ft_Div:
		return x.func_Div(rtParams, currentData)
	case ft_Mul:
		return x.func_Mul(rtParams, currentData)
	case ft_Mod:
		return x.func_Mod(rtParams, currentData)

	case ft_AnyOf:
		return x.func_AnyOf(rtParams, currentData)

	case ft_TrimRightN:
		return x.func_TrimRightN(rtParams, currentData)
	case ft_TrimLeftN:
		return x.func_TrimLeftN(rtParams, currentData)
	case ft_RightN:
		return x.func_RightN(rtParams, currentData)
	case ft_LeftN:
		return x.func_LeftN(rtParams, currentData)
	case ft_DoesMatchRegex:
		return x.func_DoesMatchRegex(rtParams, currentData)
	case ft_ReplaceRegex:
		return x.func_ReplaceRegex(rtParams, currentData)
	case ft_ReplaceAll:
		return x.func_ReplaceAll(rtParams, currentData)

	case ft_AsJSON:
		return x.func_AsJSON(rtParams, currentData)
	case ft_ParseJSON:
		return x.func_ParseJSON(rtParams, currentData)
	case ft_ParseXML:
		return x.func_ParseXML(rtParams, currentData)
	case ft_ParseYAML:
		return x.func_ParseYAML(rtParams, currentData)
	case ft_ParseTOML:
		return x.func_ParseTOML(rtParams, currentData)

	case ft_RemoveKeysByRegex:
		return x.func_RemoveKeysByRegex(rtParams, currentData)
	case ft_RemoveKeysByPrefix:
		return x.func_RemoveKeysByPrefix(rtParams, currentData)
	case ft_RemoveKeysBySuffix:
		return x.func_RemoveKeysBySuffix(rtParams, currentData)

	}

	return nil, fmt.Errorf("unrecognised function")
}

func (x *opFunction) Parse(s *scanner, r rune) (nextR rune, err error) {
	if s.sx.Peek() != '(' {
		return r, erInvalid(s, '(')
	}

	x.functionType, err = ft_GetByName(s.TokenText())
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
			x.paramsString = append(x.paramsString, tt)
		case sc.Float, sc.Int:
			f, err := strconv.ParseFloat(s.TokenText(), 64)
			if err != nil {
				// This should not be possible, but handle it just in case
				return r, erAt(s, "couldn't convert number as string '%s' to number", s.TokenText())
			}
			x.paramsNumber = append(x.paramsNumber, decimal.NewFromFloat(f))
		case sc.Ident:
			//must be bool
			switch s.TokenText() {
			case "true":
				x.paramsBool = append(x.paramsBool, true)
			case "false":
				x.paramsBool = append(x.paramsBool, false)
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
	x.paramsPath = append(x.paramsPath, op)
	return op.Parse(s, r)
}
