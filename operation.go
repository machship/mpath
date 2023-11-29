package mpath

import (
	"sort"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////

type Operation interface {
	Do(currentData, originalData any) (dataToUse any, err error)
	Parse(s *scanner, r rune) (nextR rune, err error)
	Sprint(depth int) (out string)
	Type() OT_OpType
	UserString() string
}

type opCommon struct {
	userString string
}

func (x opCommon) UserString() string {
	return x.userString
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

func GetRootFieldsAccessed(op Operation) (rootFieldsAccessed []string) {
	accessed := map[string]struct{}{}

	switch t := op.(type) {
	case *opPath:
		thisPath := []string{}
		haveSeenIdent := false
		for _, pop := range t.Operations {
			switch ot := pop.(type) {
			case *opPathIdent:
				if !haveSeenIdent && !t.IsFilter {
					haveSeenIdent = true
					thisPath = append(thisPath, ot.IdentName)
				}
			case *opFilter:
				for _, logOp := range ot.LogicalOperation.Operations {
					for _, val := range GetRootFieldsAccessed(logOp) {
						accessed[val] = struct{}{}
					}
				}
			case *opFunction:
				for _, param := range ot.Params.Paths() {
					for _, val := range GetRootFieldsAccessed(param.Value) {
						accessed[val] = struct{}{}
					}
				}
			}
		}

		if len(thisPath) > 0 {
			accessed[strings.Join(thisPath, ".")] = struct{}{}
		}

	case *opLogicalOperation:
		for _, p := range t.Operations {
			for _, val := range GetRootFieldsAccessed(p) {
				accessed[val] = struct{}{}
			}
		}
	}

	for acc := range accessed {
		rootFieldsAccessed = append(rootFieldsAccessed, acc)
	}

	sort.Strings(rootFieldsAccessed)

	return
}
