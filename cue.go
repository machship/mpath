package mpath

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func findValuePath(inputValue cue.Value, name string) (outputValue cue.Value, err error) {
	var selector cue.Selector
	switch strings.HasPrefix(name, "_") {
	case true:
		selector = cue.Hid(name, "_")
	case false:
		selector = cue.Str(name)
	}

	outputValue = inputValue.LookupPath(cue.MakePath(selector))
	if outputValue.Err() != nil {
		return outputValue, fmt.Errorf("unknown field '%s'", name)
	}

	return
}

func getUnderlyingKind(v cue.Value) (kind cue.Kind, err error) {
	if v.Kind() == cue.ListKind {
		it, err := v.List()
		if err != nil {
			return kind, fmt.Errorf("couldn't get list iterator for list kind")
		}
		it.Next()
		v = it.Value()
	}

	return v.Kind(), nil
}

func getAvailableFieldsForValue(v cue.Value) (fields []string, err error) {
	if v.Kind() == cue.ListKind {
		it, err := v.List()
		if err != nil {
			return nil, fmt.Errorf("couldn't get list iterator for list kind")
		}
		it.Next()
		v = it.Value()
	}

	it, err := v.Fields(cue.All())
	if err != nil {
		return nil, fmt.Errorf("failed to list fields: %w", err)
	}

	for it.Next() {
		fldName := it.Selector().String()

		switch fldName {
		case "_dependencies":
			continue
		}

		fields = append(fields, fldName)
	}

	return
}

var (
	mpathOpCache  = map[string]Operation{}
	cueValueCache = map[string]cue.Value{}
)

type RuntimeDataMap struct {
	String       string
	RequiredData []string
}

// query: the mpath query string
// cueFile: the cue file
// currentPath: the id of the step for which this query is an input value, or if for the output, leave blank
func CueValidate(query string, cueFile string, currentPath string) (tc *TypeaheadConfig, rdm *RuntimeDataMap, err error) {
	var ok bool

	// mpath operations are cached to ensure speed of execution as this method is expected to be hit many times
	var op Operation
	if op, ok = mpathOpCache[query]; !ok {
		op, _, err = ParseString(query)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse mpath query: %w", err)
		}
	}

	// cue values are cached to ensure speed of execution as this method is expected to be hit many times
	var v cue.Value
	if v, ok = cueValueCache[cueFile]; !ok {
		ctx := cuecontext.New()
		v = ctx.CompileString(cueFile)
		if v.Err() != nil {
			return nil, nil, fmt.Errorf("failed to parse cue file: %w", v.Err())
		}
	}

	// If we get to this point, the mpath query and cueFile are both valid,
	// thus the next steps are to walk through the "paths" in the returned AST
	// and doubt check that they are valid given the cueFile.

	// We will "walk" through the AST and build a TypeaheadConfig as we go
	// NB: topOp can only be an opPath or opLogicalOperation

	var requiredData []string
	switch t := op.(type) {
	case *opPath:
		tc = &TypeaheadConfig{
			String: op.Sprint(0), // todo: is this correct?
		}

		tc.Parts, tc.Type, requiredData, err = t.Validate(v, v)
		if err != nil {
			errMessage := err.Error()
			tc.Error = &errMessage
		}

	case *opLogicalOperation:
		tc = &TypeaheadConfig{
			String: op.Sprint(0), // todo: is this correct?
		}

		var err error
		subOperator, subOperations, subRequiredData, err := t.Validate(v, v)
		if err != nil {
			errMessage := err.Error()
			tc.Error = &errMessage
		}
		requiredData = subRequiredData

		tc.Parts = append(tc.Parts, &TypeaheadPart{
			String:            op.Sprint(0),
			Type:              PT_Boolean,
			LogicalOperator:   subOperator,
			LogicalOperations: subOperations,
		})
	}

	rdm = &RuntimeDataMap{
		String:       tc.String,
		RequiredData: requiredData,
	}

	return
}

type TypeaheadConfig struct {
	String string           `json:"string"`
	Parts  []*TypeaheadPart `json:"parts,omitempty"`
	Type   PT_ParameterType `json:"type"`
	Error  *string          `json:"error,omitempty"`
}

type TypeaheadPart struct {
	String            string                    `json:"string"`
	Error             *string                   `json:"error,omitempty"`
	FunctionName      *string                   `json:"functionName,omitempty"`
	Type              PT_ParameterType          `json:"type"`
	Available         *TypeaheadAvailable       `json:"available,omitempty"`
	Filter            *TypeaheadFilter          `json:"filter,omitempty"`
	Parameters        []*TypeaheadParameter     `json:"parameters,omitempty"`
	LogicalOperator   *LOT_LogicalOperationType `json:"logicalOperationType,omitempty"`
	LogicalOperations []*TypeaheadConfig        `json:"logicalOperations,omitempty"`
}

type TypeaheadAvailable struct {
	Fields    []string `json:"fields,omitempty"`
	Functions []string `json:"functions,omitempty"`
	Filters   []string `json:"filters,omitempty"`
}

type TypeaheadFilter struct {
	String            string                    `json:"string"`
	Error             *string                   `json:"error,omitempty"`
	LogicalOperator   *LOT_LogicalOperationType `json:"logicalOperationType,omitempty"`
	LogicalOperations []*TypeaheadConfig        `json:"logicalOperations,omitempty"`
}

type TypeaheadParameter struct {
	String    string              `json:"string"`
	Error     *string             `json:"error,omitempty"`
	Parts     []*TypeaheadPart    `json:"parts,omitempty"`
	Available *TypeaheadAvailable `json:"available,omitempty"`
}

/*
	todo:
			- 	Need to recognise structure of OpenAPI 3.0 structure
				and build AST style model to compare mpath query to, or
				we need to use cue directly to query the structure

			-	If mpath gets to an unrecognised part of the path (not including an errored function name),
				we need to return suggestions for available options based on the current point in the
				structure, along with valid available functions. This will effectively provide
				a typeahead structure.


			-	mpath method:

			// query: the mpath query string
			// cueFile: the cue file
			// currentPath: the id of the step for which this query is an input value, or if for the output, leave blank
			func validateForTypeahead(query string, cueFile string, currentPath string) (tc TypeaheadConfig, err error) {
				...
			}

			type TypeaheadConfig struct {
				...
			}

			func validateForRuntime(query string, cueFile string, currentPath string) (rdm RuntimeDataMap, err error) {
				//
				...
			}
*/
