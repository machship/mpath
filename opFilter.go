package mpath

import (
	"fmt"

	"cuelang.org/go/cue"
)

type opFilter struct {
	LogicalOperation *opLogicalOperation
	opCommon
}

func (x *opFilter) Validate(rootValue cue.Value, cuePath CuePath, blockedRootFields []string) (filter *Filter) {
	cuePathValue, err := findValueAtPath(rootValue, cuePath)
	if err != nil {
		return &Filter{
			String: x.UserString(),
			Error:  strPtr(err.Error()),
		}
	}

	filter = &Filter{}
	if cuePathValue.IncompleteKind() != cue.ListKind {
		errMessage := fmt.Sprintf("not a list (was %s); only lists can be filtered", cuePathValue.Kind())
		filter.Error = &errMessage
		return
	}

	filter.LogicalOperation = x.LogicalOperation.Validate(rootValue, cuePath, blockedRootFields)
	if filter.LogicalOperation.Error != nil {
		filter.Error = filter.LogicalOperation.Error
	}

	return
}

func (x *opFilter) Type() OT_OpType { return OT_Filter }

func (x *opFilter) Sprint(depth int) (out string) {
	return x.LogicalOperation.Sprint(depth)
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
	nextR, err = x.LogicalOperation.Parse(s, r)
	if x.LogicalOperation != nil {
		x.userString += x.LogicalOperation.UserString()
	}
	return
}
