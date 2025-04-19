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
	for it := 0; it < 1000; it++ {
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
				for k, v := range m.NewAscend(low, high) {
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
				for k, v := range m.NewDescend(high, low) {
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
				for k := range m.NewAscend(low, hi) {
					t.Fatalf("seed: %d unexpected Ascend key %v", seed, k)
				}
				for k := range m.NewDescend(hi, low) {
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
