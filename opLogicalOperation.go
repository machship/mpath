package mpath

import (
	"fmt"
	"sort"
	sc "text/scanner"

	"cuelang.org/go/cue"
)

type opLogicalOperation struct {
	IsInvalid            bool
	IsFilter             bool
	LogicalOperationType LOT_LogicalOperationType
	Operations           []Operation
	opCommon
}

func (x *opLogicalOperation) Validate(rootValue, nextValue cue.Value, blockedRootFields []string) (logicalOperation *LogicalOperation, requiredData []string, err error) {
	logicalOperation = &LogicalOperation{
		logicalOperationFields: logicalOperationFields{
			String:          x.UserString(),
			LogicalOperator: &x.LogicalOperationType,
			IsFilter:        x.IsFilter,
		},
	}

	if x.IsInvalid {
		errMessage := fmt.Sprintf("invalid operation type '%s'", x.LogicalOperationType)
		logicalOperation.Error = &errMessage
	}

	rdMap := map[string]struct{}{}

	for _, op := range x.Operations {
		switch t := op.(type) {
		case *opPath:
			var pathOp *Path
			var rd []string
			pathOp, _, rd, err = t.Validate(rootValue, nextValue, blockedRootFields)

			pathOp.String = t.UserString()
			if err != nil {
				errMessage := err.Error()
				pathOp.Error = &errMessage
			}

			for _, rdv := range rd {
				rdMap[rdv] = struct{}{}
			}

			// We need to check that the return type is boolean
			if opLen := len(pathOp.Parts); opLen > 0 {
				if rt := pathOp.Parts[opLen-1].ReturnType(); !(rt.Type == PT_Boolean && rt.IOType == IOOT_Single) {
					errMessage := "paths that are part of a logical operation must end in a boolean function that returns a single value"
					pathOp.Error = &errMessage
				}
			}
			logicalOperation.Parts = append(logicalOperation.Parts, pathOp)

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

	for i, op := range x.Operations {
		out += "\n" + op.Sprint(depth+1)
		if i != len(x.Operations)-1 {
			out += ","
		}
	}

	out += "\n" + repeatTabs(depth) + endChar

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
	} else if r == sc.Ident {
		// This is a misspelt operation type
		x.userString += tokenText
		x.IsInvalid = true
		x.LogicalOperationType = LOT_LogicalOperationType(tokenText)
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
