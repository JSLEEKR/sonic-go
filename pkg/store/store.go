// Package store provides the storage backend for sonic-go.
// It uses an in-memory store with JSON persistence to disk.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store is the primary key-value storage for inverted index data.
// It is safe for concurrent use.
type Store struct {
	mu      sync.RWMutex
	dataDir string

	// termToIIDs maps collection:bucket:termHash -> list of IIDs (most recent first)
	termToIIDs map[string][]uint32

	// oidToIID maps collection:bucket:oidHash -> IID
	oidToIID map[string]uint32

	// iidToOID maps collection:bucket:IID -> original OID string
	iidToOID map[string]string

	// iidToTerms maps collection:bucket:IID -> list of term hashes
	iidToTerms map[string][]uint32

	// iidCounters tracks the next IID for each collection:bucket
	iidCounters map[string]uint32

	// hashToWord maps collection:bucket:termHash -> original word (for trie cleanup)
	hashToWord map[string]string
}

// New creates a new Store with the given data directory.
func New(dataDir string) *Store {
	return &Store{
		dataDir:     dataDir,
		termToIIDs:  make(map[string][]uint32),
		oidToIID:    make(map[string]uint32),
		iidToOID:    make(map[string]string),
		iidToTerms:  make(map[string][]uint32),
		iidCounters: make(map[string]uint32),
		hashToWord:  make(map[string]string),
	}
}

// --- Key helpers ---

func termKey(collection, bucket string, termHash uint32) string {
	return fmt.Sprintf("t:%s:%s:%d", collection, bucket, termHash)
}

func oidKey(collection, bucket, oid string) string {
	return fmt.Sprintf("o:%s:%s:%s", collection, bucket, oid)
}

func iidOIDKey(collection, bucket string, iid uint32) string {
	return fmt.Sprintf("i:%s:%s:%d", collection, bucket, iid)
}

func iidTermsKey(collection, bucket string, iid uint32) string {
	return fmt.Sprintf("it:%s:%s:%d", collection, bucket, iid)
}

func counterKey(collection, bucket string) string {
	return fmt.Sprintf("c:%s:%s", collection, bucket)
}

func hashWordKey(collection, bucket string, termHash uint32) string {
	return fmt.Sprintf("hw:%s:%s:%d", collection, bucket, termHash)
}

// --- IID management ---

// ResolveOID returns the IID for an object, creating one if needed.
// Returns (iid, created).
func (s *Store) ResolveOID(collection, bucket, oid string) (uint32, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := oidKey(collection, bucket, oid)
	if iid, ok := s.oidToIID[key]; ok {
		return iid, false
	}

	cKey := counterKey(collection, bucket)
	s.iidCounters[cKey]++
	iid := s.iidCounters[cKey]

	s.oidToIID[key] = iid
	s.iidToOID[iidOIDKey(collection, bucket, iid)] = oid

	return iid, true
}

// GetIIDForOID returns the IID for an OID, or 0 and false if not found.
func (s *Store) GetIIDForOID(collection, bucket, oid string) (uint32, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	iid, ok := s.oidToIID[oidKey(collection, bucket, oid)]
	return iid, ok
}

// GetOIDForIID returns the OID for an IID.
func (s *Store) GetOIDForIID(collection, bucket string, iid uint32) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	oid, ok := s.iidToOID[iidOIDKey(collection, bucket, iid)]
	return oid, ok
}

// --- Term-to-IID index ---

// AddTermIID adds an IID to the term's IID list (prepended, most recent first).
// It also deduplicates.
func (s *Store) AddTermIID(collection, bucket string, termHash uint32, iid uint32, maxRetain int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := termKey(collection, bucket, termHash)
	iids := s.termToIIDs[key]

	// Dedup: check if IID already in list
	for _, existing := range iids {
		if existing == iid {
			return
		}
	}

	// Prepend (most recent first)
	newIIDs := make([]uint32, 0, len(iids)+1)
	newIIDs = append(newIIDs, iid)
	newIIDs = append(newIIDs, iids...)

	// Truncate if over limit
	if maxRetain > 0 && len(newIIDs) > maxRetain {
		newIIDs = newIIDs[:maxRetain]
	}

	s.termToIIDs[key] = newIIDs
}

// GetTermIIDs returns the list of IIDs for a term.
func (s *Store) GetTermIIDs(collection, bucket string, termHash uint32) []uint32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := termKey(collection, bucket, termHash)
	iids := s.termToIIDs[key]
	if iids == nil {
		return nil
	}
	out := make([]uint32, len(iids))
	copy(out, iids)
	return out
}

// RemoveTermIID removes an IID from a term's list.
// Returns true if the term list is now empty.
func (s *Store) RemoveTermIID(collection, bucket string, termHash uint32, iid uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := termKey(collection, bucket, termHash)
	iids := s.termToIIDs[key]

	for i, existing := range iids {
		if existing == iid {
			iids = append(iids[:i], iids[i+1:]...)
			break
		}
	}

	if len(iids) == 0 {
		delete(s.termToIIDs, key)
		return true
	}
	s.termToIIDs[key] = iids
	return false
}

// --- IID-to-Terms ---

// SetIIDTerms stores the list of term hashes for an IID.
func (s *Store) SetIIDTerms(collection, bucket string, iid uint32, termHashes []uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := iidTermsKey(collection, bucket, iid)
	if len(termHashes) == 0 {
		delete(s.iidToTerms, key)
	} else {
		out := make([]uint32, len(termHashes))
		copy(out, termHashes)
		s.iidToTerms[key] = out
	}
}

// GetIIDTerms returns the term hashes for an IID.
func (s *Store) GetIIDTerms(collection, bucket string, iid uint32) []uint32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := iidTermsKey(collection, bucket, iid)
	terms := s.iidToTerms[key]
	if terms == nil {
		return nil
	}
	out := make([]uint32, len(terms))
	copy(out, terms)
	return out
}

// AddIIDTerm appends a term hash to an IID's term list (deduped).
func (s *Store) AddIIDTerm(collection, bucket string, iid uint32, termHash uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := iidTermsKey(collection, bucket, iid)
	terms := s.iidToTerms[key]
	for _, t := range terms {
		if t == termHash {
			return
		}
	}
	s.iidToTerms[key] = append(terms, termHash)
}

// --- Hash-to-Word mapping (for trie cleanup) ---

// SetHashWord stores the mapping from a term hash to its original word.
func (s *Store) SetHashWord(collection, bucket string, termHash uint32, word string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hashToWord[hashWordKey(collection, bucket, termHash)] = word
}

// GetWordForHash returns the original word for a term hash.
func (s *Store) GetWordForHash(collection, bucket string, termHash uint32) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	word, ok := s.hashToWord[hashWordKey(collection, bucket, termHash)]
	return word, ok
}

// RemoveHashWord removes the hash-to-word mapping.
func (s *Store) RemoveHashWord(collection, bucket string, termHash uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.hashToWord, hashWordKey(collection, bucket, termHash))
}

// RemoveOIDMapping removes only the OID<->IID mappings for an object,
// without touching the term index. Used when partial Pop has already
// removed all terms individually.
func (s *Store) RemoveOIDMapping(collection, bucket, oid string, iid uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.oidToIID, oidKey(collection, bucket, oid))
	delete(s.iidToOID, iidOIDKey(collection, bucket, iid))
}

// --- Object removal ---

// RemoveObject removes all index entries for an OID.
// Returns the list of term hashes that were associated.
func (s *Store) RemoveObject(collection, bucket, oid string) []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := oidKey(collection, bucket, oid)
	iid, ok := s.oidToIID[key]
	if !ok {
		return nil
	}

	itKey := iidTermsKey(collection, bucket, iid)
	terms := s.iidToTerms[itKey]

	// Remove from term->IID index
	for _, th := range terms {
		key := termKey(collection, bucket, th)
		iids := s.termToIIDs[key]
		for i, existing := range iids {
			if existing == iid {
				iids = append(iids[:i], iids[i+1:]...)
				break
			}
		}
		if len(iids) == 0 {
			delete(s.termToIIDs, key)
		} else {
			s.termToIIDs[key] = iids
		}
	}

	// Remove mappings
	delete(s.oidToIID, oidKey(collection, bucket, oid))
	delete(s.iidToOID, iidOIDKey(collection, bucket, iid))
	delete(s.iidToTerms, iidTermsKey(collection, bucket, iid))

	return terms
}

// --- Counting ---

// CountObjects returns the number of unique objects in a bucket.
func (s *Store) CountObjects(collection, bucket string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := fmt.Sprintf("o:%s:%s:", collection, bucket)
	count := 0
	for key := range s.oidToIID {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			count++
		}
	}
	return count
}

// CountTerms returns the number of unique terms in a bucket.
func (s *Store) CountTerms(collection, bucket string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := fmt.Sprintf("t:%s:%s:", collection, bucket)
	count := 0
	for key := range s.termToIIDs {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			count++
		}
	}
	return count
}

// --- Flush ---

// FlushBucket removes all data for a bucket.
func (s *Store) FlushBucket(collection, bucket string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	tPrefix := fmt.Sprintf("t:%s:%s:", collection, bucket)
	for key := range s.termToIIDs {
		if len(key) >= len(tPrefix) && key[:len(tPrefix)] == tPrefix {
			delete(s.termToIIDs, key)
			removed++
		}
	}

	oPrefix := fmt.Sprintf("o:%s:%s:", collection, bucket)
	for key := range s.oidToIID {
		if len(key) >= len(oPrefix) && key[:len(oPrefix)] == oPrefix {
			delete(s.oidToIID, key)
			removed++
		}
	}

	iPrefix := fmt.Sprintf("i:%s:%s:", collection, bucket)
	for key := range s.iidToOID {
		if len(key) >= len(iPrefix) && key[:len(iPrefix)] == iPrefix {
			delete(s.iidToOID, key)
			removed++
		}
	}

	itPrefix := fmt.Sprintf("it:%s:%s:", collection, bucket)
	for key := range s.iidToTerms {
		if len(key) >= len(itPrefix) && key[:len(itPrefix)] == itPrefix {
			delete(s.iidToTerms, key)
			removed++
		}
	}

	hwPrefix := fmt.Sprintf("hw:%s:%s:", collection, bucket)
	for key := range s.hashToWord {
		if len(key) >= len(hwPrefix) && key[:len(hwPrefix)] == hwPrefix {
			delete(s.hashToWord, key)
		}
	}

	delete(s.iidCounters, counterKey(collection, bucket))

	return removed
}

// FlushCollection removes all data for a collection.
func (s *Store) FlushCollection(collection string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for key := range s.termToIIDs {
		if hasCollectionPrefix(key, collection) {
			delete(s.termToIIDs, key)
			removed++
		}
	}
	for key := range s.oidToIID {
		if hasCollectionPrefix(key, collection) {
			delete(s.oidToIID, key)
			removed++
		}
	}
	for key := range s.iidToOID {
		if hasCollectionPrefix(key, collection) {
			delete(s.iidToOID, key)
			removed++
		}
	}
	itPrefix := fmt.Sprintf("it:%s:", collection)
	for key := range s.iidToTerms {
		if len(key) >= len(itPrefix) && key[:len(itPrefix)] == itPrefix {
			delete(s.iidToTerms, key)
			removed++
		}
	}
	hwPrefix := fmt.Sprintf("hw:%s:", collection)
	for key := range s.hashToWord {
		if len(key) >= len(hwPrefix) && key[:len(hwPrefix)] == hwPrefix {
			delete(s.hashToWord, key)
		}
	}

	// Remove counters for this collection
	cPrefix := fmt.Sprintf("c:%s:", collection)
	for key := range s.iidCounters {
		if len(key) >= len(cPrefix) && key[:len(cPrefix)] == cPrefix {
			delete(s.iidCounters, key)
		}
	}

	return removed
}

func hasCollectionPrefix(key, collection string) bool {
	// Keys look like "X:collection:bucket:..."
	// Check if the collection portion matches
	if len(key) < 3 {
		return false
	}
	rest := key[2:] // skip "X:"
	return len(rest) >= len(collection)+1 && rest[:len(collection)] == collection && rest[len(collection)] == ':'
}

// --- Persistence ---

type snapshot struct {
	TermToIIDs  map[string][]uint32 `json:"term_to_iids"`
	OIDToIID    map[string]uint32   `json:"oid_to_iid"`
	IIDToOID    map[string]string   `json:"iid_to_oid"`
	IIDToTerms  map[string][]uint32 `json:"iid_to_terms"`
	IIDCounters map[string]uint32   `json:"iid_counters"`
	HashToWord  map[string]string   `json:"hash_to_word,omitempty"`
}

// SaveToDisk persists the store state to disk as JSON.
func (s *Store) SaveToDisk() error {
	s.mu.RLock()
	snap := snapshot{
		TermToIIDs:  deepCopyMapSlice(s.termToIIDs),
		OIDToIID:    deepCopyMapUint32(s.oidToIID),
		IIDToOID:    deepCopyMapString(s.iidToOID),
		IIDToTerms:  deepCopyMapSlice(s.iidToTerms),
		IIDCounters: deepCopyMapUint32(s.iidCounters),
		HashToWord:  deepCopyMapString(s.hashToWord),
	}
	s.mu.RUnlock()

	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	data, err := json.Marshal(&snap)
	if err != nil {
		return fmt.Errorf("marshaling store data: %w", err)
	}

	path := filepath.Join(s.dataDir, "store.json")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("writing store temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming store file: %w", err)
	}

	return nil
}

// LoadFromDisk restores the store state from disk.
func (s *Store) LoadFromDisk() error {
	path := filepath.Join(s.dataDir, "store.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Fresh start
		}
		return fmt.Errorf("reading store file: %w", err)
	}

	var snap snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("unmarshaling store data: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if snap.TermToIIDs != nil {
		s.termToIIDs = snap.TermToIIDs
	}
	if snap.OIDToIID != nil {
		s.oidToIID = snap.OIDToIID
	}
	if snap.IIDToOID != nil {
		s.iidToOID = snap.IIDToOID
	}
	if snap.IIDToTerms != nil {
		s.iidToTerms = snap.IIDToTerms
	}
	if snap.IIDCounters != nil {
		s.iidCounters = snap.IIDCounters
	}
	if snap.HashToWord != nil {
		s.hashToWord = snap.HashToWord
	}

	return nil
}

// ListCollections returns all unique collection names.
func (s *Store) ListCollections() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]struct{})
	for key := range s.iidCounters {
		// key format: "c:collection:bucket"
		if len(key) > 2 && key[:2] == "c:" {
			rest := key[2:]
			if idx := indexOf(rest, ':'); idx > 0 {
				seen[rest[:idx]] = struct{}{}
			}
		}
	}

	result := make([]string, 0, len(seen))
	for c := range seen {
		result = append(result, c)
	}
	return result
}

// ListBuckets returns all bucket names for a collection.
func (s *Store) ListBuckets(collection string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := fmt.Sprintf("c:%s:", collection)
	seen := make(map[string]struct{})
	for key := range s.iidCounters {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			seen[key[len(prefix):]] = struct{}{}
		}
	}

	result := make([]string, 0, len(seen))
	for b := range seen {
		result = append(result, b)
	}
	return result
}

func indexOf(s string, ch byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ch {
			return i
		}
	}
	return -1
}

// --- Deep copy helpers for snapshot ---

func deepCopyMapSlice(m map[string][]uint32) map[string][]uint32 {
	out := make(map[string][]uint32, len(m))
	for k, v := range m {
		cp := make([]uint32, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

func deepCopyMapUint32(m map[string]uint32) map[string]uint32 {
	out := make(map[string]uint32, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func deepCopyMapString(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
