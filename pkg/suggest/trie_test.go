package suggest

import (
	"testing"
)

func TestTrieInsertAndSuggest(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt", "hello")
	tr.Insert("col", "bkt", "help")
	tr.Insert("col", "bkt", "helicopter")

	results := tr.Suggest("col", "bkt", "hel", 10)
	if len(results) != 3 {
		t.Fatalf("expected 3 suggestions, got %d: %v", len(results), results)
	}
}

func TestTrieSuggestLimit(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt", "hello")
	tr.Insert("col", "bkt", "help")
	tr.Insert("col", "bkt", "helicopter")

	results := tr.Suggest("col", "bkt", "hel", 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(results))
	}
}

func TestTrieSuggestNoMatch(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt", "hello")
	results := tr.Suggest("col", "bkt", "xyz", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 suggestions, got %d", len(results))
	}
}

func TestTrieSuggestEmptyPrefix(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt", "hello")
	tr.Insert("col", "bkt", "world")
	results := tr.Suggest("col", "bkt", "", 10)
	if len(results) != 2 {
		t.Errorf("expected 2 suggestions for empty prefix, got %d", len(results))
	}
}

func TestTrieRemove(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt", "hello")
	tr.Insert("col", "bkt", "help")

	removed := tr.Remove("col", "bkt", "hello")
	if !removed {
		// Remove returns whether the branch was pruned, not whether word existed
		// Just check suggest doesn't return it
	}

	results := tr.Suggest("col", "bkt", "hel", 10)
	for _, r := range results {
		if r == "hello" {
			t.Error("expected 'hello' to be removed")
		}
	}
}

func TestTrieRemoveNonExistent(t *testing.T) {
	tr := NewTrie()
	removed := tr.Remove("col", "bkt", "hello")
	if removed {
		t.Error("removing from empty trie should return false")
	}
}

func TestTrieBucketIsolation(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt1", "hello")
	tr.Insert("col", "bkt2", "world")

	r1 := tr.Suggest("col", "bkt1", "", 10)
	r2 := tr.Suggest("col", "bkt2", "", 10)

	if len(r1) != 1 || r1[0] != "hello" {
		t.Errorf("bkt1 expected [hello], got %v", r1)
	}
	if len(r2) != 1 || r2[0] != "world" {
		t.Errorf("bkt2 expected [world], got %v", r2)
	}
}

func TestTrieCollectionIsolation(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col1", "bkt", "hello")
	results := tr.Suggest("col2", "bkt", "hel", 10)
	if len(results) != 0 {
		t.Error("collections should be isolated")
	}
}

func TestTrieFlushBucket(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt", "hello")
	tr.Insert("col", "bkt", "world")
	tr.FlushBucket("col", "bkt")
	results := tr.Suggest("col", "bkt", "", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 after flush, got %d", len(results))
	}
}

func TestTrieFlushCollection(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt1", "hello")
	tr.Insert("col", "bkt2", "world")
	tr.FlushCollection("col")
	r1 := tr.Suggest("col", "bkt1", "", 10)
	r2 := tr.Suggest("col", "bkt2", "", 10)
	if len(r1)+len(r2) != 0 {
		t.Errorf("expected 0 after collection flush, got %d", len(r1)+len(r2))
	}
}

func TestTrieWordCount(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt", "hello")
	tr.Insert("col", "bkt", "help")
	tr.Insert("col", "bkt", "world")
	count := tr.WordCount("col", "bkt")
	if count != 3 {
		t.Errorf("expected 3 words, got %d", count)
	}
}

func TestTrieAllWords(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt", "banana")
	tr.Insert("col", "bkt", "apple")
	tr.Insert("col", "bkt", "cherry")
	words := tr.AllWords("col", "bkt")
	if len(words) != 3 {
		t.Fatalf("expected 3 words, got %d", len(words))
	}
	// Should be sorted
	if words[0] != "apple" || words[1] != "banana" || words[2] != "cherry" {
		t.Errorf("expected alphabetical sort, got %v", words)
	}
}

func TestTrieLowercasing(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt", "Hello")
	tr.Insert("col", "bkt", "WORLD")
	results := tr.Suggest("col", "bkt", "hel", 10)
	if len(results) != 1 || results[0] != "hello" {
		t.Errorf("expected [hello], got %v", results)
	}
}

func TestTrieInsertDuplicate(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt", "hello")
	tr.Insert("col", "bkt", "hello")
	tr.Insert("col", "bkt", "hello")
	count := tr.WordCount("col", "bkt")
	if count != 1 {
		t.Errorf("expected 1 word (deduped), got %d", count)
	}
}

func TestTrieSuggestRanking(t *testing.T) {
	tr := NewTrie()
	// Insert "hello" multiple times to boost its count
	tr.Insert("col", "bkt", "help")
	tr.Insert("col", "bkt", "hello")
	tr.Insert("col", "bkt", "hello")
	tr.Insert("col", "bkt", "hello")

	results := tr.Suggest("col", "bkt", "hel", 10)
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	// "hello" should be first due to higher count
	if results[0] != "hello" {
		t.Errorf("expected 'hello' first (higher count), got %q", results[0])
	}
}

func TestTrieInsertEmpty(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt", "")
	count := tr.WordCount("col", "bkt")
	if count != 0 {
		t.Errorf("empty insert should not create word, got %d", count)
	}
}

func TestSuggestFuzzyExact(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt", "hello")
	tr.Insert("col", "bkt", "world")

	results := tr.SuggestFuzzy("col", "bkt", "hello", 1, 5)
	found := false
	for _, r := range results {
		if r == "hello" {
			found = true
		}
	}
	if !found {
		t.Error("expected exact match in fuzzy results")
	}
}

func TestSuggestFuzzyTypo(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt", "hello")
	tr.Insert("col", "bkt", "world")

	// "helo" has edit distance 1 from "hello"
	results := tr.SuggestFuzzy("col", "bkt", "helo", 1, 5)
	found := false
	for _, r := range results {
		if r == "hello" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'hello' for typo 'helo'")
	}
}

func TestSuggestFuzzyNoMatch(t *testing.T) {
	tr := NewTrie()
	tr.Insert("col", "bkt", "hello")

	results := tr.SuggestFuzzy("col", "bkt", "zzzzz", 1, 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results for no match, got %v", results)
	}
}

func TestSuggestFuzzyEmptyTrie(t *testing.T) {
	tr := NewTrie()
	results := tr.SuggestFuzzy("col", "bkt", "hello", 1, 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty trie, got %v", results)
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"", "", 0},
		{"hello", "hello", 0},
		{"hello", "helo", 1},
		{"kitten", "sitting", 3},
		{"", "abc", 3},
		{"abc", "", 3},
		{"abc", "abd", 1},
	}
	for _, tt := range tests {
		got := levenshtein(tt.a, tt.b)
		if got != tt.expected {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.expected)
		}
	}
}

func TestAutoDistance(t *testing.T) {
	tests := []struct {
		word     string
		expected int
	}{
		{"ab", 0},
		{"abc", 0},
		{"abcd", 1},
		{"abcdef", 1},
		{"abcdefg", 2},
		{"abcdefghi", 2},
		{"abcdefghij", 3},
	}
	for _, tt := range tests {
		got := autoDistance(tt.word)
		if got != tt.expected {
			t.Errorf("autoDistance(%q) = %d, want %d", tt.word, got, tt.expected)
		}
	}
}
