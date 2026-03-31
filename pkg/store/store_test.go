package store

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return New(dir)
}

func TestResolveOIDNew(t *testing.T) {
	s := newTestStore(t)
	iid, created := s.ResolveOID("col", "bkt", "obj1")
	if !created {
		t.Error("expected created=true for new OID")
	}
	if iid == 0 {
		t.Error("expected non-zero IID")
	}
}

func TestResolveOIDExisting(t *testing.T) {
	s := newTestStore(t)
	iid1, _ := s.ResolveOID("col", "bkt", "obj1")
	iid2, created := s.ResolveOID("col", "bkt", "obj1")
	if created {
		t.Error("expected created=false for existing OID")
	}
	if iid1 != iid2 {
		t.Errorf("expected same IID, got %d and %d", iid1, iid2)
	}
}

func TestResolveOIDMonotonic(t *testing.T) {
	s := newTestStore(t)
	iid1, _ := s.ResolveOID("col", "bkt", "obj1")
	iid2, _ := s.ResolveOID("col", "bkt", "obj2")
	if iid2 <= iid1 {
		t.Errorf("IIDs should be monotonically increasing: %d, %d", iid1, iid2)
	}
}

func TestGetIIDForOID(t *testing.T) {
	s := newTestStore(t)
	s.ResolveOID("col", "bkt", "obj1")
	iid, ok := s.GetIIDForOID("col", "bkt", "obj1")
	if !ok {
		t.Error("expected to find OID")
	}
	if iid == 0 {
		t.Error("expected non-zero IID")
	}
}

func TestGetIIDForOIDNotFound(t *testing.T) {
	s := newTestStore(t)
	_, ok := s.GetIIDForOID("col", "bkt", "missing")
	if ok {
		t.Error("expected not found")
	}
}

func TestGetOIDForIID(t *testing.T) {
	s := newTestStore(t)
	iid, _ := s.ResolveOID("col", "bkt", "obj1")
	oid, ok := s.GetOIDForIID("col", "bkt", iid)
	if !ok {
		t.Error("expected to find IID")
	}
	if oid != "obj1" {
		t.Errorf("expected 'obj1', got %q", oid)
	}
}

func TestAddTermIID(t *testing.T) {
	s := newTestStore(t)
	s.AddTermIID("col", "bkt", 123, 1, 1000)
	iids := s.GetTermIIDs("col", "bkt", 123)
	if len(iids) != 1 || iids[0] != 1 {
		t.Errorf("expected [1], got %v", iids)
	}
}

func TestAddTermIIDPrepend(t *testing.T) {
	s := newTestStore(t)
	s.AddTermIID("col", "bkt", 123, 1, 1000)
	s.AddTermIID("col", "bkt", 123, 2, 1000)
	iids := s.GetTermIIDs("col", "bkt", 123)
	if len(iids) != 2 {
		t.Fatalf("expected 2 IIDs, got %d", len(iids))
	}
	if iids[0] != 2 {
		t.Errorf("most recent IID should be first, got %v", iids)
	}
}

func TestAddTermIIDDedup(t *testing.T) {
	s := newTestStore(t)
	s.AddTermIID("col", "bkt", 123, 1, 1000)
	s.AddTermIID("col", "bkt", 123, 1, 1000)
	iids := s.GetTermIIDs("col", "bkt", 123)
	if len(iids) != 1 {
		t.Errorf("expected 1 IID (deduped), got %d", len(iids))
	}
}

func TestAddTermIIDRetain(t *testing.T) {
	s := newTestStore(t)
	for i := uint32(1); i <= 5; i++ {
		s.AddTermIID("col", "bkt", 123, i, 3)
	}
	iids := s.GetTermIIDs("col", "bkt", 123)
	if len(iids) != 3 {
		t.Errorf("expected 3 IIDs (truncated), got %d", len(iids))
	}
}

func TestRemoveTermIID(t *testing.T) {
	s := newTestStore(t)
	s.AddTermIID("col", "bkt", 123, 1, 1000)
	s.AddTermIID("col", "bkt", 123, 2, 1000)
	empty := s.RemoveTermIID("col", "bkt", 123, 1)
	if empty {
		t.Error("should not be empty")
	}
	iids := s.GetTermIIDs("col", "bkt", 123)
	if len(iids) != 1 || iids[0] != 2 {
		t.Errorf("expected [2], got %v", iids)
	}
}

func TestRemoveTermIIDEmpty(t *testing.T) {
	s := newTestStore(t)
	s.AddTermIID("col", "bkt", 123, 1, 1000)
	empty := s.RemoveTermIID("col", "bkt", 123, 1)
	if !empty {
		t.Error("should be empty")
	}
}

func TestSetGetIIDTerms(t *testing.T) {
	s := newTestStore(t)
	terms := []uint32{100, 200, 300}
	s.SetIIDTerms("col", "bkt", 1, terms)
	got := s.GetIIDTerms("col", "bkt", 1)
	if len(got) != 3 {
		t.Fatalf("expected 3 terms, got %d", len(got))
	}
}

func TestAddIIDTerm(t *testing.T) {
	s := newTestStore(t)
	s.AddIIDTerm("col", "bkt", 1, 100)
	s.AddIIDTerm("col", "bkt", 1, 200)
	s.AddIIDTerm("col", "bkt", 1, 100) // dedup
	got := s.GetIIDTerms("col", "bkt", 1)
	if len(got) != 2 {
		t.Errorf("expected 2 terms, got %d", len(got))
	}
}

func TestRemoveObject(t *testing.T) {
	s := newTestStore(t)
	iid, _ := s.ResolveOID("col", "bkt", "obj1")
	s.AddTermIID("col", "bkt", 100, iid, 1000)
	s.AddTermIID("col", "bkt", 200, iid, 1000)
	s.SetIIDTerms("col", "bkt", iid, []uint32{100, 200})

	terms := s.RemoveObject("col", "bkt", "obj1")
	if len(terms) != 2 {
		t.Errorf("expected 2 removed terms, got %d", len(terms))
	}

	// Verify cleaned up
	_, ok := s.GetIIDForOID("col", "bkt", "obj1")
	if ok {
		t.Error("OID should be removed")
	}
}

func TestCountObjects(t *testing.T) {
	s := newTestStore(t)
	s.ResolveOID("col", "bkt", "obj1")
	s.ResolveOID("col", "bkt", "obj2")
	count := s.CountObjects("col", "bkt")
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestCountTerms(t *testing.T) {
	s := newTestStore(t)
	s.AddTermIID("col", "bkt", 100, 1, 1000)
	s.AddTermIID("col", "bkt", 200, 2, 1000)
	count := s.CountTerms("col", "bkt")
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestFlushBucket(t *testing.T) {
	s := newTestStore(t)
	s.ResolveOID("col", "bkt", "obj1")
	s.AddTermIID("col", "bkt", 100, 1, 1000)
	s.FlushBucket("col", "bkt")
	count := s.CountObjects("col", "bkt")
	if count != 0 {
		t.Errorf("expected 0 after flush, got %d", count)
	}
}

func TestFlushCollection(t *testing.T) {
	s := newTestStore(t)
	s.ResolveOID("col", "bkt1", "obj1")
	s.ResolveOID("col", "bkt2", "obj2")
	s.FlushCollection("col")
	count1 := s.CountObjects("col", "bkt1")
	count2 := s.CountObjects("col", "bkt2")
	if count1+count2 != 0 {
		t.Errorf("expected 0 after flush, got %d", count1+count2)
	}
}

func TestSaveLoadDisk(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	s.ResolveOID("col", "bkt", "obj1")
	s.AddTermIID("col", "bkt", 100, 1, 1000)
	s.SetIIDTerms("col", "bkt", 1, []uint32{100})

	if err := s.SaveToDisk(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, "store.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("store file not created: %v", err)
	}

	// Load into new store
	s2 := New(dir)
	if err := s2.LoadFromDisk(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	iid, ok := s2.GetIIDForOID("col", "bkt", "obj1")
	if !ok {
		t.Error("expected to find OID after load")
	}
	iids := s2.GetTermIIDs("col", "bkt", 100)
	if len(iids) != 1 || iids[0] != iid {
		t.Errorf("expected IID %d in term list, got %v", iid, iids)
	}
}

func TestLoadFromDiskMissing(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	err := s.LoadFromDisk()
	if err != nil {
		t.Errorf("expected nil error for missing file, got %v", err)
	}
}

func TestListCollections(t *testing.T) {
	s := newTestStore(t)
	s.ResolveOID("col1", "bkt", "obj1")
	s.ResolveOID("col2", "bkt", "obj2")
	cols := s.ListCollections()
	if len(cols) != 2 {
		t.Errorf("expected 2 collections, got %d", len(cols))
	}
}

func TestListBuckets(t *testing.T) {
	s := newTestStore(t)
	s.ResolveOID("col", "bkt1", "obj1")
	s.ResolveOID("col", "bkt2", "obj2")
	bkts := s.ListBuckets("col")
	if len(bkts) != 2 {
		t.Errorf("expected 2 buckets, got %d", len(bkts))
	}
}

func TestFlushCollectionCleansIIDTerms(t *testing.T) {
	s := newTestStore(t)
	iid, _ := s.ResolveOID("col", "bkt", "obj1")
	s.AddTermIID("col", "bkt", 100, iid, 1000)
	s.AddIIDTerm("col", "bkt", iid, 100)
	s.SetHashWord("col", "bkt", 100, "hello")

	s.FlushCollection("col")

	// iidToTerms entries must be cleaned up (regression: "it:" prefix is 3 chars)
	terms := s.GetIIDTerms("col", "bkt", iid)
	if len(terms) != 0 {
		t.Errorf("expected 0 IID terms after FlushCollection, got %v", terms)
	}

	// hashToWord entries must also be cleaned up
	_, ok := s.GetWordForHash("col", "bkt", 100)
	if ok {
		t.Error("expected hash-to-word mapping to be removed after FlushCollection")
	}

	// counters must be cleaned up
	iid2, created := s.ResolveOID("col", "bkt", "obj2")
	if !created {
		t.Error("expected new OID after flush")
	}
	if iid2 == 0 {
		t.Error("expected non-zero IID")
	}
}

func TestGetTermIIDsReturnsNilForMissing(t *testing.T) {
	s := newTestStore(t)
	iids := s.GetTermIIDs("col", "bkt", 999)
	if iids != nil {
		t.Errorf("expected nil for missing term, got %v", iids)
	}
}

func TestGetTermIIDsReturnsCopy(t *testing.T) {
	s := newTestStore(t)
	s.AddTermIID("col", "bkt", 100, 1, 1000)
	iids := s.GetTermIIDs("col", "bkt", 100)
	iids[0] = 999 // mutate copy
	original := s.GetTermIIDs("col", "bkt", 100)
	if original[0] != 1 {
		t.Error("GetTermIIDs should return a copy, not a reference")
	}
}

func TestBucketIsolation(t *testing.T) {
	s := newTestStore(t)
	s.AddTermIID("col", "bkt1", 100, 1, 1000)
	iids := s.GetTermIIDs("col", "bkt2", 100)
	if iids != nil {
		t.Error("buckets should be isolated")
	}
}

func TestCollectionIsolation(t *testing.T) {
	s := newTestStore(t)
	s.ResolveOID("col1", "bkt", "obj1")
	_, ok := s.GetIIDForOID("col2", "bkt", "obj1")
	if ok {
		t.Error("collections should be isolated")
	}
}
