package mpath

import (
	"fmt"
	"strings"
	sc "text/scanner"

	"cuelang.org/go/cue"
	"github.com/pkg/errors"
)

type opPath struct {
	IsInvalid                bool
	StartAtRoot              bool
	IsFilter                 bool
	MustEndInFunctionOrIdent bool
	Operations               []Operation
	opCommon
}

func (x *opPath) Validate(rootValue cue.Value, cuePath CuePath, blockedRootFields []string) (path *Path, returnedType InputOrOutput) {
	errFunc := func(e error) (*Path, InputOrOutput) {
		if path == nil {
			path = &Path{}
		}

		path.String = x.UserString()
		path.Error = strPtr(e.Error())
		return path, returnedType
	}
	var err error

	var cuePathValue cue.Value
	if len(cuePath) == 0 || x.StartAtRoot {
		cuePathValue = rootValue
	} else {
		cuePathValue, err = findValueAtPath(rootValue, cuePath)
		if err != nil {
			return errFunc(err)
		}
	}

	rootPart := &PathIdent{
		pathIdentFields: pathIdentFields{
			Type: InputOrOutput{
				CueExpr: getExpr(rootValue),
			},
		},
	}

	path = &Path{
		pathFields: pathFields{
			Parts:    []CanBeAPart{rootPart},
			IsFilter: x.IsFilter,
			Type: InputOrOutput{
				CueExpr: getExpr(cuePathValue),
			},
		},
	}

	switch x.StartAtRoot {
	case true:
		rootPart.String = "$"
		rootPart.Type.Type = PT_Root
		rootPart.Type.IOType = IOOT_Single
		cuePath = CuePath{}
	case false:
		rootPart.String = "@"
		rootPart.Type.Type = PT_ElementRoot
		rootPart.Type.IOType = IOOT_Single
	}

	availableFields, err := getAvailableFieldsForValue(cuePathValue, blockedRootFields)
	if err != nil {
		return errFunc(fmt.Errorf("failed to list available fields from cue: %w", err))
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
	var previousWasFuncWithoutKnownReturn bool
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
					HasError: HasError{
						Error: &errMessage,
					},
				},
			}

			continue
		}

		switch t := op.(type) {
		case *opPathIdent:
			if previousWasFuncWithoutKnownReturn {
				// We cannot address into unknown return values, so we will return
				break
			}

			if returnedType.IOType == IOOT_Single && returnedType.Type.IsPrimitive() {
				shouldErrorRemaining = true
				errMessage := "cannot address into primitive value"
				path.Parts = append(path.Parts, &PathIdent{
					pathIdentFields: pathIdentFields{
						String: t.UserString(),
						HasError: HasError{
							Error: &errMessage,
						},
					},
				})
				continue
			}

			if returnedType.IOType == IOOT_Array {
				shouldErrorRemaining = true
				errMessage := "cannot address into array value"
				path.Parts = append(path.Parts, &PathIdent{
					pathIdentFields: pathIdentFields{
						String: t.UserString(),
						HasError: HasError{
							Error: &errMessage,
						},
					},
				})
				continue
			}

			if !foundFirstIdent {
				rdm[t.IdentName] = struct{}{}
				foundFirstIdent = true
				if strInStrSlice(t.IdentName, blockedRootFields) {
					errMessage := "field " + t.IdentName + " is not available"
					path.Parts = append(path.Parts, &PathIdent{
						pathIdentFields: pathIdentFields{
							String: t.UserString(),
							HasError: HasError{
								Error: &errMessage,
							},
						},
					})
					return
				}
			}

			cuePath = cuePath.Add(t.IdentName)

			// opPathIdent Validate advances the next value
			part, returnedType = t.Validate(rootValue, cuePath, blockedRootFields)
			path.Parts = append(path.Parts, part)
			part.(*PathIdent).Type = returnedType
			if part.HasErrors() {
				return
			}

		case *opFilter:
			pi, ok := part.(*PathIdent)
			if !ok {
				if part == nil {
					return errFunc(fmt.Errorf("tried to apply filter against wrong type"))
				}

				part.SetError(fmt.Sprintf("tried to apply filter against %T", part))
				continue
			}

			// opFilter Validate does not advance the next value
			pi.Filter = t.Validate(rootValue, cuePath, blockedRootFields)
			if pi.Filter.Error != nil {
				return errFunc(fmt.Errorf(*pi.Filter.Error))
			}

		case *opFunction:
			if part == nil {
				errMessage := "functions cannot be called here"
				path.Error = &errMessage
				continue
			}

			returnsKnownValues := false
			part, returnedType, returnsKnownValues, err = t.Validate(rootValue, cuePath, part.ReturnType(), blockedRootFields)
			if err != nil {
				shouldErrorRemaining = true
			}
			if !returnsKnownValues && (returnedType.Type == PT_Object) {
				previousWasFuncWithoutKnownReturn = true
			}
			path.Parts = append(path.Parts, part)
		}
	}

	if pl := len(path.Parts); pl > 0 {
		path.Type = path.Parts[pl-1].ReturnType()
	}

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
		var thisStr string
		if op.Type() != OT_Filter {
			thisStr = "."
		}
		thisStr += op.Sprint(depth)
		opStrings = append(opStrings, thisStr)
	}

	if len(opStrings) > 0 {
		out += strings.Join(opStrings, "")
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
				// x.userString += string(r)
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

				// return r, erAt(s, "paths that are part of a logical operation must end in a boolean function")
				x.IsInvalid = true
				return r, nil
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
			// x.userString += string(r)
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
