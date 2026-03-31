package search

import (
	"fmt"
	"testing"

	"github.com/JSLEEKR/sonic-go/pkg/store"
	"github.com/JSLEEKR/sonic-go/pkg/suggest"
)

func setupEngine(t *testing.T) (*Engine, *store.Store, *suggest.Trie) {
	t.Helper()
	s := store.New(t.TempDir())
	tr := suggest.NewTrie()
	eng := New(s, tr)
	return eng, s, tr
}

func TestQuerySingleTerm(t *testing.T) {
	eng, s, tr := setupEngine(t)

	iid1, _ := s.ResolveOID("col", "bkt", "doc1")
	iid2, _ := s.ResolveOID("col", "bkt", "doc2")

	h := fnvHash("hello")
	s.AddTermIID("col", "bkt", h, iid1, 1000)
	s.AddTermIID("col", "bkt", h, iid2, 1000)
	s.AddIIDTerm("col", "bkt", iid1, h)
	s.AddIIDTerm("col", "bkt", iid2, h)
	tr.Insert("col", "bkt", "hello")

	results := eng.QuerySingle("col", "bkt", "hello", 10)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestQueryMultiTermAND(t *testing.T) {
	eng, s, _ := setupEngine(t)

	iid1, _ := s.ResolveOID("col", "bkt", "doc1")
	iid2, _ := s.ResolveOID("col", "bkt", "doc2")

	hHello := fnvHash("hello")
	hWorld := fnvHash("world")

	// doc1 has "hello" and "world"
	s.AddTermIID("col", "bkt", hHello, iid1, 1000)
	s.AddTermIID("col", "bkt", hWorld, iid1, 1000)
	// doc2 has only "hello"
	s.AddTermIID("col", "bkt", hHello, iid2, 1000)

	// Query "hello world" - should only return doc1 (AND logic)
	results := eng.Query("col", "bkt", "hello world", DefaultQueryOptions())
	if len(results) != 1 {
		t.Fatalf("expected 1 result (AND intersection), got %d: %v", len(results), results)
	}
	if results[0] != "doc1" {
		t.Errorf("expected doc1, got %s", results[0])
	}
}

func TestQueryNoResults(t *testing.T) {
	eng, _, _ := setupEngine(t)
	results := eng.Query("col", "bkt", "nonexistent", DefaultQueryOptions())
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestQueryEmptyText(t *testing.T) {
	eng, _, _ := setupEngine(t)
	results := eng.Query("col", "bkt", "", DefaultQueryOptions())
	if results != nil {
		t.Errorf("expected nil for empty query, got %v", results)
	}
}

func TestQueryLimit(t *testing.T) {
	eng, s, _ := setupEngine(t)

	h := fnvHash("test")
	for i := uint32(1); i <= 5; i++ {
		iid, _ := s.ResolveOID("col", "bkt", fmt.Sprintf("doc%d", i))
		s.AddTermIID("col", "bkt", h, iid, 1000)
	}

	opts := QueryOptions{Limit: 3}
	results := eng.Query("col", "bkt", "test", opts)
	if len(results) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(results))
	}
}

func TestQueryOffset(t *testing.T) {
	eng, s, _ := setupEngine(t)

	h := fnvHash("test")
	for i := uint32(1); i <= 5; i++ {
		iid, _ := s.ResolveOID("col", "bkt", fmt.Sprintf("doc%d", i))
		s.AddTermIID("col", "bkt", h, iid, 1000)
	}

	opts := QueryOptions{Limit: 10, Offset: 3}
	results := eng.Query("col", "bkt", "test", opts)
	if len(results) > 2 {
		t.Errorf("expected at most 2 results with offset 3 from 5, got %d", len(results))
	}
}

func TestQueryOffsetBeyondResults(t *testing.T) {
	eng, s, _ := setupEngine(t)

	h := fnvHash("test")
	iid, _ := s.ResolveOID("col", "bkt", "doc1")
	s.AddTermIID("col", "bkt", h, iid, 1000)

	opts := QueryOptions{Limit: 10, Offset: 100}
	results := eng.Query("col", "bkt", "test", opts)
	if results != nil {
		t.Errorf("expected nil for offset beyond results, got %v", results)
	}
}

func TestSuggest(t *testing.T) {
	eng, _, tr := setupEngine(t)
	tr.Insert("col", "bkt", "hello")
	tr.Insert("col", "bkt", "help")

	results := eng.Suggest("col", "bkt", "hel", 10)
	if len(results) != 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(results))
	}
}

func TestSuggestEmpty(t *testing.T) {
	eng, _, _ := setupEngine(t)
	results := eng.Suggest("col", "bkt", "xyz", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 suggestions, got %d", len(results))
	}
}

func TestIntersectEmpty(t *testing.T) {
	result := intersect(nil)
	if result != nil {
		t.Error("expected nil for empty intersection")
	}
}

func TestIntersectSingle(t *testing.T) {
	sets := []map[uint32]struct{}{
		{1: {}, 2: {}, 3: {}},
	}
	result := intersect(sets)
	if len(result) != 3 {
		t.Errorf("expected 3, got %d", len(result))
	}
}

func TestIntersectMultiple(t *testing.T) {
	sets := []map[uint32]struct{}{
		{1: {}, 2: {}, 3: {}},
		{2: {}, 3: {}, 4: {}},
		{3: {}, 4: {}, 5: {}},
	}
	result := intersect(sets)
	if len(result) != 1 {
		t.Fatalf("expected 1 (only 3), got %d", len(result))
	}
	if _, ok := result[3]; !ok {
		t.Error("expected 3 in result")
	}
}

func TestIntersectNoOverlap(t *testing.T) {
	sets := []map[uint32]struct{}{
		{1: {}, 2: {}},
		{3: {}, 4: {}},
	}
	result := intersect(sets)
	if len(result) != 0 {
		t.Errorf("expected 0 for no overlap, got %d", len(result))
	}
}

func TestQueryDeterministicOrdering(t *testing.T) {
	eng, s, _ := setupEngine(t)

	h := fnvHash("common")
	for i := uint32(1); i <= 20; i++ {
		iid, _ := s.ResolveOID("col", "bkt", fmt.Sprintf("doc%02d", i))
		s.AddTermIID("col", "bkt", h, iid, 1000)
	}

	opts := QueryOptions{Limit: 5, Offset: 0}
	first := eng.Query("col", "bkt", "common", opts)
	for trial := 0; trial < 10; trial++ {
		got := eng.Query("col", "bkt", "common", opts)
		if len(got) != len(first) {
			t.Fatalf("trial %d: length mismatch %d vs %d", trial, len(got), len(first))
		}
		for i := range first {
			if got[i] != first[i] {
				t.Fatalf("trial %d: non-deterministic ordering at index %d: %v vs %v", trial, i, got, first)
			}
		}
	}
}

func TestQueryPaginationNoOverlap(t *testing.T) {
	eng, s, _ := setupEngine(t)

	h := fnvHash("common")
	for i := uint32(1); i <= 10; i++ {
		iid, _ := s.ResolveOID("col", "bkt", fmt.Sprintf("doc%02d", i))
		s.AddTermIID("col", "bkt", h, iid, 1000)
	}

	page1 := eng.Query("col", "bkt", "common", QueryOptions{Limit: 5, Offset: 0})
	page2 := eng.Query("col", "bkt", "common", QueryOptions{Limit: 5, Offset: 5})

	seen := make(map[string]bool)
	for _, oid := range page1 {
		seen[oid] = true
	}
	for _, oid := range page2 {
		if seen[oid] {
			t.Errorf("pagination overlap: %s appears on both pages", oid)
		}
	}

	// Together they should cover all 10 docs
	all := make(map[string]bool)
	for _, oid := range page1 {
		all[oid] = true
	}
	for _, oid := range page2 {
		all[oid] = true
	}
	if len(all) != 10 {
		t.Errorf("expected 10 unique docs across 2 pages, got %d", len(all))
	}
}

// fnvHash replicates the lexer's FNV-1a hash for test setup.
func fnvHash(s string) uint32 {
	const (
		offset32 = uint32(2166136261)
		prime32  = uint32(16777619)
	)
	h := offset32
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime32
	}
	return h
}
