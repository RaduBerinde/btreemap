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

import (
	"cmp"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"
)

// TestIteration tests the iteration of the B+ tree map. It inserts even number
// elements in random order then tests the iteration results for random ranges
// using various types of bounds.
func TestIteration(t *testing.T) {
	for it := 0; it < 100; it++ {
		seed := rand.Uint64()
		rng := rand.New(rand.NewPCG(seed, 0))
		n := 1 + int(rng.ExpFloat64()*10)
		degree := 2 + int(rng.ExpFloat64()*4)
		m := New[int, int](degree, cmp.Compare[int])
		// Insert numbers 2, 4, ..., 2n in random order.
		for i := range rng.Perm(n) {
			x := 2 * (i + 1)
			m.ReplaceOrInsert(x, x*x)
		}
		for it2 := 0; it2 < 100; it2++ {
			check := func(low LowerBound[int], high UpperBound[int], start, stop int) {
				t.Helper()
				var expected []int
				for i := start; i <= stop; i += 2 {
					expected = append(expected, i)
				}
				var actual []int
				for k, v := range m.Ascend(low, high) {
					if v != k*k {
						t.Fatalf("invalid value")
					}
					actual = append(actual, k)
				}
				if !reflect.DeepEqual(expected, actual) {
					t.Fatalf("seed: %d expected %v, Ascend produced %v", seed, expected, actual)
				}
				slices.Reverse(expected)
				actual = actual[:0]
				for k, v := range m.Descend(high, low) {
					if v != k*k {
						t.Fatalf("invalid value")
					}
					actual = append(actual, k)
				}
				if !reflect.DeepEqual(expected, actual) {
					t.Fatalf("seed: %d expected %v, Descend produced %v", seed, expected, actual)
				}
			}
			checkEmpty := func(low LowerBound[int], hi UpperBound[int]) {
				t.Helper()
				for k := range m.Ascend(low, hi) {
					t.Fatalf("seed: %d unexpected Ascend key %v", seed, k)
				}
				for k := range m.Descend(hi, low) {
					t.Fatalf("seed: %d unexpected Descend key %v", seed, k)
				}
			}

			// low are lower bounds which yield x as the smallest element.
			low := func(x int) []LowerBound[int] {
				return []LowerBound[int]{GE(x), GT(x - 1), GE(x - 1), GT(x - 2)}
			}
			// high are upper bounds which yield a as the largest element.
			high := func(x int) []UpperBound[int] {
				return []UpperBound[int]{LE(x), LT(x + 1), LE(x + 1), LT(x + 2)}
			}

			a := 2 * (rng.IntN(n) + 1)

			for _, h := range high(a) {
				check(Min[int](), h, 2, a)
			}
			for _, l := range low(a) {
				check(l, Max[int](), a, 2*n)
			}
			for _, l := range low(a) {
				for _, h := range high(a - 2) {
					checkEmpty(l, h)
				}
			}

			b := 2 * (rng.IntN(n) + 1)
			if a > b {
				a, b = b, a
			}

			for _, l := range low(a) {
				for _, h := range high(b) {
					check(l, h, a, b)
				}
			}
		}
	}
}

func TestRandomized(t *testing.T) {
	for it := 0; it < 200; it++ {
		seed := rand.Uint64()
		rng := rand.New(rand.NewPCG(seed, 0))
		degree := 2 + int(rng.ExpFloat64()*4)
		maxKey := rng.IntN(maxNaiveSize)

		for ops := 0; ops < 1000; ops++ {
			type instance struct {
				m *BTreeMap[int, int]
				n *naive
			}
			// We maintain a forest of trees.
			instances := []instance{{m: New[int, int](degree, cmp.Compare[int]), n: &naive{}}}
			const maxInstances = 10

			i := instances[rng.IntN(len(instances))]

			crossCheck(t, rng, seed, maxKey, i.m, i.n)

			op := rng.IntN(100)
			switch {
			case op <= 60:
				k := rng.IntN(maxKey + 1)
				v := 1 + rng.IntN(10000)

				ak, av, ab := i.m.ReplaceOrInsert(k, v)
				ek, ev, eb := i.n.ReplaceOrInsert(k, v)
				if ak != ek || av != ev || ab != eb {
					t.Fatalf("seed: %d ReplaceOrInsert(%d, %d) got (%d, %d, %v), want (%d, %d, %v)", seed, k, v, ak, av, ab, ek, ev, eb)
				}

			case op <= 90:
				k := rng.IntN(maxKey + 1)
				ak, av, ab := i.m.Delete(k)
				ek, ev, eb := i.n.Delete(k)
				if ak != ek || av != ev || ab != eb {
					t.Fatalf("seed: %d Delete(%d) got (%d, %d, %v), want (%d, %d, %v)", seed, k, ak, av, ab, ek, ev, eb)
				}

			case op <= 93:
				ak, av, ab := i.m.DeleteMin()
				ek, ev, eb := i.n.Min()
				if eb {
					i.n.Delete(ek)
				}
				if ak != ek || av != ev || ab != eb {
					t.Fatalf("seed: %d DeleteMin() got (%d, %d, %v), want (%d, %d, %v)", seed, ak, av, ab, ek, ev, eb)
				}

			case op <= 96:
				ak, av, ab := i.m.DeleteMax()
				ek, ev, eb := i.n.Max()
				if eb {
					i.n.Delete(ek)
				}
				if ak != ek || av != ev || ab != eb {
					t.Fatalf("seed: %d DeleteMax() got (%d, %d, %v), want (%d, %d, %v)", seed, ak, av, ab, ek, ev, eb)
				}

			default:
				// Clone.
				c := instance{
					m: i.m.Clone(),
					n: i.n.Clone(),
				}
				if len(instances) < maxInstances {
					instances = append(instances, c)
				} else {
					instances[rng.IntN(len(instances))] = c
				}
			}
		}
	}
}

// crossCheck runs various queries on m and cross-checks the results using n.
func crossCheck(
	t *testing.T, rng *rand.Rand, seed uint64, maxKey int, m *BTreeMap[int, int], n *naive,
) {
	ek, ev, eb := n.Min()
	if k, v, b := m.Min(); k != ek || v != ev || b != eb {
		t.Fatalf("seed: %d invalid Min", seed)
	}
	ek, ev, eb = n.Max()
	if k, v, b := m.Max(); k != ek || v != ev || b != eb {
		t.Fatalf("seed: %d invalid Max", seed)
	}

	for checks := 0; checks < 40; checks++ {
		a := rng.IntN(maxKey + 1)

		if val := n.values[a]; val == 0 {
			if m.Has(a) {
				t.Fatalf("seed: %d Has(%d) should be false", seed, a)
			}
			k, v, ok := m.Get(a)
			if k != 0 || v != 0 || ok {
				t.Fatalf("seed: %d Get(%d) should be (0, 0, false)", seed, a)
			}
		} else {
			if !m.Has(a) {
				t.Fatalf("seed: %d Has(%d) should be true", seed, a)
			}
			k, v, ok := m.Get(a)
			if k != a || v != val || !ok {
				t.Fatalf("seed: %d Get(%d) should be (%d, %d, true)", seed, a, a, val)
			}
		}

		b := rng.IntN(maxKey + 1)
		if a > b {
			a, b = b, a
		}
		expected := n.KeysInRange(a, b)

		low := GE(a)
		if rng.IntN(2) == 0 {
			low = GT(a - 1)
		}
		if a == 0 && rng.IntN(2) == 0 {
			low = Min[int]()
		}
		high := LE(b)
		if rng.IntN(2) == 0 {
			high = LT(b + 1)
		}
		if b == maxKey && rng.IntN(2) == 0 {
			high = Max[int]()
		}
		var actual []int
		for k, v := range m.Ascend(low, high) {
			if v != n.values[k] {
				t.Fatalf("seed: %d invalid value for key %d", seed, k)
			}
			actual = append(actual, k)
		}
		if !reflect.DeepEqual(expected, actual) {
			t.Fatalf("seed: %d expected %v, Ascend produced %v", seed, expected, actual)
		}
		slices.Reverse(expected)
		actual = actual[:0]
		for k, v := range m.Descend(high, low) {
			if v != n.values[k] {
				t.Fatalf("seed: %d invalid value for key %d", seed, k)
			}
			actual = append(actual, k)
		}
		if !reflect.DeepEqual(expected, actual) {
			t.Fatalf("seed: %d expected %v, Descend produced %v", seed, expected, actual)
		}
	}
}

const maxNaiveSize = 1000

type naive struct {
	values [maxNaiveSize]int
}

func (n *naive) ReplaceOrInsert(k int, v int) (int, int, bool) {
	old := n.values[k]
	n.values[k] = v
	if old != 0 {
		return k, old, true
	}
	return 0, 0, false
}

func (n *naive) KeysInRange(low, high int) []int {
	var r []int
	for i := low; i <= high; i++ {
		if n.values[i] != 0 {
			r = append(r, i)
		}
	}
	return r
}

func (n *naive) Min() (int, int, bool) {
	for i, v := range n.values {
		if v != 0 {
			return i, v, true
		}
	}
	return 0, 0, false
}

func (n *naive) Max() (int, int, bool) {
	for i := len(n.values) - 1; i >= 0; i-- {
		if n.values[i] != 0 {
			return i, n.values[i], true
		}
	}
	return 0, 0, false
}

func (n *naive) Delete(k int) (int, int, bool) {
	old := n.values[k]
	n.values[k] = 0
	if old != 0 {
		return k, old, true
	}
	return 0, 0, false
}

func (n *naive) Clone() *naive {
	return &naive{values: n.values}
}
