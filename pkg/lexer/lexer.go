// Package lexer provides text tokenization, normalization, and stopword removal
// for the sonic-go search backend.
package lexer

import (
	"hash/fnv"
	"strings"
	"unicode"
)

// Token represents a single extracted term with its hash.
type Token struct {
	Word string
	Hash uint32
}

// Options controls tokenizer behavior.
type Options struct {
	// RemoveStopwords controls whether English stopwords are filtered.
	RemoveStopwords bool
	// MinWordLength is the minimum character length for a term to be kept.
	MinWordLength int
	// MaxWordLength is the maximum character length for a term.
	MaxWordLength int
}

// DefaultOptions returns standard tokenization options.
func DefaultOptions() Options {
	return Options{
		RemoveStopwords: true,
		MinWordLength:   2,
		MaxWordLength:   64,
	}
}

// Tokenize splits text into normalized, deduplicated tokens.
// It performs: lowercasing, word splitting (UAX29-style), stopword removal,
// length filtering, and deduplication by hash.
func Tokenize(text string, opts Options) []Token {
	if text == "" {
		return nil
	}

	text = strings.ToLower(text)
	words := splitWords(text)

	seen := make(map[uint32]struct{})
	var tokens []Token

	for _, w := range words {
		if len([]rune(w)) < opts.MinWordLength {
			continue
		}
		if opts.MaxWordLength > 0 && len([]rune(w)) > opts.MaxWordLength {
			continue
		}
		if opts.RemoveStopwords && IsStopword(w) {
			continue
		}

		h := HashTerm(w)
		if _, exists := seen[h]; exists {
			continue
		}
		seen[h] = struct{}{}

		tokens = append(tokens, Token{Word: w, Hash: h})
	}

	return tokens
}

// TokenizeSimple is a convenience wrapper using default options.
func TokenizeSimple(text string) []Token {
	return Tokenize(text, DefaultOptions())
}

// HashTerm computes a 32-bit FNV-1a hash for a term.
func HashTerm(term string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(term))
	return h.Sum32()
}

// splitWords performs UAX29-style word boundary splitting.
// It splits on non-letter/non-digit characters and yields word tokens.
func splitWords(text string) []string {
	var words []string
	var current []rune

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current = append(current, r)
		} else {
			if len(current) > 0 {
				words = append(words, string(current))
				current = current[:0]
			}
		}
	}
	if len(current) > 0 {
		words = append(words, string(current))
	}

	return words
}

// ExtractTermHashes tokenizes text and returns only the hashes.
func ExtractTermHashes(text string) []uint32 {
	tokens := TokenizeSimple(text)
	hashes := make([]uint32, len(tokens))
	for i, t := range tokens {
		hashes[i] = t.Hash
	}
	return hashes
}

// ExtractTerms tokenizes text and returns only the words.
func ExtractTerms(text string) []string {
	tokens := TokenizeSimple(text)
	words := make([]string, len(tokens))
	for i, t := range tokens {
		words[i] = t.Word
	}
	return words
}
