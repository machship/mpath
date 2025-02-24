package mpath

import (
	"bytes"
	"io"
	"strings"
	"testing"
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
		{"ChunkedAcross", "StartOfChunk", "tOfCh", true},         // Crosses chunks
		{"ChunkedAcrossNoMatch", "StartOfChunk", "tOfGh", false}, // Crosses chunks but no match
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader([]byte(tc.input))
			result, err := readerContains(r, tc.substr)
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
		checkFn  func(io.Reader, string) (bool, error)
		expected bool
	}{
		{"SuffixMatch", "Hello, world!", "world!", readerHasSuffix, true},
		{"SuffixNoMatch", "Hello, world!", "hello", readerHasSuffix, false},
		{"EmptyInput", "", "anything", readerHasSuffix, false},
		{"EmptySuffix", "Some text here", "", readerHasSuffix, true}, // Empty suffix should always match
		{"ExactMatch", "Hello", "Hello", readerHasSuffix, true},
		{"SingleCharMatch", "x", "x", readerHasSuffix, true},
		{"SingleCharNoMatch", "x", "y", readerHasSuffix, false},
		{"SuffixAcrossChunks", "ChunkedDataEndsHere", "EndsHere", readerHasSuffix, true},
		{"SuffixAcrossChunksNoMatch", "ChunkedDataEndsHere", "StartsHere", readerHasSuffix, false},
		{"LargeSuffixMatch", strings.Repeat("a", 100000) + "suffix", "suffix", readerHasSuffix, true},
		{"LargeSuffixNoMatch", strings.Repeat("a", 100000) + "suffiy", "suffix", readerHasSuffix, false},
		{"UnicodeMatch", "こんにちは世界", "世界", readerHasSuffix, true},
		{"UnicodeNoMatch", "こんにちは世界", "さようなら", readerHasSuffix, false},
		{"MultibyteMatch", "😊🎉👍", "🎉👍", readerHasSuffix, true},
		{"MultibyteNoMatch", "😊🎉👍", "😢", readerHasSuffix, false},
	}

	for _, tc := range testCases {
		r := bytes.NewReader([]byte(tc.input))
		result, err := tc.checkFn(r, tc.check)
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
		checkFn  func(io.Reader, string) (bool, error)
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
			result, err := tc.checkFn(r, tc.check)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tc.expected {
				t.Fatalf("unexpected result: wanted: %v; got: %v", tc.expected, result)
			}
		})
	}
}
