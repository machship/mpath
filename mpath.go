package mpath

import (
	"fmt"
	"strings"
	"sync"
	sc "text/scanner"
	"unicode"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

func Setup(jsonMarshalDecimalsWithoutQuotes bool) {
	decimal.MarshalJSONWithoutQuotes = jsonMarshalDecimalsWithoutQuotes
}

var (
	scannerPool = sync.Pool{
		New: func() any {
			s := newScanner()
			s.sx.Mode = sc.ScanIdents | sc.ScanFloats | sc.ScanChars | sc.ScanStrings | sc.ScanRawStrings | sc.ScanComments | sc.SkipComments
			s.sx.IsIdentRune = func(ch rune, i int) bool {
				if i == 0 && unicode.IsDigit(ch) {
					return false
				}

				return ch != '\'' &&
					ch != '"' &&
					ch != '(' &&
					ch != ')' &&
					ch != '[' &&
					ch != ']' &&
					ch != '{' &&
					ch != '}' &&
					ch != '@' &&
					ch != '$' &&
					ch != '&' &&
					ch != '.' &&
					ch != ',' &&
					ch != '=' &&
					ch != '>' &&
					ch != '<' &&
					ch != '|' &&
					ch != '!' &&
					ch != '#' && //todo: implement this as a global variable
					ch != ';' &&
					ch != '/' &&
					ch != '*' &&
					ch != '?' &&
					!unicode.IsSpace(ch) &&
					unicode.IsPrint(ch)
			}
			s.sx.Error = func(es *sc.Scanner, msg string) {
				//todo: find a way to pipe this out
			}
			return s
		},
	}
	stringsReaderPool = sync.Pool{
		New: func() any {
			return &strings.Reader{}
		},
	}
)

func ParseString(ss string) (topOp Operation, forPath [][]string, err error) {

	s := scannerPool.Get().(*scanner)
	defer scannerPool.Put(s)
	sr := stringsReaderPool.Get().(*strings.Reader)
	defer stringsReaderPool.Put(sr)
	s.Reset(sr, ss)

	var r rune
	r = s.Scan()
	for {
		if r == sc.EOF || r == 0 {
			break
		}

		switch r {
		case '{':
			if topOp != nil {
				return nil, nil, erAt(s, "operation not terminated properly: found Logical Operation after top operation already defined")
			}
			// Curly braces are for logical operation groups (&& and ||)
			topOp = &opLogicalOperation{}
			r, err = topOp.Parse(s, r)
			if err != nil {
				return nil, nil, err
			}
		case '@', '$':
			if topOp != nil {
				return nil, nil, erAt(s, "operation not terminated properly: found Path after top operation already defined")
			}
			// @ and $ are Path starters and specify whether to use the original data, or the data at this point of the path
			topOp = &opPath{}
			r, err = topOp.Parse(s, r)
			if err != nil {
				return nil, nil, err
			}
		default:
			if topOp == nil {
				return nil, nil, errors.Wrap(erInvalid(s, '{', '@', '$'), "invalid query")
			}
			return nil, nil, erAt(s, "operation not terminated properly: found '%s' (%d) after top operation already defined", s.TokenText(), r)
		}
	}

	c, forPath, _ := topOp.ForPath(nil)
	if len(c) > 0 {
		forPath = append(forPath, c)
	}

	return
}

func erAt(s *scanner, str string, args ...any) (err error) {
	args = append([]any{s.sx.Pos().Line, s.sx.Pos().Column}, args...)
	err = fmt.Errorf("error at line %d col %d: "+str, args...)
	return
}

func erInvalid(s *scanner, validRunes ...rune) error {
	if len(validRunes) == 0 {
		return erAt(s, "invalid next character '%s'", s.TokenText())
	}
	if len(validRunes) == 1 {
		return erAt(s, "invalid next character '%s': must be '%s'", s.TokenText(), string(validRunes[0]))
	}

	validRunesAsStrings := make([]string, len(validRunes))
	for idx, vr := range validRunes {
		validRunesAsStrings[idx] = string(vr)
	}
	return erAt(s, "invalid next character '%s': must be one of '%s'", s.TokenText(), strings.Join(validRunesAsStrings, "', '"))
}

type scanner struct {
	sx *sc.Scanner
}

func newScanner() *scanner {
	return &scanner{
		sx: &sc.Scanner{},
	}
}

func (s *scanner) TokenText() (t string) {
	return s.sx.TokenText()
}

func (s *scanner) Reset(sr *strings.Reader, ss string) {
	sr.Reset(ss)
	s.sx.Init(sr)
}

func (s *scanner) Scan() (r rune) {
	for {
		r = s.sx.Scan()
		// fmt.Print(s.sx.TokenText())
		if r < 0 || unicode.IsPrint(r) {
			break
		}
	}
	return
}
