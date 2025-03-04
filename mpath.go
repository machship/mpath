package mpath

import (
	"fmt"
	"io"
	"strings"
	"sync"
	sc "text/scanner"
	"unicode"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

var invalidRunes = map[rune]bool{
	'\'': true, '"': true, '(': true, ')': true, '[': true, ']': true,
	'{': true, '}': true, '@': true, '$': true, '&': true, '.': true,
	',': true, '=': true, '>': true, '<': true, '|': true, '!': true,
	';': true, '/': true, '*': true, '#': true,
}

func Setup(jsonMarshalDecimalsWithoutQuotes bool) {
	decimal.MarshalJSONWithoutQuotes = jsonMarshalDecimalsWithoutQuotes
}

var (
	scannerPool = sync.Pool{
		New: func() any {
			s := newScanner()
			s.sx.Mode = sc.ScanIdents | sc.ScanChars | sc.ScanStrings | sc.ScanRawStrings | sc.ScanComments | sc.SkipComments
			s.sx.IsIdentRune = func(ch rune, i int) bool {
				if invalidRunes[ch] || unicode.IsSpace(ch) {
					return false
				}

				return unicode.IsPrint(ch)
			}
			s.sx.Error = func(es *sc.Scanner, msg string) {
				//todo: find a way to pipe this out
			}
			return s
		},
	}
)

// ParseReadSeeker takes an io.ReadSeeker and parses it into an operation tree.
func ParseReadSeeker(r io.ReadSeeker) (topOp Operation, err error) {
	s := scannerPool.Get().(*scanner)
	defer func() {
		s.err = nil
		scannerPool.Put(s)
	}()

	err = s.Reset(r)
	if err != nil {
		return nil, err
	}

	var tok rune
	tok = s.Scan()
	for {
		if tok == sc.EOF || tok == 0 {
			break
		}

		switch tok {
		case '{':
			if topOp != nil {
				return nil, erAt(s, "operation not terminated properly: found Logical Operation after top operation already defined")
			}
			topOp = &opLogicalOperation{}
			tok, err = topOp.Parse(s, tok)
			if err != nil {
				return nil, err
			}
		case '@', '$':
			if topOp != nil {
				return nil, erAt(s, "operation not terminated properly: found Path after top operation already defined")
			}
			topOp = &opPath{}
			tok, err = topOp.Parse(s, tok)
			if err != nil {
				return nil, err
			}
		default:
			if topOp == nil {
				return nil, errors.Wrap(erInvalid(s, '{', '@', '$'), "invalid query")
			}
			return nil, erAt(s, "operation not terminated properly: found '%s' (%d) after top operation already defined", s.TokenText(), tok)
		}
	}

	// return scanner error if any
	if err := s.Err(); err != nil {
		return nil, err
	}

	return
}

func ParseString(ss string) (topOp Operation, err error) {
	sr := strings.NewReader(ss)
	return ParseReadSeeker(sr)
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
	sx  *sc.Scanner
	err error
}

func newScanner() *scanner {
	s := &scanner{sx: &sc.Scanner{}}
	s.sx.Error = func(es *sc.Scanner, msg string) {
		s.err = errors.New(msg)
	}
	return s
}

func (s *scanner) TokenText() (t string) {
	return s.sx.TokenText()
}

func (s *scanner) Reset(reader io.ReadSeeker) error {
	_, err := reader.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	s.sx.Init(reader)
	s.sx.Mode = sc.ScanIdents | sc.ScanChars | sc.ScanStrings | sc.ScanRawStrings | sc.ScanComments | sc.SkipComments
	return nil
}

func (s *scanner) Scan() (r rune) {
	for {
		r = s.sx.Scan()

		// todo: what is this for?
		// if r == -4 {
		// 	// fmt.Print(string(r))
		// }

		// fmt.Print(s.sx.TokenText())
		if r < 0 || unicode.IsPrint(r) {
			break
		}
	}
	return
}

func (s *scanner) Err() error {
	return s.err
}
