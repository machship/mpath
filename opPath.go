package mpath

import (
	"fmt"
	"sort"
	"strings"
	sc "text/scanner"

	"cuelang.org/go/cue"
	"github.com/pkg/errors"
)

type opPath struct {
	StartAtRoot              bool
	IsFilter                 bool
	MustEndInFunctionOrIdent bool
	Operations               []Operation
	opCommon
}

func (x *opPath) Validate(rootValue, nextValue cue.Value, blockedRootFields []string) (parts []CanBeAPart, returnedType PT_ParameterType, requiredData []string, err error) {
	rootPart := &PathIdent{}

	parts = []CanBeAPart{rootPart}

	switch x.StartAtRoot {
	case true:
		rootPart.String = "$"
		rootPart.Type = PT_Root
	case false:
		rootPart.String = "@"
		rootPart.Type = PT_ElementRoot
	}

	availableFields, err := getAvailableFieldsForValue(nextValue)
	if err != nil {
		return nil, returnedType, nil, fmt.Errorf("failed to list available fields from cue: %w", err)
	}

	if len(availableFields) > 0 {
		rootPart.Available = &Available{
			Fields: availableFields,
		}
	}

	rdm := map[string]struct{}{}

	var shouldErrorRemaining bool
	var part CanBeAPart
	var foundFirstIdent bool
	for _, op := range x.Operations {
		if shouldErrorRemaining {
			var str string
			switch t := op.(type) {
			case *opPathIdent:
				str = t.UserString()
			case *opFilter:
				str = t.UserString()
			default:
				continue
			}
			errMessage := "cannot continue due to previous error"
			part = &PathIdent{
				pathIdentFields: pathIdentFields{
					String: str,
					Error:  &errMessage,
				},
			}

			continue
		}

		var rd []string
		switch t := op.(type) {
		case *opPathIdent:
			if returnedType.IsPrimitive() {
				shouldErrorRemaining = true
				errMessage := "cannot address into primitive value"
				parts = append(parts, &PathIdent{
					pathIdentFields: pathIdentFields{
						String: t.UserString(),
						Error:  &errMessage,
					},
				})
				continue
			}

			if returnedType.IsArray() {
				shouldErrorRemaining = true
				errMessage := "cannot address into array value"
				parts = append(parts, &PathIdent{
					pathIdentFields: pathIdentFields{
						String: t.UserString(),
						Error:  &errMessage,
					},
				})
				continue
			}

			if !foundFirstIdent {
				rdm[t.IdentName] = struct{}{}
				foundFirstIdent = true
				for _, brf := range blockedRootFields {
					if t.IdentName == brf {
						errMessage := fmt.Sprintf("field %s is not available", t.IdentName)
						parts = append(parts, &PathIdent{
							pathIdentFields: pathIdentFields{
								String: t.UserString(),
								Error:  &errMessage,
							},
						})
						continue
					}
				}
			}

			thisKind := nextValue.IncompleteKind()
			if thisKind == cue.ListKind {
				var it cue.Iterator
				it, err = nextValue.List()
				if err != nil {
					err = fmt.Errorf("couldn't get list iterator for list kind")
					return
				}
				it.Next()
				nextValue = it.Value()
			}

			// opPathIdent Validate advances the next value
			part, nextValue, returnedType, err = t.Validate(nextValue)
			if err != nil {
				return nil, returnedType, nil, err
			}
			parts = append(parts, part)
			part.(*PathIdent).Type = returnedType

		case *opFilter:
			// opFilter Validate does not advance the next value
			part.(*PathIdent).Filter, rd, err = t.Validate(rootValue, nextValue, blockedRootFields)
			if err != nil {
				return nil, returnedType, nil, err
			}

		case *opFunction:
			part, returnedType, rd, err = t.Validate(rootValue, nextValue, blockedRootFields)
			if err != nil {
				shouldErrorRemaining = true
			}
			parts = append(parts, part)
		}
		for _, rdv := range rd {
			rdm[rdv] = struct{}{}
		}
	}

	for rdv := range rdm {
		requiredData = append(requiredData, rdv)
	}
	sort.Strings(requiredData)

	return
}

func (x *opPath) addOpToOperationsAndParse(op Operation, s *scanner, r rune) (nextR rune, err error) {
	x.Operations = append(x.Operations, op)
	nextR, err = op.Parse(s, r)
	x.userString += op.UserString()
	return
}

func (x *opPath) Type() OT_OpType { return OT_Path }

func (x *opPath) Sprint(depth int) (out string) {

	out += repeatTabs(depth)

	switch x.StartAtRoot {
	case true:
		out += "$"
	case false:
		out += "@"
	}

	opStrings := []string{}

	for _, op := range x.Operations {
		opStrings = append(opStrings, op.Sprint(depth))
	}

	if len(opStrings) > 0 {
		out += "." + strings.Join(opStrings, ".")
	}

	return
}

func (x *opPath) ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool) {
	outCurrent = current

	for _, op := range x.Operations {
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
	if x.StartAtRoot && x.IsFilter {
		return nil, fmt.Errorf("cannot access root data in filter")
	}

	if x.StartAtRoot {
		dataToUse = originalData
	} else {
		dataToUse = currentData
	}

	if len(x.Operations) == 0 {
		// This is a special case where the root is being returned

		// As we always guarantee numbers are returned as the decimal type, we do this check
		if _, ok := dataToUse.(string); !ok {
			dataToUse = convertToDecimalIfNumber(dataToUse)
		}
	}

	// Now we know which data to use, we can apply the path parts
	for _, op := range x.Operations {
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
		if x.IsFilter {
			return r, errors.Wrap(erInvalid(s, '@'), "cannot use '$' (root) inside filter")
		}
		x.StartAtRoot = true
	case '@':
		// do nothing, this is the default
	default:
		return r, erInvalid(s, '$', '@')
	}
	x.userString += string(r)

	r = s.Scan()

	var op Operation
	for { //i := 1; i > 0; i++ {
		if r == sc.EOF {
			break
		}

		switch r {
		case '.':
			x.userString += string(r)
			// This is the separator, we can move on
			r = s.Scan()
			continue

		case ',', ')', ']', '}':
			switch r {
			case ',':
				x.userString += string(r)
			case ')':
				// do nothing?
			}

			// This should mean we are finished the path
			if x.MustEndInFunctionOrIdent {
				if len(x.Operations) > 0 {
					if pf, ok := x.Operations[len(x.Operations)-1].(*opFunction); x.Operations[len(x.Operations)-1].Type() == OT_Function && ok {
						if ft_IsBoolFunc(pf.FunctionType) {
							return r, nil
						}
					}
					if _, ok := x.Operations[len(x.Operations)-1].(*opPathIdent); ok {
						// we can assume that the user has provided a boolean property
						return r, nil
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
			x.userString += string(r)
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
