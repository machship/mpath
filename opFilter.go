package mpath

import (
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
)

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
