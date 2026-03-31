// Package search provides the query engine for sonic-go.
// It supports term-based search with AND intersection, typo correction,
// and pagination.
package search

import (
	"sort"

	"github.com/JSLEEKR/sonic-go/pkg/lexer"
	"github.com/JSLEEKR/sonic-go/pkg/store"
	"github.com/JSLEEKR/sonic-go/pkg/suggest"
)

// Engine is the search query engine.
type Engine struct {
	store *store.Store
	trie  *suggest.Trie
}

// New creates a new search Engine.
func New(s *store.Store, t *suggest.Trie) *Engine {
	return &Engine{
		store: s,
		trie:  t,
	}
}

// QueryOptions controls search behavior.
type QueryOptions struct {
	Limit          int
	Offset         int
	AlternatesTry  int // number of typo-corrected alternatives to try per term
}

// DefaultQueryOptions returns sensible defaults.
func DefaultQueryOptions() QueryOptions {
	return QueryOptions{
		Limit:         10,
		Offset:        0,
		AlternatesTry: 0,
	}
}

// Query searches for objects matching the query text.
// It tokenizes the query, looks up each term in the index,
// intersects the results (AND mode), and returns matching OIDs.
func (e *Engine) Query(collection, bucket, queryText string, opts QueryOptions) []string {
	if opts.Limit <= 0 {
		opts.Limit = 10
	}

	tokens := lexer.TokenizeSimple(queryText)
	if len(tokens) == 0 {
		return nil
	}

	// For each token, get the set of IIDs
	var iidSets []map[uint32]struct{}

	for _, tok := range tokens {
		iids := e.store.GetTermIIDs(collection, bucket, tok.Hash)

		iidSet := make(map[uint32]struct{})
		for _, iid := range iids {
			iidSet[iid] = struct{}{}
		}

		// If no results and alternates enabled, try typo correction
		if len(iidSet) == 0 && opts.AlternatesTry > 0 {
			alternates := e.trie.SuggestFuzzy(collection, bucket, tok.Word, 0, opts.AlternatesTry+1)
			for _, alt := range alternates {
				if alt == tok.Word {
					continue
				}
				altHash := lexer.HashTerm(alt)
				altIIDs := e.store.GetTermIIDs(collection, bucket, altHash)
				for _, iid := range altIIDs {
					iidSet[iid] = struct{}{}
				}
			}
		}

		iidSets = append(iidSets, iidSet)
	}

	if len(iidSets) == 0 {
		return nil
	}

	// Intersect all sets (AND mode)
	result := intersect(iidSets)

	// Convert IIDs to OIDs with deterministic ordering (sorted by IID ascending)
	sortedIIDs := make([]uint32, 0, len(result))
	for iid := range result {
		sortedIIDs = append(sortedIIDs, iid)
	}
	sort.Slice(sortedIIDs, func(i, j int) bool { return sortedIIDs[i] < sortedIIDs[j] })

	var oids []string
	for _, iid := range sortedIIDs {
		if oid, ok := e.store.GetOIDForIID(collection, bucket, iid); ok {
			oids = append(oids, oid)
		}
	}

	// Apply offset and limit
	if opts.Offset >= len(oids) {
		return nil
	}
	oids = oids[opts.Offset:]
	if len(oids) > opts.Limit {
		oids = oids[:opts.Limit]
	}

	return oids
}

// QuerySingle searches for a single term without tokenization overhead.
func (e *Engine) QuerySingle(collection, bucket, term string, limit int) []string {
	if limit <= 0 {
		limit = 10
	}

	hash := lexer.HashTerm(term)
	iids := e.store.GetTermIIDs(collection, bucket, hash)

	var oids []string
	for _, iid := range iids {
		if oid, ok := e.store.GetOIDForIID(collection, bucket, iid); ok {
			oids = append(oids, oid)
			if len(oids) >= limit {
				break
			}
		}
	}

	return oids
}

// intersect returns the intersection of multiple IID sets.
func intersect(sets []map[uint32]struct{}) map[uint32]struct{} {
	if len(sets) == 0 {
		return nil
	}
	if len(sets) == 1 {
		return sets[0]
	}

	// Start with the smallest set for efficiency
	smallest := 0
	for i, s := range sets {
		if len(s) < len(sets[smallest]) {
			smallest = i
		}
	}

	result := make(map[uint32]struct{})
	for iid := range sets[smallest] {
		inAll := true
		for i, s := range sets {
			if i == smallest {
				continue
			}
			if _, ok := s[iid]; !ok {
				inAll = false
				break
			}
		}
		if inAll {
			result[iid] = struct{}{}
		}
	}

	return result
}

// Suggest returns auto-complete suggestions for a prefix.
func (e *Engine) Suggest(collection, bucket, prefix string, limit int) []string {
	return e.trie.Suggest(collection, bucket, prefix, limit)
}
