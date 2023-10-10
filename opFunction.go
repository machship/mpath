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
	FunctionType FT_FunctionType

	Params FunctionParameters
}

func (x *opFunction) Validate(rootValue, inputValue cue.Value) (part *TypeaheadPart, nextValue cue.Value, returnedType PT_ParameterType, requiredData []string, err error) {
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

	rdm := map[string]struct{}{}

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
			var rd []string
			param.Parts, returnedType, rd, err = t.Value.Validate(rootValue, nextValue)
			if err != nil {
				errMessage := err.Error()
				param.Error = &errMessage
				continue
			}
			for _, rdv := range rd {
				rdm[rdv] = struct{}{}
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

	for rdv := range rdm {
		requiredData = append(requiredData, rdv)
	}
	sort.Strings(requiredData)

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
