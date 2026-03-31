package index

import (
	"testing"

	"github.com/JSLEEKR/sonic-go/pkg/store"
	"github.com/JSLEEKR/sonic-go/pkg/suggest"
)

func setupIndex(t *testing.T) (*Index, *store.Store, *suggest.Trie) {
	t.Helper()
	s := store.New(t.TempDir())
	tr := suggest.NewTrie()
	idx := New(s, tr, 1000)
	return idx, s, tr
}

func TestPushBasic(t *testing.T) {
	idx, _, _ := setupIndex(t)
	count := idx.Push("col", "bkt", "doc1", "hello world test")
	if count == 0 {
		t.Error("expected non-zero push count")
	}
}

func TestPushEmpty(t *testing.T) {
	idx, _, _ := setupIndex(t)
	count := idx.Push("col", "bkt", "doc1", "")
	if count != 0 {
		t.Errorf("expected 0 for empty text, got %d", count)
	}
}

func TestPushAndSearch(t *testing.T) {
	idx, s, _ := setupIndex(t)
	idx.Push("col", "bkt", "doc1", "hello world programming")
	idx.Push("col", "bkt", "doc2", "hello golang testing")

	// Both docs have "hello", check via store
	iid1, _ := s.GetIIDForOID("col", "bkt", "doc1")
	iid2, _ := s.GetIIDForOID("col", "bkt", "doc2")
	if iid1 == 0 || iid2 == 0 {
		t.Error("expected both docs to be indexed")
	}
}

func TestPushPopulatesTrie(t *testing.T) {
	idx, _, tr := setupIndex(t)
	idx.Push("col", "bkt", "doc1", "helicopter landing")

	words := tr.AllWords("col", "bkt")
	found := false
	for _, w := range words {
		if w == "helicopter" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'helicopter' in trie, got %v", words)
	}
}

func TestPopAll(t *testing.T) {
	idx, s, _ := setupIndex(t)
	idx.Push("col", "bkt", "doc1", "hello world")
	count := idx.Pop("col", "bkt", "doc1", "")
	if count == 0 {
		t.Error("expected non-zero pop count")
	}
	_, ok := s.GetIIDForOID("col", "bkt", "doc1")
	if ok {
		t.Error("doc should be removed after pop all")
	}
}

func TestPopPartial(t *testing.T) {
	idx, s, _ := setupIndex(t)
	idx.Push("col", "bkt", "doc1", "hello world programming")
	idx.Pop("col", "bkt", "doc1", "hello")

	// doc1 should still exist (has "world" and "programming")
	iid, ok := s.GetIIDForOID("col", "bkt", "doc1")
	if !ok {
		t.Error("doc should still exist after partial pop")
	}

	terms := s.GetIIDTerms("col", "bkt", iid)
	if len(terms) == 0 {
		t.Error("should still have some terms")
	}
}

func TestPopNonExistent(t *testing.T) {
	idx, _, _ := setupIndex(t)
	count := idx.Pop("col", "bkt", "missing", "hello")
	if count != 0 {
		t.Errorf("expected 0 for non-existent doc, got %d", count)
	}
}

func TestCount(t *testing.T) {
	idx, _, _ := setupIndex(t)
	idx.Push("col", "bkt", "doc1", "hello world")
	idx.Push("col", "bkt", "doc2", "hello golang")
	count := idx.Count("col", "bkt")
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestFlushBucket(t *testing.T) {
	idx, _, _ := setupIndex(t)
	idx.Push("col", "bkt", "doc1", "hello world")
	idx.Push("col", "bkt", "doc2", "hello golang")
	idx.FlushBucket("col", "bkt")
	count := idx.Count("col", "bkt")
	if count != 0 {
		t.Errorf("expected 0 after flush, got %d", count)
	}
}

func TestFlushCollection(t *testing.T) {
	idx, _, _ := setupIndex(t)
	idx.Push("col", "bkt1", "doc1", "hello world")
	idx.Push("col", "bkt2", "doc2", "hello golang")
	idx.FlushCollection("col")
	c1 := idx.Count("col", "bkt1")
	c2 := idx.Count("col", "bkt2")
	if c1+c2 != 0 {
		t.Errorf("expected 0 after collection flush, got %d", c1+c2)
	}
}

func TestFlushObject(t *testing.T) {
	idx, _, _ := setupIndex(t)
	idx.Push("col", "bkt", "doc1", "hello world")
	idx.Push("col", "bkt", "doc2", "hello golang")
	idx.FlushObject("col", "bkt", "doc1")
	count := idx.Count("col", "bkt")
	if count != 1 {
		t.Errorf("expected 1 after flushing doc1, got %d", count)
	}
}

func TestPushDuplicateObject(t *testing.T) {
	idx, _, _ := setupIndex(t)
	idx.Push("col", "bkt", "doc1", "hello world")
	idx.Push("col", "bkt", "doc1", "hello world extra")
	count := idx.Count("col", "bkt")
	if count != 1 {
		t.Errorf("expected 1 (same object), got %d", count)
	}
}

func TestDefaultRetainWordObjects(t *testing.T) {
	s := store.New(t.TempDir())
	tr := suggest.NewTrie()
	idx := New(s, tr, 0) // 0 should default to 1000
	if idx.retainWordObjs != 1000 {
		t.Errorf("expected default 1000, got %d", idx.retainWordObjs)
	}
}

func TestMultipleBuckets(t *testing.T) {
	idx, _, _ := setupIndex(t)
	idx.Push("col", "bkt1", "doc1", "hello world")
	idx.Push("col", "bkt2", "doc2", "hello golang")

	c1 := idx.Count("col", "bkt1")
	c2 := idx.Count("col", "bkt2")
	if c1 != 1 || c2 != 1 {
		t.Errorf("expected 1 each, got %d and %d", c1, c2)
	}
}

func TestMultipleCollections(t *testing.T) {
	idx, _, _ := setupIndex(t)
	idx.Push("col1", "bkt", "doc1", "hello world")
	idx.Push("col2", "bkt", "doc2", "hello golang")

	c1 := idx.Count("col1", "bkt")
	c2 := idx.Count("col2", "bkt")
	if c1 != 1 || c2 != 1 {
		t.Errorf("expected 1 each, got %d and %d", c1, c2)
	}
}

func TestPopAllCleansUpTrie(t *testing.T) {
	idx, _, tr := setupIndex(t)
	idx.Push("col", "bkt", "doc1", "helicopter landing")

	// Verify trie has the words
	words := tr.AllWords("col", "bkt")
	if len(words) == 0 {
		t.Fatal("expected words in trie after push")
	}

	// Pop all terms for doc1 (text="") — trie should be cleaned up
	count := idx.Pop("col", "bkt", "doc1", "")
	if count == 0 {
		t.Fatal("expected non-zero pop count")
	}

	words = tr.AllWords("col", "bkt")
	if len(words) != 0 {
		t.Errorf("expected 0 words in trie after popping only object, got %v", words)
	}
}

func TestPopAllTriePreservesSharedTerms(t *testing.T) {
	idx, _, tr := setupIndex(t)
	idx.Push("col", "bkt", "doc1", "hello world")
	idx.Push("col", "bkt", "doc2", "hello golang")

	// Pop all for doc1 — "hello" is shared with doc2, should remain
	idx.Pop("col", "bkt", "doc1", "")

	words := tr.AllWords("col", "bkt")
	found := false
	for _, w := range words {
		if w == "hello" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'hello' to remain in trie (shared by doc2), got %v", words)
	}
}

func TestFlushObjectCleansUpTrie(t *testing.T) {
	idx, _, tr := setupIndex(t)
	idx.Push("col", "bkt", "doc1", "helicopter landing")

	// Verify trie has the words
	words := tr.AllWords("col", "bkt")
	if len(words) == 0 {
		t.Fatal("expected words in trie after push")
	}

	// Flush the only object — trie should be cleaned up
	idx.FlushObject("col", "bkt", "doc1")

	words = tr.AllWords("col", "bkt")
	if len(words) != 0 {
		t.Errorf("expected 0 words in trie after flushing only object, got %v", words)
	}
}

func TestPopPartialRemovesOrphanedOID(t *testing.T) {
	idx, s, _ := setupIndex(t)
	idx.Push("col", "bkt", "doc1", "helicopter landing")

	// Pop all terms via partial pop (specifying exact text)
	idx.Pop("col", "bkt", "doc1", "helicopter landing")

	// doc1 should no longer exist in OID mappings
	_, ok := s.GetIIDForOID("col", "bkt", "doc1")
	if ok {
		t.Error("doc1 OID should be removed after all terms are popped via partial pop")
	}

	// Count should be 0
	count := idx.Count("col", "bkt")
	if count != 0 {
		t.Errorf("expected count 0 after removing all terms, got %d", count)
	}
}

func TestPopPartialTrieCleanupWithMultipleDocs(t *testing.T) {
	idx, _, tr := setupIndex(t)
	idx.Push("col", "bkt", "doc1", "hello world")
	idx.Push("col", "bkt", "doc2", "hello golang")

	// Pop "hello" from doc1
	idx.Pop("col", "bkt", "doc1", "hello")

	// "hello" should still be in trie (doc2 still has it)
	words := tr.AllWords("col", "bkt")
	foundHello := false
	for _, w := range words {
		if w == "hello" {
			foundHello = true
		}
	}
	if !foundHello {
		t.Error("hello should remain in trie while doc2 still has it")
	}

	// Pop "hello" from doc2 - now no document has "hello"
	idx.Pop("col", "bkt", "doc2", "hello")

	// "hello" should be GONE from trie
	words = tr.AllWords("col", "bkt")
	for _, w := range words {
		if w == "hello" {
			t.Error("hello should be removed from trie after all documents popped it")
		}
	}
}

func TestFlushObjectTriePreservesSharedTerms(t *testing.T) {
	idx, _, tr := setupIndex(t)
	idx.Push("col", "bkt", "doc1", "hello world")
	idx.Push("col", "bkt", "doc2", "hello golang")

	// Flush doc1 — "hello" is shared, should remain in trie
	idx.FlushObject("col", "bkt", "doc1")

	words := tr.AllWords("col", "bkt")
	found := false
	for _, w := range words {
		if w == "hello" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'hello' to remain in trie (shared by doc2), got %v", words)
	}
}
