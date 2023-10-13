package mpath

import (
	"encoding/json"
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/google/uuid"
)

// query: the mpath query string
// cueFile: the cue file
// currentPath: the id of the step for which this query is an input value, or if for the output, leave blank
func CueValidate(query, cueFile, currentPath string) (tc CanBeAPart, rdm *RuntimeDataMap, err error) {
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
	var rootValue cue.Value
	if rootValue, ok = cueValueCache[cueFile]; !ok {
		ctx := cuecontext.New()
		rootValue = ctx.CompileString(cueFile)
		if rootValue.Err() != nil {
			return nil, nil, fmt.Errorf("failed to parse cue file: %w", rootValue.Err())
		}
	}

	blockedRootFields, err := getBlockedRootFields(rootValue, currentPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get blocked fields for rootValue: %w", err)
	}

	// If we get to this point, the mpath query and cueFile are both valid,
	// thus the next steps are to walk through the "paths" in the returned AST
	// and doubt check that they are valid given the cueFile.

	// We will "walk" through the AST and build a TypeaheadConfig as we go
	// NB: topOp can only be an opPath or opLogicalOperation

	var requiredData []string
	switch t := op.(type) {
	case *opPath:
		ptc := &TypeaheadPath{
			typeaheadPathFields: typeaheadPathFields{
				String: query,
			},
		}

		parts, typ, rd, subErr := t.Validate(rootValue, rootValue, blockedRootFields)
		if subErr != nil {
			err = subErr
			return
		}

		requiredData = rd
		ptc.Type = typ
		ptc.Parts = append(ptc.Parts, parts...)
		tc = ptc

	case *opLogicalOperation:
		tc = &TypeaheadPath{
			typeaheadPathFields: typeaheadPathFields{
				String: query,
			},
		}

		logicalOperation, subRequiredData, subErr := t.Validate(rootValue, rootValue, blockedRootFields)
		if err != nil {
			err = subErr
			return
		}
		requiredData = subRequiredData

		tc = logicalOperation
	}

	rdm = &RuntimeDataMap{
		String:       query,
		RequiredData: requiredData,
	}

	return
}

func findValuePath(inputValue cue.Value, name string) (outputValue cue.Value, err error) {
	var selector cue.Selector
	switch strings.HasPrefix(name, "_") {
	case true:
		selector = cue.Hid(name, "_")
	case false:
		selector = cue.Str(name)
	}

	if inputValue.IncompleteKind() == cue.ListKind {
		it, err := inputValue.List()
		if err != nil {
			return outputValue, fmt.Errorf("couldn't get list iterator for list kind")
		}
		it.Next()
		inputValue = it.Value()
	}

	outputValue = inputValue.LookupPath(cue.MakePath(selector))
	if outputValue.Err() != nil {
		return outputValue, fmt.Errorf("unknown field '%s'", name)
	}

	return
}

func getUnderlyingKind(v cue.Value) (kind cue.Kind, err error) {
	if v.IncompleteKind() == cue.ListKind {
		it, err := v.List()
		if err != nil {
			return kind, fmt.Errorf("couldn't get list iterator for list kind")
		}
		it.Next()
		v = it.Value()
	}

	return v.IncompleteKind(), nil
}

func getAvailableFieldsForValue(v cue.Value) (fields []string, err error) {
	if v.IncompleteKind() == cue.ListKind {
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

type BP_BasePath string

const (
	BP_Dependencies BP_BasePath = "_dependencies"
	BP_Input        BP_BasePath = "_input"
	BP_Variables    BP_BasePath = "_variables"
)

func getConcreteValuesForListOfStringValueAtPath(inputValue cue.Value, path string) (output []string, err error) {
	foundValue, err := findValuePath(inputValue, path)
	if err != nil {
		return nil, fmt.Errorf("failed to find output in cue value: %w", err)
	}

	if dk := foundValue.Kind(); dk != cue.ListKind {
		return nil, fmt.Errorf("output was of the wrong kind: wanted List")
	}

	uk, err := getUnderlyingKind(foundValue)
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying kind of output: %w", err)
	}

	if uk != cue.StringKind {
		if uk == cue.BottomKind {
			return
		}

		return nil, fmt.Errorf("output was of the wrong underlying kind: wanted String")
	}

	it, err := foundValue.List()
	if err != nil {
		return nil, fmt.Errorf("failed to access list of strings as cue List type: %w", err)
	}

	for it.Next() {
		thisString, err := it.Value().String()
		if err != nil {
			return nil, fmt.Errorf("failed to access String for cue value: %w", err)
		}

		output = append(output, thisString)
	}

	return
}

func getBlockedRootFields(rootValue cue.Value, currentPath string) (blockedFields []string, err error) {
	// Find the current path:
	nextValue, err := findValuePath(rootValue, currentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find currentPath in cue value: %w", err)
	}

	// We need to recursively get the dependencies of the currentPath
	dependencies, err := getConcreteValuesForListOfStringValueAtPath(nextValue, string(BP_Dependencies))
	if err != nil {
		return nil, fmt.Errorf("failed to find dependencies in cue value: %w", err)
	}

	validFields := map[string]struct{}{
		currentPath:          {},
		string(BP_Input):     {},
		string(BP_Variables): {},
	}
	for _, dep := range dependencies {
		validFields[dep] = struct{}{}
	}

	var nextDependencies []string
loop:
	for _, d := range dependencies {
		nextValue, err = findValuePath(rootValue, d)
		if err != nil {
			return nil, fmt.Errorf("failed to find dependency '%s' in cue value: %w", d, err)
		}

		nextDependencies, err = getConcreteValuesForListOfStringValueAtPath(nextValue, string(BP_Dependencies))
		if err != nil {
			return nil, fmt.Errorf("failed to find nextDependencies in cue value: %w", err)
		}

		for _, dep := range nextDependencies {
			validFields[dep] = struct{}{}
		}
	}
	if len(nextDependencies) > 0 {
		dependencies = nextDependencies
		goto loop
	}

	allFields, err := getAvailableFieldsForValue(rootValue)
	if err != nil {
		return nil, fmt.Errorf("failed to list fields in cue value: %w", err)
	}

	for _, fieldName := range allFields {
		if _, ok := validFields[fieldName]; !ok {
			blockedFields = append(blockedFields, fieldName)
		}
	}

	return
}

type CanBeAPart interface {
	CanBeAPart()
	PartType() string
	MarshalJSON() ([]byte, error)
	ReturnType() PT_ParameterType
}

type typeaheadPathFields struct {
	String string           `json:"string"`
	Type   PT_ParameterType `json:"type"`
	Error  *string          `json:"error,omitempty"`
	Parts  []CanBeAPart     `json:"parts,omitempty"`
}

type TypeaheadPath struct {
	typeaheadPathFields
}

func (x *TypeaheadPath) CanBeAPart() {}
func (x *TypeaheadPath) ReturnType() PT_ParameterType {
	return x.Type
}
func (x *TypeaheadPath) PartType() string {
	return "TypeaheadPath"
}
func (x *TypeaheadPath) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID       string `json:"id"`
		PartType string `json:"partType"`
		typeaheadPathFields
	}{
		ID:                  uuid.New().String(),
		PartType:            x.PartType(),
		typeaheadPathFields: x.typeaheadPathFields,
	})
}

type typeaheadPathIdentFields struct {
	String          string              `json:"string"`
	Error           *string             `json:"error,omitempty"`
	Type            PT_ParameterType    `json:"type"`
	Available       *TypeaheadAvailable `json:"available,omitempty"`
	TypeaheadFilter *TypeaheadFilter    `json:"typeaheadFilter,omitempty"`
}

type TypeaheadPathIdent struct {
	typeaheadPathIdentFields
}

func (x *TypeaheadPathIdent) CanBeAPart() {}
func (x *TypeaheadPathIdent) ReturnType() PT_ParameterType {
	return x.Type
}
func (x *TypeaheadPathIdent) PartType() string {
	return "TypeaheadPathIdent"
}
func (x *TypeaheadPathIdent) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID       string `json:"id"`
		PartType string `json:"partType"`
		typeaheadPathIdentFields
	}{
		ID:                       uuid.New().String(),
		PartType:                 x.PartType(),
		typeaheadPathIdentFields: x.typeaheadPathIdentFields,
	})
}

type typeaheadFunctionFields struct {
	String              string                `json:"string"`
	Error               *string               `json:"error,omitempty"`
	Type                PT_ParameterType      `json:"type"`
	Available           *TypeaheadAvailable   `json:"available,omitempty"`
	FunctionName        *string               `json:"functionName,omitempty"`
	FunctionExplanation *string               `json:"functionExplanation,omitempty"`
	FunctionParameters  []*TypeaheadParameter `json:"functionParameters,omitempty"`
}

type TypeaheadFunction struct {
	typeaheadFunctionFields
}

func (x *TypeaheadFunction) CanBeAPart() {}
func (x *TypeaheadFunction) ReturnType() PT_ParameterType {
	return x.Type
}
func (x *TypeaheadFunction) PartType() string {
	return "TypeaheadFunction"
}
func (x *TypeaheadFunction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID       string `json:"id"`
		PartType string `json:"partType"`
		typeaheadFunctionFields
	}{
		ID:                      uuid.New().String(),
		PartType:                x.PartType(),
		typeaheadFunctionFields: x.typeaheadFunctionFields,
	})
}

type TypeaheadAvailable struct {
	Fields    []string `json:"fields,omitempty"`
	Functions []string `json:"functions,omitempty"`
	Filters   []string `json:"filters,omitempty"`
}

type TypeaheadFilter struct {
	String           string                     `json:"string"`
	Error            *string                    `json:"error,omitempty"`
	LogicalOperation *TypeaheadLogicalOperation `json:"logicalOperation,omitempty"`
}

type typeaheadLogicalOperationFields struct {
	String          string                    `json:"string"`
	Error           *string                   `json:"error,omitempty"`
	LogicalOperator *LOT_LogicalOperationType `json:"logicalOperator,omitempty"`
	Parts           []CanBeAPart              `json:"parts,omitempty"`
}

type TypeaheadLogicalOperation struct {
	typeaheadLogicalOperationFields
}

func (x *TypeaheadLogicalOperation) CanBeAPart() {}
func (x *TypeaheadLogicalOperation) ReturnType() PT_ParameterType {
	return PT_Boolean
}
func (x *TypeaheadLogicalOperation) PartType() string {
	return "TypeaheadLogicalOperation"
}
func (x *TypeaheadLogicalOperation) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID       string `json:"id"`
		PartType string `json:"partType"`
		typeaheadLogicalOperationFields
	}{
		ID:                              uuid.New().String(),
		PartType:                        x.PartType(),
		typeaheadLogicalOperationFields: x.typeaheadLogicalOperationFields,
	})
}

type TypeaheadParameter struct {
	String    string              `json:"string"`
	Type      PT_ParameterType    `json:"type"`
	Error     *string             `json:"error,omitempty"`
	Parts     []CanBeAPart        `json:"parts,omitempty"`
	Available *TypeaheadAvailable `json:"available,omitempty"`
}
