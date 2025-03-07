package mpath

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestReaderContains(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		substr   string
		expected bool
	}{
		{"BasicMatch", "Hello, world!", "world", true},
		{"BasicNoMatch", "Hello, world!", "planet", false},
		{"EmptyInput", "", "anything", false},          // No data at all
		{"EmptySubstring", "Some text here", "", true}, // Empty substring should always match
		{"ExactMatch", "Hello", "Hello", true},         // Input matches substring exactly
		{"SingleCharMatch", "x", "x", true},
		{"SingleCharNoMatch", "x", "y", false},
		{"StartsWithSubstring", "abcde", "abc", true},
		{"DoesNotStartWithSubstring", "abcde", "bcd", true},
		{"EndsWithSubstring", "abcdefg", "efg", true},
		{"EndsWithSubstringNoMatch", "abcdefg", "gh", false},
		{"SubstringAtBoundary", "FirstHalf", "stHa", true}, // Crosses chunk boundary
		{"SubstringAtBoundaryNoMatch", "FirstHalf", "stHo", false},
		{"LargeInputMatch", strings.Repeat("a", 100000) + "match", "match", true},
		{"LargeInputNoMatch", strings.Repeat("a", 100000) + "nomatch", "miss", false},
		{"UnicodeMatch", "こんにちは世界", "世界", true},
		{"UnicodeNoMatch", "こんにちは世界", "さようなら", false},
		{"MultibyteMatch", "😊🎉👍", "🎉", true},
		{"MultibyteNoMatch", "😊🎉👍", "😢", false},
		{"NewlineMatch", "Hello\nWorld", "\n", true},
		{"TabMatch", "Hello\tWorld", "\t", true},
		{"SpecialCharMatch", "Email: test@example.com", "@example.com", true},
		{"ChunkedMatch", "This is the first part of the ", "part of the ", true},
		{"ChunkedNoMatch", "This is the first part of the ", "nonexistent substring", false},
		{"ChunkedAcross", "StartOfChunk", "tOfCh", true},                              // Crosses chunks
		{"ChunkedAcrossNoMatch", "StartOfChunk", "tOfGh", false},                      // Crosses chunks but no match
		{"BoundaryExactMatch", strings.Repeat("A", 4095) + "BCDEFG", "ABCDEFG", true}, // Substring bridging exactly at the 4096 boundary
		{"BoundaryExactNoMatch", strings.Repeat("A", 4095) + "BCDEFG", "ABZDEFG", false},
		{"BoundaryCrossPartial", strings.Repeat("X", 4093) + "XYZ" + "ABC", "XYZABC", true}, // Substring partly in chunk N, partly in chunk N+1. e.g. half is at end of chunk, half is at start of next chunk
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader([]byte(tc.input))
			result, err := readerContains(r, strings.NewReader(tc.substr))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tc.expected {
				t.Fatalf("unexpected result: wanted: %v; got: %v", tc.expected, result)
			}
		})
	}
}

func TestReaderHasSuffix(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		check    string
		expected bool
	}{
		{"SuffixMatch", "Hello, world!", "world!", true},
		{"SuffixNoMatch", "Hello, world!", "hello", false},
		{"EmptyInput", "", "anything", false},
		{"EmptySuffix", "Some text here", "", true}, // Empty suffix should always match
		{"ExactMatch", "Hello", "Hello", true},
		{"SingleCharMatch", "x", "x", true},
		{"SingleCharNoMatch", "x", "y", false},
		{"SuffixAcrossChunks", "ChunkedDataEndsHere", "EndsHere", true},
		{"SuffixAcrossChunksNoMatch", "ChunkedDataEndsHere", "StartsHere", false},
		{"LargeSuffixMatch", strings.Repeat("a", 100000) + "suffix", "suffix", true},
		{"LargeSuffixNoMatch", strings.Repeat("a", 100000) + "suffiy", "suffix", false},
		{"UnicodeMatch", "こんにちは世界", "世界", true},
		{"UnicodeNoMatch", "こんにちは世界", "さようなら", false},
		{"MultibyteMatch", "😊🎉👍", "🎉👍", true},
		{"MultibyteNoMatch", "😊🎉👍", "😢", false},
		{"BoundaryExactSuffix", strings.Repeat("A", 4096) + "END", "END", true}, // Check boundary crossing (4096 chunk). We'll put suffix so it might appear at the boundary (e.g. 4096 'A' plus "END")
		{"BoundaryExactNoMatch", strings.Repeat("A", 4096) + "END", "XEND", false},
		{"SuffixCrossBoundary", strings.Repeat("A", 4094) + "XY" + "Z", "XYZ", true}, // Suffix crossing chunk boundary in a partial manner
	}

	for _, tc := range testCases {
		r := bytes.NewReader([]byte(tc.input))
		result, err := readerHasSuffix(r, strings.NewReader(tc.check))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != tc.expected {
			t.Fatalf("unexpected result: wanted: %v; got: %v", tc.expected, result)
		}
	}
}

func TestReaderHasPrefix(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		check    string
		checkFn  func(io.Reader, io.Reader) (bool, error)
		expected bool
	}{
		{"PrefixMatch", "Hello, world!", "Hello", readerHasPrefix, true},
		{"PrefixNoMatch", "Hello, world!", "World", readerHasPrefix, false},
		{"EmptyInput", "", "anything", readerHasPrefix, false},       // Empty input should not match anything
		{"EmptyPrefix", "Some text here", "", readerHasPrefix, true}, // Empty prefix should always match
		{"ExactMatch", "Hello", "Hello", readerHasPrefix, true},      // Input matches prefix exactly
		{"SingleCharMatch", "x", "x", readerHasPrefix, true},
		{"SingleCharNoMatch", "x", "y", readerHasPrefix, false},
		{"LargePrefixMatch", "prefix" + strings.Repeat("a", 100000), "prefix", readerHasPrefix, true},
		{"LargePrefixNoMatch", "prefiy" + strings.Repeat("a", 100000), "prefix", readerHasPrefix, false},
		{"UnicodeMatch", "こんにちは世界", "こんにちは", readerHasPrefix, true},
		{"UnicodeNoMatch", "こんにちは世界", "さようなら", readerHasPrefix, false},
		{"MultibyteMatch", "😊🎉👍", "😊🎉", readerHasPrefix, true},
		{"MultibyteNoMatch", "😊🎉👍", "😢", readerHasPrefix, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader([]byte(tc.input))
			result, err := tc.checkFn(r, strings.NewReader(tc.check))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tc.expected {
				t.Fatalf("unexpected result: wanted: %v; got: %v", tc.expected, result)
			}
		})
	}
}

func TestReadDecimalFromReaderAndCheck(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected bool
		value    decimal.Decimal
	}{
		{"ValidInteger", "12345", true, decimal.NewFromInt(12345)},
		{"ValidDecimal", "123.45", true, decimal.NewFromFloat(123.45)},
		{"Zero", "0", true, decimal.NewFromInt(0)},
		{"LeadingZeros", "0000123", true, decimal.NewFromInt(123)},
		{"TrailingZerosDecimal", "123.4500", true, decimal.NewFromFloat(123.45)},
		{"OnlyDot", ".", false, decimal.Decimal{}},
		{"MultipleDots", "12.34.56", false, decimal.Decimal{}},
		{"NonNumeric", "abc123", false, decimal.Decimal{}},
		{"EmbeddedLetters", "12a34", false, decimal.Decimal{}},
		{"EmptyInput", "", false, decimal.Decimal{}},
		{"LargeNumber", strings.Repeat("9", 100), true, decimal.RequireFromString(strings.Repeat("9", 100))},
		{"TooLargeNumber", strings.Repeat("9", 600), false, decimal.Decimal{}}, // Over limit, should fail
		{"NumberWithSpaces", " 12345 ", false, decimal.Decimal{}},              // Spaces are invalid
		{"NegativeNumber", "-123.45", false, decimal.Decimal{}},                // No support for negatives
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := strings.NewReader(tc.input)
			result, value := readDecimalFromReaderAndCheck(r)

			if result != tc.expected {
				t.Fatalf("unexpected result: wanted %v; got %v", tc.expected, result)
			}

			if result && !value.Equal(tc.value) {
				t.Fatalf("unexpected decimal value: wanted %v; got %v", tc.value, value)
			}
		})
	}
}
