// Copyright 2017 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package intsets

import (
	"bytes"
	b64 "encoding/base64"
	"fmt"
	"reflect"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util"
	"github.com/cockroachdb/cockroach/pkg/util/base64"
	"github.com/cockroachdb/cockroach/pkg/util/randutil"
)

func TestFast(t *testing.T) {
	for _, minVal := range []int{-10, -1, 0, 8, smallCutoff, 2 * smallCutoff} {
		for _, maxVal := range []int{-1, 1, 8, 30, smallCutoff, 2 * smallCutoff, 4 * smallCutoff} {
			if maxVal <= minVal {
				continue
			}
			// We are using Parallel, we need to make local instances of the loop
			// variables.
			minVal := minVal
			maxVal := maxVal
			t.Run(fmt.Sprintf("%d_%d", minVal, maxVal), func(t *testing.T) {
				t.Parallel() // SAFE FOR TESTING (this comment is for the linter)
				rng, _ := randutil.NewTestRand()
				in := make(map[int]bool)
				forEachRes := make(map[int]bool)

				var s Fast
				for i := 0; i < 1000; i++ {
					v := minVal + rng.Intn(maxVal-minVal)
					if rng.Intn(2) == 0 {
						in[v] = true
						s.Add(v)
					} else {
						in[v] = false
						s.Remove(v)
					}
					empty := true
					for j := minVal; j < maxVal; j++ {
						empty = empty && !in[j]
						if in[j] != s.Contains(j) {
							t.Fatalf("incorrect result for Contains(%d), expected %t", j, in[j])
						}
					}
					if empty != s.Empty() {
						t.Fatalf("incorrect result for Empty(), expected %t", empty)
					}
					// Test ForEach
					for j := range forEachRes {
						forEachRes[j] = false
					}
					s.ForEach(func(j int) {
						forEachRes[j] = true
					})
					for j := minVal; j < maxVal; j++ {
						if in[j] != forEachRes[j] {
							t.Fatalf("incorrect ForEachResult for %d (%t, expected %t)", j, forEachRes[j], in[j])
						}
					}
					// Cross-check Ordered and Next().
					var vals []int
					// Start at the minimum value, or before.
					startVal := minVal - rng.Intn(10)
					for i, ok := s.Next(startVal); ok; i, ok = s.Next(i + 1) {
						vals = append(vals, i)
					}
					if o := s.Ordered(); !reflect.DeepEqual(vals, o) {
						t.Fatalf("set built with Next doesn't match Ordered: %v vs %v", vals, o)
					}
					assertSame := func(orig, copy Fast) {
						t.Helper()
						if !orig.Equals(copy) || !copy.Equals(orig) {
							t.Fatalf("expected equality: %v, %v", orig, copy)
						}
						if col, ok := copy.Next(startVal); ok {
							copy.Remove(col)
							if orig.Equals(copy) || copy.Equals(orig) {
								t.Fatalf("unexpected equality: %v, %v", orig, copy)
							}
							copy.Add(col)
							if !orig.Equals(copy) || !copy.Equals(orig) {
								t.Fatalf("expected equality: %v, %v", orig, copy)
							}
						}
					}
					// Test Copy.
					s2 := s.Copy()
					assertSame(s, s2)
					// Test CopyFrom.
					var s3 Fast
					s3.CopyFrom(s)
					assertSame(s, s3)
					// Make sure CopyFrom into a non-empty set still works.
					s3.Add(minVal + rng.Intn(maxVal-minVal))
					s3.CopyFrom(s)
					assertSame(s, s3)

					// Test Encode/Decode.
					if minVal >= 0 {
						var buf bytes.Buffer
						if err := s.Encode(&buf); err != nil {
							t.Fatalf("error during Encode: %v", err)
						}
						encoded := buf.String()
						var s2 Fast
						if err := s2.Decode(bytes.NewReader([]byte(encoded))); err != nil {
							t.Fatalf("error during Decode: %v", err)
						}
						assertSame(s, s2)
						// Verify that EncodeBase64 decodes to the result of
						// Encoded.
						var enc base64.Encoder
						var hash util.FNV64
						enc.Init(b64.StdEncoding)
						hash.Init()
						if err := s.EncodeBase64(&enc, &hash); err != nil {
							t.Fatalf("error during EncodeBase64: %v", err)
						}
						dec, err := b64.StdEncoding.DecodeString(enc.String())
						if err != nil {
							t.Fatalf("error during base64 Decode: %v", err)
						}
						if encoded != string(dec) {
							t.Fatalf("expected decoded base64, %q, to match encoding %q", string(dec), encoded)
						}
						// Verify that decoding into a non-empty set still works.
						var s3 Fast
						s3.Add(minVal + rng.Intn(maxVal-minVal))
						if err := s3.Decode(bytes.NewReader([]byte(encoded))); err != nil {
							t.Fatalf("error during Decode: %v", err)
						}
						assertSame(s, s3)
					}
				}
			})
		}
	}
}

func TestFastTwoSetOps(t *testing.T) {
	rng, _ := randutil.NewTestRand()
	// genSet creates a set of numElem values in [minVal, minVal + valRange)
	// It also adds and then removes numRemoved elements.
	genSet := func(numElem, numRemoved, minVal, valRange int) (Fast, map[int]bool) {
		var s Fast
		vals := rng.Perm(valRange)[:numElem+numRemoved]
		used := make(map[int]bool, len(vals))
		for _, i := range vals {
			used[i] = true
		}
		for k := range used {
			s.Add(k)
		}
		p := rng.Perm(len(vals))
		for i := 0; i < numRemoved; i++ {
			k := vals[p[i]]
			s.Remove(k)
			delete(used, k)
		}
		return s, used
	}

	// returns true if a is a subset of b
	subset := func(a, b map[int]bool) bool {
		for k := range a {
			if !b[k] {
				return false
			}
		}
		return true
	}

	for _, minVal := range []int{-10, -1, 0, smallCutoff, 2 * smallCutoff} {
		for _, valRange := range []int{0, 20, 200} {
			for _, num1 := range []int{0, 1, 5, 10, 20} {
				for _, removed1 := range []int{0, 1, 3, 8} {
					s1, m1 := genSet(num1, removed1, minVal, num1+removed1+valRange)
					for _, num2 := range []int{0, 1, 5, 10, 20} {
						for _, removed2 := range []int{0, 1, 4, 10} {
							s2, m2 := genSet(num2, removed2, minVal, num2+removed2+valRange)

							subset1 := subset(m1, m2)
							if subset1 != s1.SubsetOf(s2) {
								t.Errorf("SubsetOf result incorrect: %s, %s", &s1, &s2)
							}
							subset2 := subset(m2, m1)
							if subset2 != s2.SubsetOf(s1) {
								t.Errorf("SubsetOf result incorrect: %s, %s", &s2, &s1)
							}
							eq := subset1 && subset2
							if eq != s1.Equals(s2) || eq != s2.Equals(s1) {
								t.Errorf("Equals result incorrect: %s, %s", &s1, &s2)
							}

							// Test union.

							u := s1.Copy()
							u.UnionWith(s2)

							if !u.Equals(s1.Union(s2)) {
								t.Errorf("inconsistency between UnionWith and Union on %s %s\n", s1, s2)
							}
							// Verify all elements from m1 and m2 are in u.
							for _, m := range []map[int]bool{m1, m2} {
								for x := range m {
									if !u.Contains(x) {
										t.Errorf("incorrect union result %s union %s = %s", &s1, &s2, &u)
										break
									}
								}
							}
							// Verify all elements from u are in m2 or m1.
							for x, ok := u.Next(minVal); ok; x, ok = u.Next(x + 1) {
								if !(m1[x] || m2[x]) {
									t.Errorf("incorrect union result %s union %s = %s", &s1, &s2, &u)
									break
								}
							}

							// Test intersection.
							u = s1.Copy()
							u.IntersectionWith(s2)
							if s1.Intersects(s2) != !u.Empty() ||
								s2.Intersects(s1) != !u.Empty() {
								t.Errorf("inconsistency between IntersectionWith and Intersect on %s %s\n", s1, s2)
							}
							if !u.Equals(s1.Intersection(s2)) {
								t.Errorf("inconsistency between IntersectionWith and Intersection on %s %s\n", s1, s2)
							}
							// Verify all elements from m1 and m2 are in u.
							for x := range m1 {
								if m2[x] && !u.Contains(x) {
									t.Errorf("incorrect intersection result %s union %s = %s  x=%d", &s1, &s2, &u, x)
									break
								}
							}
							// Verify all elements from u are in m2 and m1.
							for x, ok := u.Next(minVal); ok; x, ok = u.Next(x + 1) {
								if !(m1[x] && m2[x]) {
									t.Errorf("incorrect intersection result %s intersect %s = %s", &s1, &s2, &u)
									break
								}
							}

							// Test difference.
							u = s1.Copy()
							u.DifferenceWith(s2)

							if !u.Equals(s1.Difference(s2)) {
								t.Errorf("inconsistency between DifferenceWith and Difference on %s %s\n", s1, s2)
							}

							// Verify all elements in m1 but not in m2 are in u.
							for x := range m1 {
								if !m2[x] && !u.Contains(x) {
									t.Errorf("incorrect difference result %s \\ %s = %s  x=%d", &s1, &s2, &u, x)
									break
								}
							}
							// Verify all elements from u are in m1.
							for x, ok := u.Next(minVal); ok; x, ok = u.Next(x + 1) {
								if !m1[x] {
									t.Errorf("incorrect difference result %s \\ %s = %s", &s1, &s2, &u)
									break
								}
							}
						}
					}
				}
			}
		}
	}
}

func TestFastAddRange(t *testing.T) {
	assertSet := func(set *Fast, from, to int) {
		t.Helper()
		// Iterate through the set and ensure that the values
		// it contain are the values from 'from' to 'to' (inclusively).
		expected := from
		set.ForEach(func(actual int) {
			t.Helper()
			if actual > to {
				t.Fatalf("expected last value in Fast to be %d, got %d", to, actual)
			}
			if expected != actual {
				t.Fatalf("expected next value in Fast to be %d, got %d", expected, actual)
			}
			expected++
		})
	}

	max := smallCutoff + 20
	// Test all O(n^2) sub-intervals of [from,to] in the interval
	// [-5, smallCutoff + 20].
	for from := -5; from <= max; from++ {
		for to := from; to <= max; to++ {
			var set Fast
			set.AddRange(from, to)
			assertSet(&set, from, to)
		}
	}
}

func TestFastString(t *testing.T) {
	testCases := []struct {
		vals []int
		exp  string
	}{
		{
			vals: []int{},
			exp:  "()",
		},
		{
			vals: []int{-5, -3, -2, -1, 0, 1, 2, 3, 4, 5},
			exp:  "(-5,-3,-2,-1,0-5)",
		},
		{
			vals: []int{0, 1, 3, 4, 5},
			exp:  "(0,1,3-5)",
		},
		{
			vals: []int{-1, 0, 1, 127, 128, 255, 256, 257, 512},
			exp:  "(-1,0,1,127,128,255-257,512)",
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			s := MakeFast(tc.vals...)
			if str := s.String(); str != tc.exp {
				t.Errorf("expected %s, got %s", tc.exp, str)
			}
		})
	}
}
