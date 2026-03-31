package lexer

import (
	"testing"
)

func TestTokenizeSimple(t *testing.T) {
	tokens := TokenizeSimple("Hello World")
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].Word != "hello" {
		t.Errorf("expected 'hello', got %q", tokens[0].Word)
	}
	if tokens[1].Word != "world" {
		t.Errorf("expected 'world', got %q", tokens[1].Word)
	}
}

func TestTokenizeEmpty(t *testing.T) {
	tokens := TokenizeSimple("")
	if tokens != nil {
		t.Errorf("expected nil, got %v", tokens)
	}
}

func TestTokenizeLowercasing(t *testing.T) {
	tokens := TokenizeSimple("UPPERCASE MiXeD")
	for _, tok := range tokens {
		for _, r := range tok.Word {
			if r >= 'A' && r <= 'Z' {
				t.Errorf("word %q contains uppercase", tok.Word)
			}
		}
	}
}

func TestTokenizeStopwordRemoval(t *testing.T) {
	tokens := TokenizeSimple("the quick brown fox is a very good animal")
	for _, tok := range tokens {
		if IsStopword(tok.Word) {
			t.Errorf("stopword %q was not removed", tok.Word)
		}
	}
	// "quick", "brown", "fox", "good", "animal" should remain
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}
	for _, expected := range []string{"quick", "brown", "fox", "good", "animal"} {
		if !words[expected] {
			t.Errorf("expected word %q missing", expected)
		}
	}
}

func TestTokenizeDedup(t *testing.T) {
	tokens := TokenizeSimple("hello hello hello world world")
	if len(tokens) != 2 {
		t.Fatalf("expected 2 deduped tokens, got %d: %v", len(tokens), tokens)
	}
}

func TestTokenizeMinLength(t *testing.T) {
	opts := Options{
		RemoveStopwords: false,
		MinWordLength:   3,
		MaxWordLength:   64,
	}
	tokens := Tokenize("I am ok yes no go", opts)
	for _, tok := range tokens {
		if len([]rune(tok.Word)) < 3 {
			t.Errorf("word %q shorter than min length 3", tok.Word)
		}
	}
}

func TestTokenizeMaxLength(t *testing.T) {
	opts := Options{
		RemoveStopwords: false,
		MinWordLength:   1,
		MaxWordLength:   5,
	}
	tokens := Tokenize("hi there longword ab", opts)
	for _, tok := range tokens {
		if len([]rune(tok.Word)) > 5 {
			t.Errorf("word %q longer than max length 5", tok.Word)
		}
	}
}

func TestTokenizePunctuation(t *testing.T) {
	tokens := TokenizeSimple("hello, world! how are you?")
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}
	if !words["hello"] || !words["world"] {
		t.Error("punctuation was not properly handled")
	}
}

func TestTokenizeNumbers(t *testing.T) {
	tokens := Tokenize("test123 456 abc", Options{
		RemoveStopwords: false,
		MinWordLength:   1,
		MaxWordLength:   64,
	})
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}
	if !words["test123"] {
		t.Error("expected 'test123' to be preserved")
	}
	if !words["456"] {
		t.Error("expected '456' to be preserved")
	}
}

func TestTokenizeUnicode(t *testing.T) {
	tokens := Tokenize("cafe resume", Options{
		RemoveStopwords: false,
		MinWordLength:   1,
		MaxWordLength:   64,
	})
	if len(tokens) < 2 {
		t.Fatalf("expected at least 2 tokens, got %d", len(tokens))
	}
}

func TestHashTermConsistency(t *testing.T) {
	h1 := HashTerm("hello")
	h2 := HashTerm("hello")
	if h1 != h2 {
		t.Errorf("same word produced different hashes: %d != %d", h1, h2)
	}
}

func TestHashTermDifferent(t *testing.T) {
	h1 := HashTerm("hello")
	h2 := HashTerm("world")
	if h1 == h2 {
		t.Error("different words produced same hash")
	}
}

func TestExtractTermHashes(t *testing.T) {
	hashes := ExtractTermHashes("hello world")
	if len(hashes) != 2 {
		t.Fatalf("expected 2 hashes, got %d", len(hashes))
	}
	if hashes[0] == 0 || hashes[1] == 0 {
		t.Error("hash should not be zero")
	}
}

func TestExtractTerms(t *testing.T) {
	terms := ExtractTerms("Hello World test")
	found := make(map[string]bool)
	for _, term := range terms {
		found[term] = true
	}
	if !found["hello"] || !found["world"] || !found["test"] {
		t.Errorf("expected hello, world, test; got %v", terms)
	}
}

func TestIsStopword(t *testing.T) {
	tests := []struct {
		word     string
		expected bool
	}{
		{"the", true},
		{"is", true},
		{"hello", false},
		{"world", false},
		{"and", true},
		{"", false},
	}

	for _, tt := range tests {
		got := IsStopword(tt.word)
		if got != tt.expected {
			t.Errorf("IsStopword(%q) = %v, want %v", tt.word, got, tt.expected)
		}
	}
}

func TestSplitWordsHyphen(t *testing.T) {
	words := splitWords("well-known pattern")
	if len(words) != 3 {
		t.Fatalf("expected 3 words from hyphenated, got %d: %v", len(words), words)
	}
}

func TestSplitWordsMultipleSpaces(t *testing.T) {
	words := splitWords("hello    world")
	if len(words) != 2 {
		t.Fatalf("expected 2 words, got %d", len(words))
	}
}

func TestTokenizeNoStopwords(t *testing.T) {
	opts := Options{
		RemoveStopwords: false,
		MinWordLength:   1,
		MaxWordLength:   64,
	}
	tokens := Tokenize("the quick brown fox", opts)
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}
	if !words["the"] {
		t.Error("expected 'the' to be kept when stopwords disabled")
	}
}

func TestTokenizeOnlyStopwords(t *testing.T) {
	tokens := TokenizeSimple("the is a an")
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for all-stopword input, got %d", len(tokens))
	}
}

func TestTokenizeSpecialChars(t *testing.T) {
	tokens := TokenizeSimple("hello@world.com test_case")
	if len(tokens) == 0 {
		t.Error("expected some tokens")
	}
}

func TestApostropheStopwordsFiltered(t *testing.T) {
	// Apostrophe-containing stopwords like "aren't" get split by the tokenizer
	// into ["aren", "t"]. The fragment "aren" should still be filtered as a
	// stopword to avoid polluting the index with noise.
	tokens := TokenizeSimple("they aren't going to the store and we couldn't find it")
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}

	// "aren", "couldn" should be filtered (split forms of "aren't", "couldn't")
	noiseWords := []string{"aren", "couldn"}
	for _, w := range noiseWords {
		if words[w] {
			t.Errorf("split-form stopword %q should have been filtered", w)
		}
	}

	// Real words should remain
	expectedWords := []string{"going", "store", "find"}
	for _, w := range expectedWords {
		if !words[w] {
			t.Errorf("expected word %q to remain after filtering", w)
		}
	}
}

func TestSplitFormStopwords(t *testing.T) {
	// Verify all split-form stopwords are recognized
	fragments := []string{"aren", "couldn", "didn", "doesn", "don", "hadn",
		"hasn", "haven", "isn", "mustn", "shan", "shouldn", "wasn",
		"weren", "won", "wouldn", "ll", "ve", "re"}
	for _, w := range fragments {
		if !IsStopword(w) {
			t.Errorf("expected %q to be recognized as a stopword", w)
		}
	}
}
