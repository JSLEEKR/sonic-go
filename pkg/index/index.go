// Package index provides the inverted index engine for sonic-go.
// It coordinates between the lexer, store, and suggest trie.
package index

import (
	"github.com/JSLEEKR/sonic-go/pkg/lexer"
	"github.com/JSLEEKR/sonic-go/pkg/store"
	"github.com/JSLEEKR/sonic-go/pkg/suggest"
)

// Index is the main indexing engine that coordinates push/pop operations.
type Index struct {
	store           *store.Store
	trie            *suggest.Trie
	retainWordObjs  int
}

// New creates a new Index engine.
func New(s *store.Store, t *suggest.Trie, retainWordObjs int) *Index {
	if retainWordObjs <= 0 {
		retainWordObjs = 1000
	}
	return &Index{
		store:          s,
		trie:           t,
		retainWordObjs: retainWordObjs,
	}
}

// Push indexes text for an object in a collection/bucket.
// It tokenizes the text, resolves the OID to an IID, and updates
// both the inverted index and the suggest trie.
func (idx *Index) Push(collection, bucket, oid, text string) int {
	tokens := lexer.TokenizeSimple(text)
	if len(tokens) == 0 {
		return 0
	}

	iid, _ := idx.store.ResolveOID(collection, bucket, oid)
	pushed := 0

	for _, tok := range tokens {
		idx.store.AddTermIID(collection, bucket, tok.Hash, iid, idx.retainWordObjs)
		idx.store.AddIIDTerm(collection, bucket, iid, tok.Hash)
		idx.store.SetHashWord(collection, bucket, tok.Hash, tok.Word)
		idx.trie.Insert(collection, bucket, tok.Word)
		pushed++
	}

	return pushed
}

// Pop removes specific text terms from an object's index.
// If text is empty, removes all terms for the object.
func (idx *Index) Pop(collection, bucket, oid, text string) int {
	iid, ok := idx.store.GetIIDForOID(collection, bucket, oid)
	if !ok {
		return 0
	}

	if text == "" {
		// Remove all terms for this object
		terms := idx.store.RemoveObject(collection, bucket, oid)

		// Clean up trie entries for terms that are now empty
		for _, th := range terms {
			iids := idx.store.GetTermIIDs(collection, bucket, th)
			if len(iids) == 0 {
				if word, ok := idx.store.GetWordForHash(collection, bucket, th); ok {
					idx.trie.Remove(collection, bucket, word)
					idx.store.RemoveHashWord(collection, bucket, th)
				}
			}
		}

		return len(terms)
	}

	// Remove only the specified terms
	tokens := lexer.TokenizeSimple(text)
	removed := 0

	for _, tok := range tokens {
		empty := idx.store.RemoveTermIID(collection, bucket, tok.Hash, iid)
		if empty {
			idx.trie.Remove(collection, bucket, tok.Word)
			idx.store.RemoveHashWord(collection, bucket, tok.Hash)
		}
		removed++
	}

	// Update IID terms list
	existingTerms := idx.store.GetIIDTerms(collection, bucket, iid)
	tokenHashes := make(map[uint32]struct{})
	for _, tok := range tokens {
		tokenHashes[tok.Hash] = struct{}{}
	}

	var remaining []uint32
	for _, th := range existingTerms {
		if _, toRemove := tokenHashes[th]; !toRemove {
			remaining = append(remaining, th)
		}
	}
	idx.store.SetIIDTerms(collection, bucket, iid, remaining)

	return removed
}

// Count returns the number of indexed objects in a bucket.
func (idx *Index) Count(collection, bucket string) int {
	return idx.store.CountObjects(collection, bucket)
}

// FlushBucket clears all data in a bucket.
func (idx *Index) FlushBucket(collection, bucket string) int {
	idx.trie.FlushBucket(collection, bucket)
	return idx.store.FlushBucket(collection, bucket)
}

// FlushCollection clears all data in a collection.
func (idx *Index) FlushCollection(collection string) int {
	idx.trie.FlushCollection(collection)
	return idx.store.FlushCollection(collection)
}

// FlushObject removes a single object's data, including trie cleanup.
func (idx *Index) FlushObject(collection, bucket, oid string) int {
	terms := idx.store.RemoveObject(collection, bucket, oid)

	// Clean up trie entries for terms that are now empty
	for _, th := range terms {
		iids := idx.store.GetTermIIDs(collection, bucket, th)
		if len(iids) == 0 {
			// Term has no more references; look up the original word
			// via our hash-to-word map and remove from trie.
			if word, ok := idx.store.GetWordForHash(collection, bucket, th); ok {
				idx.trie.Remove(collection, bucket, word)
				idx.store.RemoveHashWord(collection, bucket, th)
			}
		}
	}

	return len(terms)
}
