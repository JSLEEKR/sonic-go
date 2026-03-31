// Package suggest provides prefix-based auto-complete using an in-memory trie.
package suggest

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// trieNode is a single node in the trie.
type trieNode struct {
	children map[rune]*trieNode
	isEnd    bool
	count    int // number of times this word was inserted (for ranking)
}

func newTrieNode() *trieNode {
	return &trieNode{
		children: make(map[rune]*trieNode),
	}
}

// Trie is a prefix tree for auto-complete suggestions.
// It is partitioned by collection:bucket for isolation.
// Safe for concurrent use.
type Trie struct {
	mu    sync.RWMutex
	roots map[string]*trieNode // key: "collection:bucket"
}

// NewTrie creates a new empty Trie.
func NewTrie() *Trie {
	return &Trie{
		roots: make(map[string]*trieNode),
	}
}

func bucketKey(collection, bucket string) string {
	return fmt.Sprintf("%s:%s", collection, bucket)
}

// Insert adds a word to the trie for a given collection/bucket.
func (t *Trie) Insert(collection, bucket, word string) {
	word = strings.ToLower(word)
	if word == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	key := bucketKey(collection, bucket)
	root, ok := t.roots[key]
	if !ok {
		root = newTrieNode()
		t.roots[key] = root
	}

	node := root
	for _, ch := range word {
		child, ok := node.children[ch]
		if !ok {
			child = newTrieNode()
			node.children[ch] = child
		}
		node = child
	}
	node.isEnd = true
	node.count++
}

// Remove removes a word from the trie. Returns true if the word existed.
func (t *Trie) Remove(collection, bucket, word string) bool {
	word = strings.ToLower(word)
	if word == "" {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	key := bucketKey(collection, bucket)
	root, ok := t.roots[key]
	if !ok {
		return false
	}

	return t.removeHelper(root, []rune(word), 0)
}

func (t *Trie) removeHelper(node *trieNode, runes []rune, depth int) bool {
	if depth == len(runes) {
		if !node.isEnd {
			return false
		}
		node.count--
		if node.count <= 0 {
			node.isEnd = false
		}
		return len(node.children) == 0 && !node.isEnd
	}

	ch := runes[depth]
	child, ok := node.children[ch]
	if !ok {
		return false
	}

	shouldDelete := t.removeHelper(child, runes, depth+1)
	if shouldDelete {
		delete(node.children, ch)
		return len(node.children) == 0 && !node.isEnd
	}

	return false
}

// ForceRemove completely removes a word from the trie regardless of its
// insertion count. This is used when the inverted index confirms no documents
// reference the term anymore.
func (t *Trie) ForceRemove(collection, bucket, word string) bool {
	word = strings.ToLower(word)
	if word == "" {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	key := bucketKey(collection, bucket)
	root, ok := t.roots[key]
	if !ok {
		return false
	}

	return t.forceRemoveHelper(root, []rune(word), 0)
}

func (t *Trie) forceRemoveHelper(node *trieNode, runes []rune, depth int) bool {
	if depth == len(runes) {
		if !node.isEnd {
			return false
		}
		node.isEnd = false
		node.count = 0
		return len(node.children) == 0
	}

	ch := runes[depth]
	child, ok := node.children[ch]
	if !ok {
		return false
	}

	shouldDelete := t.forceRemoveHelper(child, runes, depth+1)
	if shouldDelete {
		delete(node.children, ch)
		return len(node.children) == 0 && !node.isEnd
	}

	return false
}

// Suggest returns up to limit words that start with the given prefix.
func (t *Trie) Suggest(collection, bucket, prefix string, limit int) []string {
	prefix = strings.ToLower(prefix)
	if limit <= 0 {
		limit = 10
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	key := bucketKey(collection, bucket)
	root, ok := t.roots[key]
	if !ok {
		return nil
	}

	// Navigate to the prefix node
	node := root
	for _, ch := range prefix {
		child, ok := node.children[ch]
		if !ok {
			return nil
		}
		node = child
	}

	// Collect all words from this point
	var results []wordEntry
	t.collectWords(node, prefix, &results, limit*3) // Over-collect for sorting

	// Sort by count descending, then alphabetically
	sort.Slice(results, func(i, j int) bool {
		if results[i].count != results[j].count {
			return results[i].count > results[j].count
		}
		return results[i].word < results[j].word
	})

	if len(results) > limit {
		results = results[:limit]
	}

	words := make([]string, len(results))
	for i, r := range results {
		words[i] = r.word
	}
	return words
}

type wordEntry struct {
	word  string
	count int
}

type fuzzyResult struct {
	word     string
	distance int
	count    int
}

func (t *Trie) collectWords(node *trieNode, prefix string, results *[]wordEntry, maxCollect int) {
	if len(*results) >= maxCollect {
		return
	}

	if node.isEnd {
		*results = append(*results, wordEntry{word: prefix, count: node.count})
	}

	// Sort children for deterministic output
	runes := make([]rune, 0, len(node.children))
	for ch := range node.children {
		runes = append(runes, ch)
	}
	sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })

	for _, ch := range runes {
		child := node.children[ch]
		t.collectWords(child, prefix+string(ch), results, maxCollect)
	}
}

// SuggestFuzzy returns suggestions using Levenshtein distance for typo correction.
// It finds words within maxDistance edits of the query.
func (t *Trie) SuggestFuzzy(collection, bucket, query string, maxDistance, limit int) []string {
	query = strings.ToLower(query)
	if limit <= 0 {
		limit = 10
	}
	if maxDistance <= 0 {
		maxDistance = autoDistance(query)
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	key := bucketKey(collection, bucket)
	root, ok := t.roots[key]
	if !ok {
		return nil
	}

	var results []fuzzyResult
	t.fuzzySearch(root, "", query, maxDistance, &results)

	// Sort: distance ascending, then count descending
	sort.Slice(results, func(i, j int) bool {
		if results[i].distance != results[j].distance {
			return results[i].distance < results[j].distance
		}
		return results[i].count > results[j].count
	})

	if len(results) > limit {
		results = results[:limit]
	}

	words := make([]string, len(results))
	for i, r := range results {
		words[i] = r.word
	}
	return words
}

func (t *Trie) fuzzySearch(node *trieNode, current, target string, maxDist int, results *[]fuzzyResult) {
	if len(*results) > 100 { // safety limit
		return
	}

	if node.isEnd {
		dist := levenshtein(current, target)
		if dist <= maxDist {
			*results = append(*results, fuzzyResult{
				word:     current,
				distance: dist,
				count:    node.count,
			})
		}
	}

	// Pruning: if the current prefix is already too far from target, skip
	if len(current) > len(target)+maxDist {
		return
	}

	for ch, child := range node.children {
		t.fuzzySearch(child, current+string(ch), target, maxDist, results)
	}
}

// autoDistance returns the appropriate edit distance based on word length.
func autoDistance(word string) int {
	l := len([]rune(word))
	switch {
	case l <= 3:
		return 0
	case l <= 6:
		return 1
	case l <= 9:
		return 2
	default:
		return 3
	}
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	la := len(ra)
	lb := len(rb)

	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Create DP matrix (using two rows for space efficiency)
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min(
				curr[j-1]+1,    // insertion
				prev[j]+1,      // deletion
				prev[j-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}

	return prev[lb]
}

func min(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

// FlushBucket removes all data for a bucket.
func (t *Trie) FlushBucket(collection, bucket string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.roots, bucketKey(collection, bucket))
}

// FlushCollection removes all data for a collection.
func (t *Trie) FlushCollection(collection string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	prefix := collection + ":"
	for key := range t.roots {
		if strings.HasPrefix(key, prefix) {
			delete(t.roots, key)
		}
	}
}

// WordCount returns the total number of unique words in a bucket's trie.
func (t *Trie) WordCount(collection, bucket string) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := bucketKey(collection, bucket)
	root, ok := t.roots[key]
	if !ok {
		return 0
	}

	return t.countWords(root)
}

func (t *Trie) countWords(node *trieNode) int {
	count := 0
	if node.isEnd {
		count++
	}
	for _, child := range node.children {
		count += t.countWords(child)
	}
	return count
}

// AllWords returns all words in a bucket's trie (for debugging/testing).
func (t *Trie) AllWords(collection, bucket string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := bucketKey(collection, bucket)
	root, ok := t.roots[key]
	if !ok {
		return nil
	}

	var words []string
	t.collectAllWords(root, "", &words)
	sort.Strings(words)
	return words
}

func (t *Trie) collectAllWords(node *trieNode, prefix string, words *[]string) {
	if node.isEnd {
		*words = append(*words, prefix)
	}
	for ch, child := range node.children {
		t.collectAllWords(child, prefix+string(ch), words)
	}
}
