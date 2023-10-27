package mpath

import (
	"encoding/json"
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/google/uuid"
)

func ListFunctions() (funcs map[FT_FunctionType]FunctionDescriptor) {
	return funcMap
}

type CanBeAPart interface {
	PathString() string
	PartType() string
	MarshalJSON() ([]byte, error)
	ReturnType() InputOrOutput
	HasErrors() bool
	SetError(errMessage string)
}

type HasError struct {
	Error *string `json:"error,omitempty"`
}

func (x *HasError) SetError(errMessage string) {
	if x.Error != nil {
		errMessage = *x.Error + "; " + errMessage
	}

	x.Error = &errMessage
}

type RuntimeDataMap struct {
	String       string   `json:"string"`
	HasErrors    bool     `json:"hasErrors"`
	RequiredData []string `json:"requiredData"`
}

// query: the mpath query string
// cueFile: the cue file
// currentPath: the id of the step for which this query is an input value, or if for the output, leave blank
func CueValidate(query, cueFile, currentPath string) (tc CanBeAPart, rdm *RuntimeDataMap, err error) {
	if query == "" || cueFile == "" {
		return nil, nil, fmt.Errorf("missing parameter value")
	}

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

	var requiredData []string
	switch t := op.(type) {
	case *opPath:
		ptc, _, rd, subErr := t.Validate(rootValue, rootValue, blockedRootFields)
		if subErr != nil {
			err = subErr
			return
		}
		ptc.String = query

		requiredData = rd

		pps := t.Sprint(0)
		ptc.PrettyPrintedString = &pps

		tc = ptc

	case *opLogicalOperation:
		logicalOperation, subRequiredData, subErr := t.Validate(rootValue, rootValue, blockedRootFields)
		if err != nil {
			err = subErr
			return
		}
		requiredData = subRequiredData

		pps := t.Sprint(0)
		logicalOperation.PrettyPrintedString = &pps

		tc = logicalOperation
	}

	rdm = &RuntimeDataMap{
		String:       query,
		RequiredData: requiredData,
		HasErrors:    tc.HasErrors(),
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

type pathFields struct {
	HasError

	IsFilter            bool          `json:"isFilter"`
	String              string        `json:"string"`
	PrettyPrintedString *string       `json:"prettyPrintedString,omitempty"`
	Type                InputOrOutput `json:"type"`
	Parts               []CanBeAPart  `json:"parts,omitempty"`
}

type Path struct {
	pathFields
}

func (x *Path) PathString() string {
	return x.String
}

func (x *Path) HasErrors() (out bool) {
	if x.Error != nil {
		return true
	}

	for _, p := range x.Parts {
		subErrs := p.HasErrors()
		if subErrs {
			return true
		}
	}

	return
}

func (x *Path) ReturnType() InputOrOutput {
	return x.Type
}
func (x *Path) PartType() string {
	return "Path"
}
func (x *Path) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID       string `json:"id"`
		PartType string `json:"partType"`
		pathFields
	}{
		ID:         uuid.New().String(),
		PartType:   x.PartType(),
		pathFields: x.pathFields,
	})
}

type pathIdentFields struct {
	HasError

	String    string        `json:"string"`
	Type      InputOrOutput `json:"type"`
	Available *Available    `json:"available,omitempty"`
	Filter    *Filter       `json:"filter,omitempty"`
}

type PathIdent struct {
	pathIdentFields
}

func (x *PathIdent) PathString() string {
	return x.String
}

func (x *PathIdent) HasErrors() (out bool) {
	if x.Error != nil {
		return true
	}

	return
}

func (x *PathIdent) ReturnType() InputOrOutput {
	return x.Type
}
func (x *PathIdent) PartType() string {
	return "PathIdent"
}
func (x *PathIdent) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID       string `json:"id"`
		PartType string `json:"partType"`
		pathIdentFields
	}{
		ID:              uuid.New().String(),
		PartType:        x.PartType(),
		pathIdentFields: x.pathIdentFields,
	})
}

type FunctionParameter struct {
	String                          string        `json:"string"`
	Type                            InputOrOutput `json:"type"`
	IsVariadicOfParameterAtPosition *int          `json:"isVariadicOfParameterAtPosition,omitempty"`
	Error                           *string       `json:"error,omitempty"`
	Part                            CanBeAPart    `json:"part,omitempty"`
	Available                       *Available    `json:"available,omitempty"`
}

type functionFields struct {
	HasError

	String              string               `json:"string"`
	Type                InputOrOutput        `json:"type"`
	Available           *Available           `json:"available,omitempty"`
	FunctionName        *string              `json:"functionName,omitempty"`
	FunctionExplanation *string              `json:"functionExplanation,omitempty"`
	FunctionParameters  []*FunctionParameter `json:"functionParameters,omitempty"`
}

type Function struct {
	functionFields
}

func (x *Function) PathString() string {
	return x.String
}

func (x *Function) HasErrors() (out bool) {
	if x.Error != nil {
		return true
	}

	for _, fp := range x.FunctionParameters {
		if fp.Error != nil {
			return true
		}

		if fp.Part != nil && fp.Part.HasErrors() {
			return true
		}
	}

	return
}

func (x *Function) ReturnType() InputOrOutput {
	return x.Type
}
func (x *Function) PartType() string {
	return "Function"
}
func (x *Function) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID       string `json:"id"`
		PartType string `json:"partType"`
		functionFields
	}{
		ID:             uuid.New().String(),
		PartType:       x.PartType(),
		functionFields: x.functionFields,
	})
}

type Available struct {
	Fields    []string `json:"fields,omitempty"`
	Functions []string `json:"functions,omitempty"`
	Filters   []string `json:"filters,omitempty"`
}

type Filter struct {
	String           string            `json:"string"`
	Error            *string           `json:"error,omitempty"`
	LogicalOperation *LogicalOperation `json:"logicalOperation,omitempty"`
}

type logicalOperationFields struct {
	HasError

	IsFilter            bool                      `json:"isFilter"`
	String              string                    `json:"string"`
	PrettyPrintedString *string                   `json:"prettyPrintedString,omitempty"`
	LogicalOperator     *LOT_LogicalOperationType `json:"logicalOperator,omitempty"`
	Parts               []CanBeAPart              `json:"parts,omitempty"`
}

type LogicalOperation struct {
	logicalOperationFields
}

func (x *LogicalOperation) PathString() string {
	return x.String
}

func (x *LogicalOperation) HasErrors() (out bool) {
	if x.Error != nil {
		return true
	}

	for _, p := range x.Parts {
		subErrs := p.HasErrors()
		if subErrs {
			return true
		}
	}

	return
}

func (x *LogicalOperation) ReturnType() InputOrOutput {
	return inputOrOutput(PT_Boolean, IOOT_Single)
}
func (x *LogicalOperation) PartType() string {
	return "LogicalOperation"
}
func (x *LogicalOperation) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID       string `json:"id"`
		PartType string `json:"partType"`
		logicalOperationFields
	}{
		ID:                     uuid.New().String(),
		PartType:               x.PartType(),
		logicalOperationFields: x.logicalOperationFields,
	})
}
