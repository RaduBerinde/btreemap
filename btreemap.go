// Copyright 2014-2022 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// In Go 1.18 and beyond, a BTreeMap generic is created, and BTreeMap is a specific
// instantiation of that generic for the Item interface, with a backwards-
// compatible API.  Before go1.18, generics are not supported,
// and BTreeMap is just an implementation based around the Item interface.

// Package btree implements in-memory B-Trees of arbitrary degree.
//
// btree implements an in-memory B-Tree for use as an ordered data structure.
// It is not meant for persistent storage solutions.
//
// It has a flatter structure than an equivalent red-black or other binary tree,
// which in some cases yields better memory usage and/or performance.
// See some discussion on the matter here:
//
//	http://google-opensource.blogspot.com/2013/01/c-containers-that-save-memory-and-time.html
//
// Note, though, that this project is in no way related to the C++ B-Tree
// implementation written about there.
//
// Within this tree, each node contains a slice of items and a (possibly nil)
// slice of children.  For basic numeric values or raw structs, this can cause
// efficiency differences when compared to equivalent C++ template code that
// stores values in arrays within the node:
//   - Due to the overhead of storing values as interfaces (each
//     value needs to be stored as the value itself, then 2 words for the
//     interface pointing to that value and its type), resulting in higher
//     memory use.
//   - Since interfaces can point to values anywhere in memory, values are
//     most likely not stored in contiguous blocks, resulting in a higher
//     number of cache misses.
//
// These issues don't tend to matter, though, when working with strings or other
// heap-allocated structures, since C++-equivalent structures also must store
// pointers and also distribute their values across the heap.
//
// This implementation is designed to be a drop-in replacement to gollrb.LLRB
// trees, (http://github.com/petar/gollrb), an excellent and probably the most
// widely used ordered tree implementation in the Go ecosystem currently.
// Its functions, therefore, exactly mirror those of
// llrb.LLRB where possible.  Unlike gollrb, though, we currently don't
// support storing multiple equivalent values.
//
// There are two implementations; those suffixed with 'G' are generics, usable
// for any type, and require a passed-in "less" function to define their ordering.
// Those without this prefix are specific to the 'Item' interface, and use
// its 'Less' function for ordering.
package btreemap

import "sort"

// Item represents a single object in the tree.
type Item interface {
	// Less tests whether the current item is less than the given argument.
	//
	// This must provide a strict weak ordering.
	// If !a.Less(b) && !b.Less(a), we treat this to mean a == b (i.e. we can only
	// hold one of either a or b in the tree).
	Less(than Item) bool
}

// ItemIterator allows callers of {A/De}scend* to iterate in-order over portions of
// the tree.  When this function returns false, iteration will stop and the
// associated Ascend* function will immediately return.
type ItemIterator[K, V any] func(key K, value V) bool

// Ordered represents the set of types for which the '<' operator work.
type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64 | ~string
}

// Less[T] returns a default LessFunc that uses the '<' operator for types that support it.
func Less[T Ordered]() LessFunc[T] {
	return func(a, b T) bool { return a < b }
}

// NewOrdered creates a new B-Tree for ordered types.
func NewOrdered[K Ordered, V any](degree int) *BTreeMap[K, V] {
	return New[K, V](degree, Less[K]())
}

// New creates a new B-Tree with the given degree.
//
// New(2), for example, will create a 2-3-4 tree (each node contains 1-3 items
// and 2-4 children).
//
// The passed-in LessFunc determines how objects of type T are ordered.
func New[K any, V any](degree int, less LessFunc[K]) *BTreeMap[K, V] {
	return NewWithFreeList(degree, less, NewFreeList[K, V](DefaultFreeListSize))
}

// NewWithFreeList creates a new B-Tree that uses the given node free list.
func NewWithFreeList[K any, V any](
	degree int, less LessFunc[K], f *FreeList[K, V],
) *BTreeMap[K, V] {
	if degree <= 1 {
		panic("bad degree")
	}
	return &BTreeMap[K, V]{
		degree: degree,
		cow:    &copyOnWriteContext[K, V]{freelist: f, less: less},
	}
}

// findKV returns the index where the given key should be inserted into this
// list.  'found' is true if the kty already exists in the list at the given
// index.
func findKV[K any, V any](s items[kv[K, V]], key K, less func(K, K) bool) (index int, found bool) {
	i := sort.Search(len(s), func(i int) bool {
		return less(key, s[i].k)
	})
	if i > 0 && !less(s[i-1].k, key) {
		return i - 1, true
	}
	return i, false
}

// BTreeMap is a generic implementation of a B-Tree.
//
// BTreeMap stores items of type T in an ordered structure, allowing easy insertion,
// removal, and iteration.
//
// Write operations are not safe for concurrent mutation by multiple
// goroutines, but Read operations are.
type BTreeMap[K any, V any] struct {
	degree int
	length int
	root   *node[K, V]
	cow    *copyOnWriteContext[K, V]
}

// LessFunc[T] determines how to order a type 'T'.  It should implement a strict
// ordering, and should return true if within that ordering, 'a' < 'b'.
type LessFunc[T any] func(a, b T) bool

// copyOnWriteContext pointers determine node ownership... a tree with a write
// context equivalent to a node's write context is allowed to modify that node.
// A tree whose write context does not match a node's is not allowed to modify
// it, and must create a new, writable copy (IE: it's a Clone).
//
// When doing any write operation, we maintain the invariant that the current
// node's context is equal to the context of the tree that requested the write.
// We do this by, before we descend into any node, creating a copy with the
// correct context if the contexts don't match.
//
// Since the node we're currently visiting on any write has the requesting
// tree's context, that node is modifiable in place.  Children of that node may
// not share context, but before we descend into them, we'll make a mutable
// copy.
type copyOnWriteContext[K any, V any] struct {
	freelist *FreeList[K, V]
	less     LessFunc[K]
}

// Clone clones the btree, lazily.  Clone should not be called concurrently,
// but the original tree (t) and the new tree (t2) can be used concurrently
// once the Clone call completes.
//
// The internal tree structure of b is marked read-only and shared between t and
// t2.  Writes to both t and t2 use copy-on-write logic, creating new nodes
// whenever one of b's original nodes would have been modified.  Read operations
// should have no performance degredation.  Write operations for both t and t2
// will initially experience minor slow-downs caused by additional allocs and
// copies due to the aforementioned copy-on-write logic, but should converge to
// the original performance characteristics of the original tree.
func (t *BTreeMap[K, V]) Clone() (t2 *BTreeMap[K, V]) {
	// Create two entirely new copy-on-write contexts.
	// This operation effectively creates three trees:
	//   the original, shared nodes (old b.cow)
	//   the new b.cow nodes
	//   the new out.cow nodes
	cow1, cow2 := *t.cow, *t.cow
	out := *t
	t.cow = &cow1
	out.cow = &cow2
	return &out
}

// maxItems returns the max number of items to allow per node.
func (t *BTreeMap[K, V]) maxItems() int {
	return t.degree*2 - 1
}

// minItems returns the min number of items to allow per node (ignored for the
// root node).
func (t *BTreeMap[K, V]) minItems() int {
	return t.degree - 1
}

func (c *copyOnWriteContext[K, V]) newNode() (n *node[K, V]) {
	n = c.freelist.newNode()
	n.cow = c
	return
}

type freeType int

const (
	ftFreelistFull freeType = iota // node was freed (available for GC, not stored in freelist)
	ftStored                       // node was stored in the freelist for later use
	ftNotOwned                     // node was ignored by COW, since it's owned by another one
)

// freeNode frees a node within a given COW context, if it's owned by that
// context.  It returns what happened to the node (see freeType const
// documentation).
func (c *copyOnWriteContext[K, V]) freeNode(n *node[K, V]) freeType {
	if n.cow == c {
		// clear to allow GC
		n.items.truncate(0)
		n.children.truncate(0)
		n.cow = nil
		if c.freelist.freeNode(n) {
			return ftStored
		} else {
			return ftFreelistFull
		}
	} else {
		return ftNotOwned
	}
}

// ReplaceOrInsert adds the given item to the tree.  If an item in the tree
// already equals the given one, it is removed from the tree and returned,
// and the second return value is true.  Otherwise, (zeroValue, false)
//
// nil cannot be added to the tree (will panic).
func (t *BTreeMap[K, V]) ReplaceOrInsert(key K, value V) (_ K, _ V, replaced bool) {
	if t.root == nil {
		t.root = t.cow.newNode()
		t.root.items = append(t.root.items, kv[K, V]{k: key, v: value})
		t.length++
		return
	} else {
		t.root = t.root.mutableFor(t.cow)
		if len(t.root.items) >= t.maxItems() {
			item2, second := t.root.split(t.maxItems() / 2)
			oldroot := t.root
			t.root = t.cow.newNode()
			t.root.items = append(t.root.items, item2)
			t.root.children = append(t.root.children, oldroot, second)
		}
	}
	out, outb := t.root.insert(kv[K, V]{k: key, v: value}, t.maxItems())
	if !outb {
		t.length++
	}
	return out.k, out.v, outb
}

// Delete removes an item equal to the passed in item from the tree, returning
// it.  If no such item exists, returns (zeroValue, false).
func (t *BTreeMap[K, V]) Delete(key K) (K, V, bool) {
	return t.deleteItem(key, removeItem)
}

// DeleteMin removes the smallest item in the tree and returns it.
// If no such item exists, returns (zeroValue, false).
func (t *BTreeMap[K, V]) DeleteMin() (K, V, bool) {
	var zero K
	return t.deleteItem(zero, removeMin)
}

// DeleteMax removes the largest item in the tree and returns it.
// If no such item exists, returns (zeroValue, false).
func (t *BTreeMap[K, V]) DeleteMax() (K, V, bool) {
	var zero K
	return t.deleteItem(zero, removeMax)
}

func (t *BTreeMap[K, V]) deleteItem(key K, typ toRemove) (_ K, _ V, _ bool) {
	if t.root == nil || len(t.root.items) == 0 {
		return
	}
	t.root = t.root.mutableFor(t.cow)
	out, outb := t.root.remove(key, t.minItems(), typ)
	if len(t.root.items) == 0 && len(t.root.children) > 0 {
		oldroot := t.root
		t.root = t.root.children[0]
		t.cow.freeNode(oldroot)
	}
	if outb {
		t.length--
	}
	return out.k, out.v, outb
}

// AscendRange calls the iterator for every value in the tree within the range
// [greaterOrEqual, lessThan), until iterator returns false.
func (t *BTreeMap[K, V]) AscendRange(greaterOrEqual, lessThan K, iterator ItemIterator[K, V]) {
	if t.root == nil {
		return
	}
	t.root.ascend(optional[K](greaterOrEqual), optional[K](lessThan), true, false, iterator)
}

// AscendLessThan calls the iterator for every value in the tree within the range
// [first, pivot), until iterator returns false.
func (t *BTreeMap[K, V]) AscendLessThan(pivot K, iterator ItemIterator[K, V]) {
	if t.root == nil {
		return
	}
	t.root.ascend(empty[K](), optional(pivot), false, false, iterator)
}

// AscendGreaterOrEqual calls the iterator for every value in the tree within
// the range [pivot, last], until iterator returns false.
func (t *BTreeMap[K, V]) AscendGreaterOrEqual(pivot K, iterator ItemIterator[K, V]) {
	if t.root == nil {
		return
	}
	t.root.ascend(optional[K](pivot), empty[K](), true, false, iterator)
}

// Ascend calls the iterator for every value in the tree within the range
// [first, last], until iterator returns false.
func (t *BTreeMap[K, V]) Ascend(iterator ItemIterator[K, V]) {
	if t.root == nil {
		return
	}
	t.root.ascend(empty[K](), empty[K](), false, false, iterator)
}

// DescendRange calls the iterator for every value in the tree within the range
// [lessOrEqual, greaterThan), until iterator returns false.
func (t *BTreeMap[K, V]) DescendRange(lessOrEqual, greaterThan K, iterator ItemIterator[K, V]) {
	if t.root == nil {
		return
	}
	t.root.descend(optional[K](lessOrEqual), optional[K](greaterThan), true, false, iterator)
}

// DescendLessOrEqual calls the iterator for every value in the tree within the range
// [pivot, first], until iterator returns false.
func (t *BTreeMap[K, V]) DescendLessOrEqual(pivot K, iterator ItemIterator[K, V]) {
	if t.root == nil {
		return
	}
	t.root.descend(optional[K](pivot), empty[K](), true, false, iterator)
}

// DescendGreaterThan calls the iterator for every value in the tree within
// the range [last, pivot), until iterator returns false.
func (t *BTreeMap[K, V]) DescendGreaterThan(pivot K, iterator ItemIterator[K, V]) {
	if t.root == nil {
		return
	}
	t.root.descend(empty[K](), optional[K](pivot), false, false, iterator)
}

// Descend calls the iterator for every value in the tree within the range
// [last, first], until iterator returns false.
func (t *BTreeMap[K, V]) Descend(iterator ItemIterator[K, V]) {
	if t.root == nil {
		return
	}
	t.root.descend(empty[K](), empty[K](), false, false, iterator)
}

// Get looks for the key item in the tree, returning it.  It returns
// (zeroValue, false) if unable to find that item.
func (t *BTreeMap[K, V]) Get(key K) (_ K, _ V, _ bool) {
	if t.root == nil {
		return
	}
	return t.root.get(key)
}

// Min returns the smallest item in the tree, or (zeroValue, false) if the tree is empty.
func (t *BTreeMap[K, V]) Min() (K, V, bool) {
	return min(t.root)
}

// Max returns the largest item in the tree, or (zeroValue, false) if the tree is empty.
func (t *BTreeMap[K, V]) Max() (K, V, bool) {
	return max(t.root)
}

// Has returns true if the given key is in the tree.
func (t *BTreeMap[K, V]) Has(key K) bool {
	_, _, ok := t.Get(key)
	return ok
}

// Len returns the number of items currently in the tree.
func (t *BTreeMap[K, V]) Len() int {
	return t.length
}

// Clear removes all items from the btree.  If addNodesToFreelist is true,
// t's nodes are added to its freelist as part of this call, until the freelist
// is full.  Otherwise, the root node is simply dereferenced and the subtree
// left to Go's normal GC processes.
//
// This can be much faster
// than calling Delete on all elements, because that requires finding/removing
// each element in the tree and updating the tree accordingly.  It also is
// somewhat faster than creating a new tree to replace the old one, because
// nodes from the old tree are reclaimed into the freelist for use by the new
// one, instead of being lost to the garbage collector.
//
// This call takes:
//
//	O(1): when addNodesToFreelist is false, this is a single operation.
//	O(1): when the freelist is already full, it breaks out immediately
//	O(freelist size):  when the freelist is empty and the nodes are all owned
//	    by this tree, nodes are added to the freelist until full.
//	O(tree size):  when all nodes are owned by another tree, all nodes are
//	    iterated over looking for nodes to add to the freelist, and due to
//	    ownership, none are.
func (t *BTreeMap[K, V]) Clear(addNodesToFreelist bool) {
	if t.root != nil && addNodesToFreelist {
		t.root.reset(t.cow)
	}
	t.root, t.length = nil, 0
}

// reset returns a subtree to the freelist.  It breaks out immediately if the
// freelist is full, since the only benefit of iterating is to fill that
// freelist up.  Returns true if parent reset call should continue.
func (n *node[K, V]) reset(c *copyOnWriteContext[K, V]) bool {
	for _, child := range n.children {
		if !child.reset(c) {
			return false
		}
	}
	return c.freeNode(n) != ftFreelistFull
}
