package mpath

import (
	"encoding/json"
	"fmt"
	sc "text/scanner"

	"cuelang.org/go/cue"
)

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
