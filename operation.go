package mpath

////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////

type Operation interface {
	Do(currentData, originalData any) (dataToUse any, err error)
	Parse(s *scanner, r rune) (nextR rune, err error)
	Sprint(depth int) (out string)
	ForPath(current []string) (outCurrent []string, additional [][]string, shouldStopLoop bool)
	Type() OT_OpType
	MarshalJSON() ([]byte, error)
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
