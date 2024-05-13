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
	GetErrors() (errMessage string)
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

type CuePath []string

func (c CuePath) Add(s string) CuePath {
	if len(c) == 0 {
		return CuePath{s}
	}

	return append(c, s)
}

// query: the mpath query string
// cueFile: the cue file
// currentPath: the id of the step for which this query is an input value, or if for the output, leave blank
func CueValidate(query, cueFile, currentPath string) (tc CanBeAPart, err error) {
	if query == "" || cueFile == "" {
		return nil, fmt.Errorf("missing parameter value")
	}

	var ok bool

	// mpath operations are cached to ensure speed of execution as this method is expected to be hit many times
	var op Operation
	if op, ok = mpathOpCache[query]; !ok {
		op, err = ParseString(query)
		if err != nil {
			return nil, fmt.Errorf("failed to parse mpath query: %w", err)
		}
		mpathOpCache[query] = op
	}

	// cue values are cached to ensure speed of execution as this method is expected to be hit many times
	var rootValue cue.Value
	if rootValue, ok = cueValueCache[cueFile]; !ok {
		ctx := cuecontext.New()
		rootValue = ctx.CompileString(cueFile)
		if rootValue.Err() != nil {
			return nil, fmt.Errorf("failed to parse cue file: %w", rootValue.Err())
		}
		cueValueCache[cueFile] = rootValue
	}

	var blockedRootFields []string
	if currentPath != "" {
		blockedRootFields, err = getBlockedRootFields(rootValue, currentPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get blocked fields for rootValue: %w", err)
		}
	}

	// If we get to this point, the mpath query and cueFile are both valid,
	// thus the next steps are to walk through the "paths" in the returned AST
	// and doubt check that they are valid given the cueFile.

	switch t := op.(type) {
	case *opPath:
		ptc, _ := t.Validate(rootValue, CuePath{}, blockedRootFields)
		tc = ptc
		if ptc.Error != nil {
			err = fmt.Errorf(*ptc.Error)
			return
		}
		ptc.String = query

		pps := t.Sprint(0)
		ptc.PrettyPrintedString = &pps

		tc = ptc

	case *opLogicalOperation:
		logicalOperation := t.Validate(rootValue, CuePath{}, blockedRootFields)
		tc = logicalOperation
		if logicalOperation.Error != nil {
			err = fmt.Errorf(*logicalOperation.Error)
			return
		}

		pps := t.Sprint(0)
		logicalOperation.PrettyPrintedString = &pps
	}

	return
}

func strPtr(s string) *string {
	return &s
}

func strInStrSlice(s string, ss []string) (isInSlice bool) {
	for _, sss := range ss {
		if sss == s {
			return true
		}
	}

	return false
}

func getSelectorForField(inputValue cue.Value, name string) (selector cue.Selector) {
	if !(strings.HasPrefix(name, "_") && !strings.Contains(name, "-")) {
		return cue.Str(name)
	}

	selector = cue.Hid(name, "_")

	findValue := inputValue.LookupPath(cue.MakePath(selector))
	if findValue.Err() == nil {
		return selector
	}

	return cue.Str(name)
}

func findValueAtPath(inputValue cue.Value, cuePath CuePath) (outputValue cue.Value, err error) {
	errFunc := func(s string, v cue.Value, _ error) (cue.Value, error) {
		return v, fmt.Errorf("couldn't access field '%s'", s)
		// return v, fmt.Errorf("couldn't access field '%s': %w; value is %#v", s, err, v)
	}
	outputValue = inputValue

	/*
		todo: this function is ugly as hell.
		Need to see if there is a better way to achieve all of this
	*/

	var thisValue cue.Value
	for _, cp := range cuePath {
		selector := getSelectorForField(outputValue, cp)
		thisValue = outputValue.LookupPath(cue.MakePath(selector))
		if thisValue.Err() != nil {
			thisValue = outputValue.LookupPath(cue.MakePath(selector.Optional()))
			if err = thisValue.Err(); err != nil {
				thisValue = outputValue.LookupPath(cue.MakePath(cue.AnyIndex))
				if err = thisValue.Err(); err != nil {
					outputValueKind := outputValue.IncompleteKind()

					if outputValueKind == cue.TopKind {
						return outputValue, nil
					}

					if outputValueKind == cue.ListKind {
						// Get an iterator
						it, err := outputValue.List()
						if err != nil {
							return errFunc(cp, thisValue, err)
						}

						it.Next()
						thisValue = it.Value()
					}

					if err = thisValue.Err(); err != nil {
						return errFunc(cp, thisValue, err)
					}

					thisValue = thisValue.LookupPath(cue.MakePath(selector))
					if err = outputValue.Err(); err != nil {
						return errFunc(cp, thisValue, err)
					}
				}
			}
		}
		outputValue = thisValue
	}

	if err = outputValue.Err(); err != nil {
		return errFunc(strings.Join(cuePath, "."), outputValue, err)
	}

	switch outputValue.IncompleteKind() {
	case cue.BottomKind:
		outputValue = outputValue.LookupPath(cue.MakePath(cue.AnyIndex))
	}

	return outputValue, outputValue.Err()
}

func getUnderlyingKind(v cue.Value) (kind cue.Kind, err error) {
	if v.IncompleteKind() == cue.ListKind {
		var it cue.Iterator
		it, err = v.List()
		if err != nil {
			return kind, fmt.Errorf("couldn't get list iterator for list kind")
		}

		if !it.Next() {
			// it isn't iterable, therefore it is of the form: `[...int]`
			kind = v.LookupPath(cue.MakePath(cue.AnyIndex)).IncompleteKind()
			return
		}

		v = it.Value()
	}

	kind = v.IncompleteKind()
	if kind == cue.BottomKind {
		kind = v.LookupPath(cue.MakePath(cue.AnyIndex)).IncompleteKind()
	}

	return
}

func getUnderlyingValue(v cue.Value) (val cue.Value, err error) {
	if v.IncompleteKind() == cue.ListKind {
		it, err := v.List()
		if err != nil {
			return val, fmt.Errorf("couldn't get list iterator for list kind")
		}

		if !it.Next() {
			// it isn't iterable, therefore it is of the form: `[...int]`
			return v.LookupPath(cue.MakePath(cue.AnyIndex)), nil
		}

		v = it.Value()
	}

	return v, nil
}

func getAvailableFieldsForValue(v cue.Value, blockedRootFields []string) (fields []string, err error) {
	if v.IncompleteKind() != cue.StructKind {
		v, err = getUnderlyingValue(v)
		if err != nil {
			return nil, err
		}
	}

	it, err := v.Fields(cue.All())
	if err != nil {
		// k := v.IncompleteKind()
		// fmt.Println(k == cue.BottomKind)
		return nil, fmt.Errorf("failed to list fields: %w\n%#v", err, v)
	}

	for it.Next() {
		fldName := it.Selector().String()

		switch fldName {
		case string(BP_Dependencies):
			continue
		}

		if checkIfValueInList(fldName, blockedRootFields) {
			continue
		}

		// Strip leading and trailing quotation marks from names:
		if strings.HasPrefix(fldName, `"`) && strings.HasSuffix(fldName, `"`) {
			fldName = strings.TrimPrefix(fldName, `"`)
			fldName = strings.TrimSuffix(fldName, `"`)
		}

		if strings.HasSuffix(fldName, "?") {
			fldName = strings.TrimSuffix(fldName, "?")
		}

		if strings.HasSuffix(fldName, "!") {
			fldName = strings.TrimSuffix(fldName, "!")
		}

		fields = append(fields, fldName)
	}

	return
}

func checkIfValueInList(value string, list []string) (isInList bool) {
	if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		value = strings.TrimPrefix(value, `"`)
		value = strings.TrimSuffix(value, `"`)
	}

	for _, listValue := range list {
		if value == listValue {
			return true
		}
	}

	return false
}

var (
	mpathOpCache  = map[string]Operation{}
	cueValueCache = map[string]cue.Value{}
)

type BP_BasePath string

const (
	BP_Dependencies              BP_BasePath = "_dependencies"
	BP_InputWithUnderscore       BP_BasePath = "_input"
	BP_Input                     BP_BasePath = "input"
	BP_VariablesWithUnderscore   BP_BasePath = "_variables"
	BP_Variables                 BP_BasePath = "variables"
	BP_Secrets                   BP_BasePath = "secrets"
	BP_SecretsWithUnderscore     BP_BasePath = "_secrets"
	BP_Connections               BP_BasePath = "connections"
	BP_ConnectionsWithUnderscore BP_BasePath = "_connections"
)

func getConcreteValuesForListOfStringValueAtPath(inputValue cue.Value, cuePath CuePath) (output []string, err error) {
	foundValue, err := findValueAtPath(inputValue, cuePath)
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

func getExpr(inputValue cue.Value) *string {
	// Leaving this here... this will give you the actual addressed value rather than the type
	// source := inputValue.Source()
	// cp, _ := format.Node(source) // ignore error as this is currently best effort
	// cpStr := string(cp)
	// return &cpStr

	outStr := fmt.Sprintf("%#v", inputValue)
	return &outStr
}

func getBlockedRootFields(rootValue cue.Value, rootFieldName string) (blockedFields []string, err error) {
	// Find the current path:
	nextValue, err := findValueAtPath(rootValue, CuePath{rootFieldName})
	if err != nil {
		return nil, fmt.Errorf("failed to find currentPath in cue value: %w", err)
	}

	if rootFieldName != string(BP_Input) {
		blockedFields = append(blockedFields, rootFieldName)
	}

	// We need to recursively get the dependencies of the currentPath
	dependencies, err := getConcreteValuesForListOfStringValueAtPath(nextValue, CuePath{string(BP_Dependencies)})
	if err != nil {
		return nil, fmt.Errorf("failed to find dependencies in cue value: %w", err)
	}

	validFields := map[string]struct{}{
		rootFieldName:                        {},
		string(BP_Input):                     {},
		string(BP_InputWithUnderscore):       {},
		string(BP_Variables):                 {},
		string(BP_VariablesWithUnderscore):   {},
		string(BP_Secrets):                   {},
		string(BP_SecretsWithUnderscore):     {},
		string(BP_Connections):               {},
		string(BP_ConnectionsWithUnderscore): {},
	}
	for _, dep := range dependencies {
		validFields[dep] = struct{}{}
	}

	var nextDependencies []string
loop:
	for _, d := range dependencies {
		nextValue, err = findValueAtPath(rootValue, CuePath{d})
		if err != nil {
			return nil, fmt.Errorf("failed to find dependency '%s' in cue value: %w", d, err)
		}

		nextDependencies, err = getConcreteValuesForListOfStringValueAtPath(nextValue, CuePath{string(BP_Dependencies)})
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

	allFields, err := getAvailableFieldsForValue(rootValue, nil)
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

func (x *Path) GetErrors() (errMessage string) {
	errMessages := []string{}
	if x.Error != nil && *x.Error != "" {
		errMessages = append(errMessages, *x.Error)
	}

	for _, p := range x.Parts {
		if errs := p.GetErrors(); errs != "" {
			errMessages = append(errMessages, errs)
		}
	}

	errMessage = strings.Join(errMessages, "; ")
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

func (x *PathIdent) GetErrors() (errMessage string) {
	if x.Error != nil && *x.Error != "" {
		return *x.Error
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

func (x *Function) GetErrors() (errMessage string) {
	errMessages := []string{}
	if x.Error != nil && *x.Error != "" {
		errMessages = append(errMessages, *x.Error)
	}

	for _, fp := range x.FunctionParameters {
		if fp.Error != nil && *fp.Error != "" {
			errMessages = append(errMessages, *fp.Error)
		}

		if fp.Part != nil && fp.Part.HasErrors() {
			if errs := fp.Part.GetErrors(); errs != "" {
				errMessages = append(errMessages, errs)
			}
		}
	}

	return strings.Join(errMessages, "; ")
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

func (x *LogicalOperation) GetErrors() (errMessage string) {
	errMessages := []string{}
	if x.Error != nil && *x.Error != "" {
		errMessages = append(errMessages, *x.Error)
	}

	for _, p := range x.Parts {
		if errs := p.GetErrors(); p.HasErrors() && errs != "" {
			errMessages = append(errMessages, errs)
		}
	}

	return strings.Join(errMessages, "; ")
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
