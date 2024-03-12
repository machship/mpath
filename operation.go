package mpath

import (
	"reflect"
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

func AddressedPaths(op Operation) (addressedPaths [][]string) {
	// check stuff
	switch v := op.(type) {
	case *opPath:
		if len(v.Operations) < 1 {
			break
		}

		idents := []string{}

		for _, subOp := range v.Operations {
			switch vv := subOp.(type) {
			case *opPathIdent:
				idents = append(idents, vv.IdentName)

			case *opFilter:
				for _, logOp := range vv.LogicalOperation.Operations {
					for _, val := range AddressedPaths(logOp) {
						addressedPaths = append(addressedPaths, append(idents, val...))
					}
				}

			case *opFunction:
				for _, p := range vv.Params.Paths() {
					for _, val := range AddressedPaths(p.Value) {
						addressedPaths = append(addressedPaths, val)
					}
				}
			}
		}

		addressedPaths = append(addressedPaths, idents)

	case *opLogicalOperation:
		for _, p := range v.Operations {
			for _, val := range AddressedPaths(p) {
				addressedPaths = append(addressedPaths, val)
			}
		}
	}

	retAddressedPaths := make([][]string, 0, len(addressedPaths))

	for _, val := range addressedPaths {
		if !sliceContains(retAddressedPaths, val) && !slicesContainsSubsetSlice(retAddressedPaths, val) && len(val) > 0 {
			retAddressedPaths = append(retAddressedPaths, val)
		}
	}

	return retAddressedPaths
}

// Checks if a given value is present in a slice.
// If a match is found, it returns true. Otherwise, it returns false.
func sliceContains[T any](sl []T, val T) bool {
	for _, v := range sl {
		if reflect.DeepEqual(v, val) {
			return true
		}
	}

	return false
}

// Takes a slice of any type and returns a 2D slice where each element
// is a prefix of the original slice.
// E.g.
// Input: ["one", "two", "three", "four"]
// Output: [["one"], ["one", "two"], ["one", "two", "three"], ["one", "two", "three", "four"]]
func spreadSlice[T any](sl []T) (ret [][]T) {
	ret = make([][]T, len(sl))
	for i := range sl {
		ret[i] = sl[:i+1]
	}
	return ret
}

// Checks if any of the slices in the given 2D slice contains the given subset slice.
// It returns true if a match is found, and false otherwise.
func slicesContainsSubsetSlice[T any](slices [][]T, subset []T) bool {
	for _, slice := range slices {
		sliceSubsets := spreadSlice(slice)

		for _, ss := range sliceSubsets {
			if reflect.DeepEqual(ss, subset) {
				return true
			}
		}
	}

	return false
}
