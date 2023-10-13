package mpath

import (
	"fmt"
	"sort"
	sc "text/scanner"

	"cuelang.org/go/cue"
)

type opLogicalOperation struct {
	IsFilter             bool
	LogicalOperationType LOT_LogicalOperationType
	Operations           []Operation
	opCommon
}

func (x *opLogicalOperation) Validate(rootValue, nextValue cue.Value, blockedRootFields []string) (logicalOperation *TypeaheadLogicalOperation, requiredData []string, err error) {
	logicalOperation = &TypeaheadLogicalOperation{
		typeaheadLogicalOperationFields: typeaheadLogicalOperationFields{
			String:          x.UserString(),
			LogicalOperator: &x.LogicalOperationType,
		},
	}

	rdMap := map[string]struct{}{}

	for _, op := range x.Operations {
		switch t := op.(type) {
		case *opPath:
			operation := &TypeaheadConfig{
				typeaheadConfigFields: typeaheadConfigFields{
					String: t.UserString(),
				},
			}
			logicalOperation.Parts = append(logicalOperation.Parts, operation)
			var rd []string
			operation.Parts, operation.Type, rd, err = t.Validate(rootValue, nextValue, blockedRootFields)
			if err != nil {
				errMessage := err.Error()
				operation.Error = &errMessage
			}
			for _, rdv := range rd {
				rdMap[rdv] = struct{}{}
			}

			// We need to check that the return type is boolean
			if opLen := len(operation.Parts); opLen > 0 {
				if operation.Parts[opLen-1].ReturnType() != PT_Boolean {
					errMessage := "paths that are part of a logical operation must end in a boolean function"
					operation.Error = &errMessage
				}
			}

		case *opLogicalOperation:
			subLogicalOperation, rd, err := t.Validate(rootValue, nextValue, blockedRootFields)
			if err != nil {
				errMessage := err.Error()
				subLogicalOperation.Error = &errMessage
				continue
			}
			for _, rdv := range rd {
				rdMap[rdv] = struct{}{}
			}

			logicalOperation.Parts = append(logicalOperation.Parts, subLogicalOperation)
		}
	}

	for rd := range rdMap {
		requiredData = append(requiredData, rd)
	}
	sort.Strings(requiredData)

	return
}

func (x *opLogicalOperation) addOpToOperationsAndParse(op Operation, s *scanner, r rune) (nextR rune, err error) {
	x.Operations = append(x.Operations, op)
	nextR, err = op.Parse(s, r)
	x.userString += op.UserString()
	return
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
	x.userString += string(r)
	r = s.Scan()

	tokenText := s.TokenText()
	if r == sc.Ident && (tokenText == "AND" || tokenText == "OR") {
		x.userString += tokenText
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
			x.userString += string(r)
			// This is the separator, we can move on
			r = s.Scan()
			continue

		case '$', '@':
			// This is an opPath
			op = &opPath{MustEndInFunctionOrIdent: true, IsFilter: x.IsFilter}
		case '{':
			// This is an opLogicalOperation
			op = &opLogicalOperation{}
		case '}', ']':
			x.userString += string(r)

			// This is the end of this logical operation
			r = s.Scan()

			return r, nil
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
