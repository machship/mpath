package mpath

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	sc "text/scanner"

	"cuelang.org/go/cue"
	"github.com/shopspring/decimal"
)

// Functions can only be part of an opPath
type opFunction struct {
	IsInvalid    bool
	FunctionType FT_FunctionType

	Params FunctionParameterTypes
	opCommon
}

func (x *opFunction) Validate(rootValue, inputValue cue.Value, previousType InputOrOutput, blockedRootFields []string) (part *Function, returnedType InputOrOutput, requiredData []string, err error) {
	part = &Function{
		functionFields: functionFields{
			String:       x.UserString(),
			FunctionName: (*string)(&x.FunctionType),
			Available:    &Available{},
		},
	}

	if x.IsInvalid {
		errMessage := fmt.Sprintf("invalid operation type '%s'", x.FunctionType)
		part.Error = &errMessage
	}

	// Find the function descriptor
	fd, ok := funcMap[x.FunctionType]
	if !ok {
		errMessage := "unknown function"
		part.Error = &errMessage
		return
	}

	if fd.ValidOn.IOType != IOOT_Variadic && fd.ValidOn.Type != PT_Any && fd.ValidOn.Type != previousType.Type {
		errMessage := fmt.Sprintf("cannot use this function on type %s; can use on %s", previousType.Type, fd.ValidOn.Type)
		part.Error = &errMessage
	}

	if fd.ValidOn.Type != PT_Any && fd.ValidOn.IOType != previousType.IOType {
		errMessage := fmt.Sprintf("cannot use this function on type %s; can use on %s", previousType.IOType, fd.ValidOn.IOType)
		if part.Error != nil {
			errMessage = fmt.Sprintf("%s; %s", *part.Error, errMessage)
		}
		part.Error = &errMessage
	}

	returnedType = fd.Returns
	part.Type = fd.Returns

	rdm := map[string]struct{}{}

	var variadicType *PT_ParameterType
	var variadicPosition int

	part.FunctionParameters = []*FunctionParameter{}
	for i, p := range x.Params {
		param := &FunctionParameter{
			String: p.String(),
		}
		part.FunctionParameters = append(part.FunctionParameters, param)

		paramReturns := p.IsFuncParam()

		switch t := p.(type) {
		case *FP_Path:
			var rd []string
			var pathOp *Path
			pathOp, _, rd, err = t.Value.Validate(rootValue, rootValue, blockedRootFields)
			param.Part = pathOp
			if err != nil {
				errMessage := err.Error()
				param.Error = &errMessage
				continue
			}
			pathOp.String = p.String()

			for _, rdv := range rd {
				rdm[rdv] = struct{}{}
			}

			if len(pathOp.Parts) == 0 {
				errMessage := "no parts returned for path"
				param.Error = &errMessage
				continue
			}

			paramReturns = pathOp.ReturnType()

		case *FP_LogicalOperation:
			var rd []string
			var logOp *LogicalOperation
			logOp, rd, err = t.Value.Validate(rootValue, rootValue, blockedRootFields)
			param.Part = logOp
			if err != nil {
				errMessage := err.Error()
				param.Error = &errMessage
				continue
			}
			logOp.String = p.String()

			for _, rdv := range rd {
				rdm[rdv] = struct{}{}
			}

			if len(logOp.Parts) == 0 {
				errMessage := "no parts returned for path"
				param.Error = &errMessage
				continue
			}

			paramReturns = logOp.ReturnType()
		}

		pos := i
		if variadicType != nil {
			pos = variadicPosition
		}
		//get the parameter at this position
		pd, err := fd.GetParamAtPosition(pos)
		if err != nil {
			errMessage := err.Error()
			param.Error = &errMessage
			continue
		}

		if variadicType == nil && pd.IOType == IOOT_Variadic {
			vpt := paramReturns.Type
			variadicType = &vpt
			variadicPosition = i
		}

		if variadicType != nil {
			param.IsVariadicOfParameterAtPosition = &variadicPosition
		}

		param.Type = paramReturns

		switch pd.IOType {
		case IOOT_Single:
			if paramReturns.IOType != IOOT_Single {
				errMessage := fmt.Sprintf("incorrect parameter type: expected single value, got %s", paramReturns.IOType)
				param.Error = &errMessage
			}
			continue
		case IOOT_Array:
			if paramReturns.IOType != IOOT_Array {
				errMessage := fmt.Sprintf("incorrect parameter type: expected array value, got %s", paramReturns.IOType)
				param.Error = &errMessage
			}
			continue
		case IOOT_Variadic:
			// Do nothing, this can accept either a single or an array value
		}

		if pd.Type != PT_Any && pd.Type != paramReturns.Type {
			// This means that the parameter does not accept "Any" type and the returned type is wrong for the expected input
			errMessage := fmt.Sprintf("incorrect parameter type: wanted '%s'; got '%s'", pd.Type, paramReturns.Type)
			param.Error = &errMessage
		}
	}

	for rdv := range rdm {
		requiredData = append(requiredData, rdv)
	}
	sort.Strings(requiredData)

	explanation := fd.explanationFunc(*part)
	part.FunctionExplanation = &explanation

	var k cue.Kind
	k, _ = getUnderlyingKind(inputValue)

	if fd.Returns.Type == PT_Any {
		switch k {
		// Primative Kinds:
		case cue.BoolKind:
			returnedType.Type = PT_Boolean
		case cue.StringKind:
			returnedType.Type = PT_String
		case cue.NumberKind, cue.IntKind, cue.FloatKind:
			returnedType.Type = PT_Number
		case cue.StructKind:
			returnedType.Type = PT_Object
		case cue.ListKind:
			returnedType.Type = PT_Any //todo: can I use the underlying type?
		}
	}
	part.Type = returnedType

	if fd.ReturnsKnownValues && previousType.IOType == IOOT_Array && k == cue.StructKind {
		// We can find available fields
		returnedType.Type = PT_Object
		part.Available.Fields, err = getAvailableFieldsForValue(inputValue, blockedRootFields)
		if err != nil {
			errMessage := fmt.Sprintf("failed to get available fields: %v", err)
			if part.Error != nil {
				errMessage = *part.Error + "; " + errMessage
			}
			part.Error = &errMessage
		}
	}
	part.Available.Functions = append(part.Available.Functions, getAvailableFunctionsForKind(returnedType)...)

	return
}

func (x *opFunction) Type() OT_OpType { return OT_Function }

func (x *opFunction) Sprint(depth int) (out string) {
	paramsAsStrings := []string{}

	for _, p := range x.Params {
		paramsAsStrings = append(paramsAsStrings, p.String())
	}

	return fmt.Sprintf("%s(%s)", ft_GetName(x.FunctionType), strings.Join(paramsAsStrings, ","))
}

func (x *opFunction) Do(currentData, originalData any) (dataToUse any, err error) {
	var rtParams FunctionParameterTypes

	// get the pathParams and put them in the appropriate bucket
	for _, param := range x.Params {
		var ppOp Operation
		switch t := param.(type) {
		case *FP_Number, *FP_String, *FP_Bool:
			rtParams = append(rtParams, t)
			continue
		case *FP_Path:
			ppOp = t.Value
		case *FP_LogicalOperation:
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
		x.IsInvalid = true
		x.FunctionType = FT_FunctionType(s.TokenText())
		// return r, erAt(s, err.Error())
	}
	x.userString += string(x.FunctionType)

	r = s.Scan()
	x.userString += string(r)

	for {
		if r == sc.EOF {
			break
		}

		switch r {
		case ',':
			x.userString += string(r)
			// This is the separator, we can move on
			r = s.Scan()
			continue
		case ')':
			x.userString += string(r)
			// This is the end of the function
			return s.Scan(), nil
		case '$', '@':
			// This is a path
			if r, err = x.addOpToParamsAndParse(s, r); err != nil {
				return r, err
			}
			continue
		case '{':
			// This is a logical operation
			if r, err = x.addLogicalOperationToParamsAndParse(s, r); err != nil {
				return r, err
			}
			continue
		case sc.String, sc.RawString, sc.Char:
			tt := s.TokenText()
			x.userString += string(tt)

			if len(tt) >= 2 && strings.HasPrefix(tt, `"`) && strings.HasSuffix(tt, `"`) {
				tt = tt[1 : len(tt)-1]
			}
			x.Params = append(x.Params, &FP_String{tt})
		case sc.Float, sc.Int:
			// tt := s.TokenText()
			// x.userString += string(tt)

			// f, err := strconv.ParseFloat(tt, 64)
			// if err != nil {
			// 	// This should not be possible, but handle it just in case
			// 	return r, erAt(s, "couldn't convert number as string '%s' to number", s.TokenText())
			// }
			// x.Params = append(x.Params, &FP_Number{decimal.NewFromFloat(f)})
			r, err = dealWithNumbers(s, x, r)
			if err != nil {
				return r, erInvalid(s)
			}
		case sc.Ident:

			//must be bool
			tt := s.TokenText()
			switch tt {
			case "true":
				x.userString += tt
				x.Params = append(x.Params, &FP_Bool{true})
			case "false":
				x.userString += tt
				x.Params = append(x.Params, &FP_Bool{false})
			default:
				r, err = dealWithNumbers(s, x, r)
				if err != nil {
					return r, erInvalid(s)
				}
			}
		}
		r = s.Scan()
	}

	return
}

func dealWithNumbers(s *scanner, x *opFunction, r rune) (rune, error) {
	tt := s.TokenText()
	x.userString += string(tt)

	f, err := strconv.ParseFloat(tt, 64)
	if err != nil {
		// This should not be possible, but handle it just in case
		return r, erAt(s, "couldn't convert number as string '%s' to number", s.TokenText())
	}
	x.Params = append(x.Params, &FP_Number{decimal.NewFromFloat(f)})
	return r, nil
}

func (x *opFunction) addOpToParamsAndParse(s *scanner, r rune) (nextR rune, err error) {
	op := &opPath{}
	x.Params = append(x.Params, &FP_Path{op})
	nextR, err = op.Parse(s, r)
	x.userString += op.UserString()
	return
}

func (x *opFunction) addLogicalOperationToParamsAndParse(s *scanner, r rune) (nextR rune, err error) {
	op := &opLogicalOperation{}
	x.Params = append(x.Params, &FP_LogicalOperation{op})
	nextR, err = op.Parse(s, r)
	x.userString += op.UserString()
	return
}

type FunctionParameterTypes []FunctionParameterType

func (x FunctionParameterTypes) Numbers() (out []*FP_Number) {
	for _, fp := range x {
		switch t := fp.(type) {
		case *FP_Number:
			out = append(out, t)
		}
	}
	return
}

func (x FunctionParameterTypes) Strings() (out []*FP_String) {
	for _, fp := range x {
		switch t := fp.(type) {
		case *FP_String:
			out = append(out, t)
		}
	}
	return
}

func (x FunctionParameterTypes) Bools() (out []*FP_Bool) {
	for _, fp := range x {
		switch t := fp.(type) {
		case *FP_Bool:
			out = append(out, t)
		}
	}
	return
}

func (x FunctionParameterTypes) Paths() (out []*FP_Path) {
	for _, fp := range x {
		switch t := fp.(type) {
		case *FP_Path:
			out = append(out, t)
		}
	}
	return
}

type FunctionParameterType interface {
	IsFuncParam() (returns InputOrOutput)
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

func (x *FP_Number) IsFuncParam() (returns InputOrOutput) {
	return inputOrOutput(PT_Number, IOOT_Single)
}

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

func (x *FP_String) IsFuncParam() (returns InputOrOutput) {
	return inputOrOutput(PT_String, IOOT_Single)
}

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

func (x *FP_Bool) IsFuncParam() (returns InputOrOutput) {
	return inputOrOutput(PT_Boolean, IOOT_Single)
}

func (x *FP_Bool) GetValue() any { return x.Value }

func (x *FP_Bool) MarshalJSON() ([]byte, error) {
	return functionParameterMarshalJSON(x.Value, "Bool")
}

type FP_Path struct {
	Value *opPath
}

func (p FP_Path) String() string {
	return p.Value.UserString()
}

func (x *FP_Path) IsFuncParam() (returns InputOrOutput) {
	return inputOrOutput(PT_Any, IOOT_Single)
}

func (x *FP_Path) GetValue() any { return x.Value }

func (x *FP_Path) MarshalJSON() ([]byte, error) {
	return functionParameterMarshalJSON(x.Value, "Path")
}

type FP_LogicalOperation struct {
	Value *opLogicalOperation
}

func (p FP_LogicalOperation) String() string {
	return p.Value.UserString()
}

func (x *FP_LogicalOperation) IsFuncParam() (returns InputOrOutput) {
	return inputOrOutput(PT_Any, IOOT_Single)
}

func (x *FP_LogicalOperation) GetValue() any { return x.Value }

func (x *FP_LogicalOperation) MarshalJSON() ([]byte, error) {
	return functionParameterMarshalJSON(x.Value, "LogicalOperation")
}
