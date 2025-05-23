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

package btreemap

// This file contains the original github.com/google/btree/btree_generic_test.go
// tests, adapted to use BTreeMap.

import (
	"cmp"
	"flag"
	"math/rand"
	"reflect"
	"sort"
	"sync"
	"testing"
)

type bTree[T any] struct {
	*BTreeMap[T, struct{}]
}

func (b *bTree[T]) Clone() *bTree[T] {
	return &bTree[T]{
		BTreeMap: b.BTreeMap.Clone(),
	}
}

// ItemIterator allows callers of {A/De}scend* to iterate in-order over portions of
// the tree.  When this function returns false, iteration will stop and the
// associated Ascend* function will immediately return.
type ItemIterator[K, V any] func(key K, value V) bool

func (t *bTree[T]) AscendRange(greaterOrEqual, lessThan T, iterator ItemIterator[T, struct{}]) {
	t.BTreeMap.AscendFunc(GE(greaterOrEqual), LT(lessThan), iterator)
}

// AscendLessThan calls the iterator for every value in the tree within the range
// [first, pivot), until iterator returns false.
func (t *bTree[T]) AscendLessThan(pivot T, iterator ItemIterator[T, struct{}]) {
	t.BTreeMap.AscendFunc(Min[T](), LT(pivot), iterator)
}

// AscendGreaterOrEqual calls the iterator for every value in the tree within
// the range [pivot, last], until iterator returns false.
func (t *bTree[T]) AscendGreaterOrEqual(pivot T, iterator ItemIterator[T, struct{}]) {
	t.BTreeMap.AscendFunc(GE(pivot), Max[T](), iterator)
}

// Ascend calls the iterator for every value in the tree within the range
// [first, last], until iterator returns false.
func (t *bTree[T]) Ascend(iterator ItemIterator[T, struct{}]) {
	t.BTreeMap.AscendFunc(Min[T](), Max[T](), iterator)
}

// DescendRange calls the iterator for every value in the tree within the range
// [lessOrEqual, greaterThan), until iterator returns false.
func (t *bTree[T]) DescendRange(lessOrEqual, greaterThan T, iterator ItemIterator[T, struct{}]) {
	t.BTreeMap.DescendFunc(LE(lessOrEqual), GT(greaterThan), iterator)
}

// DescendLessOrEqual calls the iterator for every value in the tree within the range
// [pivot, first], until iterator returns false.
func (t *bTree[T]) DescendLessOrEqual(pivot T, iterator ItemIterator[T, struct{}]) {
	t.BTreeMap.DescendFunc(LE(pivot), Min[T](), iterator)
}

// DescendGreaterThan calls the iterator for every value in the tree within
// the range [last, pivot), until iterator returns false.
func (t *bTree[T]) DescendGreaterThan(pivot T, iterator ItemIterator[T, struct{}]) {
	t.BTreeMap.DescendFunc(Max[T](), GT(pivot), iterator)
}

// Descend calls the iterator for every value in the tree within the range
// [last, first], until iterator returns false.
func (t *bTree[T]) Descend(iterator ItemIterator[T, struct{}]) {
	t.BTreeMap.DescendFunc(Max[T](), Min[T](), iterator)
}

func newInt(degree int) *bTree[int] {
	return &bTree[int]{
		BTreeMap: New[int, struct{}](degree, cmp.Compare[int]),
	}
}

func intRange(s int, reverse bool) []int {
	out := make([]int, s)
	for i := 0; i < s; i++ {
		v := i
		if reverse {
			v = s - i - 1
		}
		out[i] = v
	}
	return out
}

func intAll(t *bTree[int]) (out []int) {
	t.Ascend(func(a int, _ struct{}) bool {
		out = append(out, a)
		return true
	})
	return
}

func intAllRev(t *bTree[int]) (out []int) {
	t.Descend(func(a int, _ struct{}) bool {
		out = append(out, a)
		return true
	})
	return
}

var btreeDegree = flag.Int("degree", 32, "B-Tree degree")

func TestBTreeG(t *testing.T) {
	tr := newInt(*btreeDegree)
	const treeSize = 10000
	for i := 0; i < 10; i++ {
		if min, _, ok := tr.Min(); ok || min != 0 {
			t.Fatalf("empty min, got %+v", min)
		}
		if max, _, ok := tr.Max(); ok || max != 0 {
			t.Fatalf("empty max, got %+v", max)
		}
		for _, item := range rand.Perm(treeSize) {
			if x, _, ok := tr.ReplaceOrInsert(item, struct{}{}); ok || x != 0 {
				t.Fatal("insert found item", item)
			}
		}
		for _, item := range rand.Perm(treeSize) {
			if x, _, ok := tr.ReplaceOrInsert(item, struct{}{}); !ok || x != item {
				t.Fatal("insert didn't find item", item)
			}
		}
		want := 0
		if min, _, ok := tr.Min(); !ok || min != want {
			t.Fatalf("min: ok %v want %+v, got %+v", ok, want, min)
		}
		want = treeSize - 1
		if max, _, ok := tr.Max(); !ok || max != want {
			t.Fatalf("max: ok %v want %+v, got %+v", ok, want, max)
		}
		got := intAll(tr)
		wantRange := intRange(treeSize, false)
		if !reflect.DeepEqual(got, wantRange) {
			t.Fatalf("mismatch:\n got: %v\nwant: %v", got, wantRange)
		}

		gotrev := intAllRev(tr)
		wantrev := intRange(treeSize, true)
		if !reflect.DeepEqual(gotrev, wantrev) {
			t.Fatalf("mismatch:\n got: %v\nwant: %v", gotrev, wantrev)
		}

		for _, item := range rand.Perm(treeSize) {
			if x, _, ok := tr.Delete(item); !ok || x != item {
				t.Fatalf("didn't find %v", item)
			}
		}
		if got = intAll(tr); len(got) > 0 {
			t.Fatalf("some left!: %v", got)
		}
		if got = intAllRev(tr); len(got) > 0 {
			t.Fatalf("some left!: %v", got)
		}
	}
}

func TestDeleteMinG(t *testing.T) {
	tr := newInt(3)
	for _, v := range rand.Perm(100) {
		tr.ReplaceOrInsert(v, struct{}{})
	}
	var got []int
	for v, _, ok := tr.DeleteMin(); ok; v, _, ok = tr.DeleteMin() {
		got = append(got, v)
	}
	if want := intRange(100, false); !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
}

func TestDeleteMaxG(t *testing.T) {
	tr := newInt(3)
	for _, v := range rand.Perm(100) {
		tr.ReplaceOrInsert(v, struct{}{})
	}
	var got []int
	for v, _, ok := tr.DeleteMax(); ok; v, _, ok = tr.DeleteMax() {
		got = append(got, v)
	}
	if want := intRange(100, true); !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
}

func TestAscendRangeG(t *testing.T) {
	tr := newInt(2)
	for _, v := range rand.Perm(100) {
		tr.ReplaceOrInsert(v, struct{}{})
	}
	var got []int
	tr.AscendRange(40, 60, func(a int, _ struct{}) bool {
		got = append(got, a)
		return true
	})
	if want := intRange(100, false)[40:60]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	tr.AscendRange(40, 60, func(a int, _ struct{}) bool {
		if a > 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := intRange(100, false)[40:51]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
}

func TestDescendRangeG(t *testing.T) {
	tr := newInt(2)
	for _, v := range rand.Perm(100) {
		tr.ReplaceOrInsert(v, struct{}{})
	}
	var got []int
	tr.DescendRange(60, 40, func(a int, _ struct{}) bool {
		got = append(got, a)
		return true
	})
	if want := intRange(100, true)[39:59]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendrange:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	tr.DescendRange(60, 40, func(a int, _ struct{}) bool {
		if a < 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := intRange(100, true)[39:50]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendrange:\n got: %v\nwant: %v", got, want)
	}
}

func TestAscendLessThanG(t *testing.T) {
	tr := newInt(*btreeDegree)
	for _, v := range rand.Perm(100) {
		tr.ReplaceOrInsert(v, struct{}{})
	}
	var got []int
	tr.AscendLessThan(60, func(a int, _ struct{}) bool {
		got = append(got, a)
		return true
	})
	if want := intRange(100, false)[:60]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	tr.AscendLessThan(60, func(a int, _ struct{}) bool {
		if a > 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := intRange(100, false)[:51]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
}

func TestDescendLessOrEqualG(t *testing.T) {
	tr := newInt(*btreeDegree)
	for _, v := range rand.Perm(100) {
		tr.ReplaceOrInsert(v, struct{}{})
	}
	var got []int
	tr.DescendLessOrEqual(40, func(a int, _ struct{}) bool {
		got = append(got, a)
		return true
	})
	if want := intRange(100, true)[59:]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendlessorequal:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	tr.DescendLessOrEqual(60, func(a int, _ struct{}) bool {
		if a < 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := intRange(100, true)[39:50]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendlessorequal:\n got: %v\nwant: %v", got, want)
	}
}

func TestAscendGreaterOrEqualG(t *testing.T) {
	tr := newInt(*btreeDegree)
	for _, v := range rand.Perm(100) {
		tr.ReplaceOrInsert(v, struct{}{})
	}
	var got []int
	tr.AscendGreaterOrEqual(40, func(a int, _ struct{}) bool {
		got = append(got, a)
		return true
	})
	if want := intRange(100, false)[40:]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	tr.AscendGreaterOrEqual(40, func(a int, _ struct{}) bool {
		if a > 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := intRange(100, false)[40:51]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
}

func TestDescendGreaterThanG(t *testing.T) {
	tr := newInt(*btreeDegree)
	for _, v := range rand.Perm(100) {
		tr.ReplaceOrInsert(v, struct{}{})
	}
	var got []int
	tr.DescendGreaterThan(40, func(a int, _ struct{}) bool {
		got = append(got, a)
		return true
	})
	if want := intRange(100, true)[:59]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendgreaterthan:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	tr.DescendGreaterThan(40, func(a int, _ struct{}) bool {
		if a < 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := intRange(100, true)[:50]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendgreaterthan:\n got: %v\nwant: %v", got, want)
	}
}

const benchmarkTreeSize = 10000

func BenchmarkInsertG(b *testing.B) {
	b.StopTimer()
	insertP := rand.Perm(benchmarkTreeSize)
	b.StartTimer()
	i := 0
	for i < b.N {
		tr := newInt(*btreeDegree)
		for _, item := range insertP {
			tr.ReplaceOrInsert(item, struct{}{})
			i++
			if i >= b.N {
				return
			}
		}
	}
}

func BenchmarkSeekG(b *testing.B) {
	b.StopTimer()
	size := 100000
	insertP := rand.Perm(size)
	tr := newInt(*btreeDegree)
	for _, item := range insertP {
		tr.ReplaceOrInsert(item, struct{}{})
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		tr.AscendGreaterOrEqual(i%size, func(i int, _ struct{}) bool { return false })
	}
}

func BenchmarkDeleteInsertG(b *testing.B) {
	b.StopTimer()
	insertP := rand.Perm(benchmarkTreeSize)
	tr := newInt(*btreeDegree)
	for _, item := range insertP {
		tr.ReplaceOrInsert(item, struct{}{})
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tr.Delete(insertP[i%benchmarkTreeSize])
		tr.ReplaceOrInsert(insertP[i%benchmarkTreeSize], struct{}{})
	}
}

func BenchmarkDeleteInsertCloneOnceG(b *testing.B) {
	b.StopTimer()
	insertP := rand.Perm(benchmarkTreeSize)
	tr := newInt(*btreeDegree)
	for _, item := range insertP {
		tr.ReplaceOrInsert(item, struct{}{})
	}
	tr = tr.Clone()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tr.Delete(insertP[i%benchmarkTreeSize])
		tr.ReplaceOrInsert(insertP[i%benchmarkTreeSize], struct{}{})
	}
}

func BenchmarkDeleteInsertCloneEachTimeG(b *testing.B) {
	b.StopTimer()
	insertP := rand.Perm(benchmarkTreeSize)
	tr := newInt(*btreeDegree)
	for _, item := range insertP {
		tr.ReplaceOrInsert(item, struct{}{})
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tr = tr.Clone()
		tr.Delete(insertP[i%benchmarkTreeSize])
		tr.ReplaceOrInsert(insertP[i%benchmarkTreeSize], struct{}{})
	}
}

func BenchmarkDeleteG(b *testing.B) {
	b.StopTimer()
	insertP := rand.Perm(benchmarkTreeSize)
	removeP := rand.Perm(benchmarkTreeSize)
	b.StartTimer()
	i := 0
	for i < b.N {
		b.StopTimer()
		tr := newInt(*btreeDegree)
		for _, v := range insertP {
			tr.ReplaceOrInsert(v, struct{}{})
		}
		b.StartTimer()
		for _, item := range removeP {
			tr.Delete(item)
			i++
			if i >= b.N {
				return
			}
		}
		if tr.Len() > 0 {
			panic(tr.Len())
		}
	}
}

func BenchmarkGetG(b *testing.B) {
	b.StopTimer()
	insertP := rand.Perm(benchmarkTreeSize)
	removeP := rand.Perm(benchmarkTreeSize)
	b.StartTimer()
	i := 0
	for i < b.N {
		b.StopTimer()
		tr := newInt(*btreeDegree)
		for _, v := range insertP {
			tr.ReplaceOrInsert(v, struct{}{})
		}
		b.StartTimer()
		for _, item := range removeP {
			tr.Get(item)
			i++
			if i >= b.N {
				return
			}
		}
	}
}

func BenchmarkGetCloneEachTimeG(b *testing.B) {
	b.StopTimer()
	insertP := rand.Perm(benchmarkTreeSize)
	removeP := rand.Perm(benchmarkTreeSize)
	b.StartTimer()
	i := 0
	for i < b.N {
		b.StopTimer()
		tr := newInt(*btreeDegree)
		for _, v := range insertP {
			tr.ReplaceOrInsert(v, struct{}{})
		}
		b.StartTimer()
		for _, item := range removeP {
			tr = tr.Clone()
			tr.Get(item)
			i++
			if i >= b.N {
				return
			}
		}
	}
}

func BenchmarkAscendG(b *testing.B) {
	arr := rand.Perm(benchmarkTreeSize)
	tr := newInt(*btreeDegree)
	for _, v := range arr {
		tr.ReplaceOrInsert(v, struct{}{})
	}
	sort.Ints(arr)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := 0
		tr.Ascend(func(item int, _ struct{}) bool {
			if item != arr[j] {
				b.Fatalf("mismatch: expected: %v, got %v", arr[j], item)
			}
			j++
			return true
		})
	}
}

func BenchmarkDescendG(b *testing.B) {
	arr := rand.Perm(benchmarkTreeSize)
	tr := newInt(*btreeDegree)
	for _, v := range arr {
		tr.ReplaceOrInsert(v, struct{}{})
	}
	sort.Ints(arr)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := len(arr) - 1
		tr.Descend(func(item int, _ struct{}) bool {
			if item != arr[j] {
				b.Fatalf("mismatch: expected: %v, got %v", arr[j], item)
			}
			j--
			return true
		})
	}
}

func BenchmarkAscendRangeG(b *testing.B) {
	arr := rand.Perm(benchmarkTreeSize)
	tr := newInt(*btreeDegree)
	for _, v := range arr {
		tr.ReplaceOrInsert(v, struct{}{})
	}
	sort.Ints(arr)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := 100
		tr.AscendRange(100, arr[len(arr)-100], func(item int, _ struct{}) bool {
			if item != arr[j] {
				b.Fatalf("mismatch: expected: %v, got %v", arr[j], item)
			}
			j++
			return true
		})
		if j != len(arr)-100 {
			b.Fatalf("expected: %v, got %v", len(arr)-100, j)
		}
	}
}

func BenchmarkDescendRangeG(b *testing.B) {
	arr := rand.Perm(benchmarkTreeSize)
	tr := newInt(*btreeDegree)
	for _, v := range arr {
		tr.ReplaceOrInsert(v, struct{}{})
	}
	sort.Ints(arr)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := len(arr) - 100
		tr.DescendRange(arr[len(arr)-100], 100, func(item int, _ struct{}) bool {
			if item != arr[j] {
				b.Fatalf("mismatch: expected: %v, got %v", arr[j], item)
			}
			j--
			return true
		})
		if j != 100 {
			b.Fatalf("expected: %v, got %v", len(arr)-100, j)
		}
	}
}

func BenchmarkAscendGreaterOrEqualG(b *testing.B) {
	arr := rand.Perm(benchmarkTreeSize)
	tr := newInt(*btreeDegree)
	for _, v := range arr {
		tr.ReplaceOrInsert(v, struct{}{})
	}
	sort.Ints(arr)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := 100
		k := 0
		tr.AscendGreaterOrEqual(100, func(item int, _ struct{}) bool {
			if item != arr[j] {
				b.Fatalf("mismatch: expected: %v, got %v", arr[j], item)
			}
			j++
			k++
			return true
		})
		if j != len(arr) {
			b.Fatalf("expected: %v, got %v", len(arr), j)
		}
		if k != len(arr)-100 {
			b.Fatalf("expected: %v, got %v", len(arr)-100, k)
		}
	}
}

func BenchmarkDescendLessOrEqualG(b *testing.B) {
	arr := rand.Perm(benchmarkTreeSize)
	tr := newInt(*btreeDegree)
	for _, v := range arr {
		tr.ReplaceOrInsert(v, struct{}{})
	}
	sort.Ints(arr)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := len(arr) - 100
		k := len(arr)
		tr.DescendLessOrEqual(arr[len(arr)-100], func(item int, _ struct{}) bool {
			if item != arr[j] {
				b.Fatalf("mismatch: expected: %v, got %v", arr[j], item)
			}
			j--
			k--
			return true
		})
		if j != -1 {
			b.Fatalf("expected: %v, got %v", -1, j)
		}
		if k != 99 {
			b.Fatalf("expected: %v, got %v", 99, k)
		}
	}
}

const cloneTestSize = 10000

func cloneTestG(
	t *testing.T,
	b *bTree[int],
	start int,
	p []int,
	wg *sync.WaitGroup,
	trees *[]*bTree[int],
	lock *sync.Mutex,
) {
	t.Logf("Starting new clone at %v", start)
	lock.Lock()
	*trees = append(*trees, b)
	lock.Unlock()
	for i := start; i < cloneTestSize; i++ {
		b.ReplaceOrInsert(p[i], struct{}{})
		if i%(cloneTestSize/5) == 0 {
			wg.Add(1)
			go cloneTestG(t, b.Clone(), i+1, p, wg, trees, lock)
		}
	}
	wg.Done()
}

func TestCloneConcurrentOperationsG(t *testing.T) {
	b := newInt(*btreeDegree)
	trees := []*bTree[int]{}
	p := rand.Perm(cloneTestSize)
	var wg sync.WaitGroup
	wg.Add(1)
	go cloneTestG(t, b, 0, p, &wg, &trees, &sync.Mutex{})
	wg.Wait()
	want := intRange(cloneTestSize, false)
	t.Logf("Starting equality checks on %d trees", len(trees))
	for i, tree := range trees {
		if !reflect.DeepEqual(want, intAll(tree)) {
			t.Errorf("tree %v mismatch", i)
		}
	}
	t.Log("Removing half from first half")
	toRemove := intRange(cloneTestSize, false)[cloneTestSize/2:]
	for i := 0; i < len(trees)/2; i++ {
		tree := trees[i]
		wg.Add(1)
		go func() {
			for _, item := range toRemove {
				tree.Delete(item)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	t.Log("Checking all values again")
	for i, tree := range trees {
		var wantpart []int
		if i < len(trees)/2 {
			wantpart = want[:cloneTestSize/2]
		} else {
			wantpart = want
		}
		if got := intAll(tree); !reflect.DeepEqual(wantpart, got) {
			t.Errorf("tree %v mismatch, want %v got %v", i, len(want), len(got))
		}
	}
}

func BenchmarkDeleteAndRestoreG(b *testing.B) {
	items := rand.Perm(16392)
	b.ResetTimer()
	b.Run(`CopyBigFreeList`, func(b *testing.B) {
		fl := NewFreeList[int, struct{}](16392)
		tr := NewWithFreeList[int, struct{}](*btreeDegree, cmp.Compare[int], fl)
		for _, v := range items {
			tr.ReplaceOrInsert(v, struct{}{})
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			dels := make([]int, 0, tr.Len())
			tr.AscendFunc(Min[int](), Max[int](), func(b int, _ struct{}) bool {
				dels = append(dels, b)
				return true
			})
			for _, del := range dels {
				tr.Delete(del)
			}
			// tr is now empty, we make a new empty copy of it.
			tr = NewWithFreeList[int](*btreeDegree, cmp.Compare[int], fl)
			for _, v := range items {
				tr.ReplaceOrInsert(v, struct{}{})
			}
		}
	})
	b.Run(`Copy`, func(b *testing.B) {
		tr := newInt(*btreeDegree)
		for _, v := range items {
			tr.ReplaceOrInsert(v, struct{}{})
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			dels := make([]int, 0, tr.Len())
			tr.Ascend(func(b int, _ struct{}) bool {
				dels = append(dels, b)
				return true
			})
			for _, del := range dels {
				tr.Delete(del)
			}
			// tr is now empty, we make a new empty copy of it.
			tr = newInt(*btreeDegree)
			for _, v := range items {
				tr.ReplaceOrInsert(v, struct{}{})
			}
		}
	})
	b.Run(`ClearBigFreelist`, func(b *testing.B) {
		fl := NewFreeList[int, struct{}](16392)
		tr := NewWithFreeList[int, struct{}](*btreeDegree, cmp.Compare[int], fl)
		for _, v := range items {
			tr.ReplaceOrInsert(v, struct{}{})
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr.Clear(true)
			for _, v := range items {
				tr.ReplaceOrInsert(v, struct{}{})
			}
		}
	})
	b.Run(`Clear`, func(b *testing.B) {
		tr := newInt(*btreeDegree)
		for _, v := range items {
			tr.ReplaceOrInsert(v, struct{}{})
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr.Clear(true)
			for _, v := range items {
				tr.ReplaceOrInsert(v, struct{}{})
			}
		}
	})
}
