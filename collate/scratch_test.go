// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package collate

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"golang.org/x/text/internal/colltab"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

func referenceCompare(c *Collator, a, b string) int {
	var ia, ib iter
	ia.init(c)
	ib.init(c)
	ia.SetInputString(a)
	ib.SetInputString(b)
	if result := c.compareIters(&ia, &ib); result != 0 {
		return result
	}
	if !c.ignore[colltab.Identity] {
		return strings.Compare(a, b)
	}
	return 0
}

func TestConcurrentComparisonScratch(t *testing.T) {
	values := []string{"", "alpha-9", "alpha-10", "Aa", "Å", "a\u0327\u030a", "a\u030a\u0327", "É", "e\u0301", "ch", "ll", "ß", "SS", "한글", "\xff\xfe", strings.Repeat("é", 700)}
	for _, tag := range []language.Tag{language.Und, language.Danish, language.German, language.Spanish, language.French, language.Turkish} {
		c := New(tag, Numeric, IgnoreCase)
		want := make([]int, len(values)*len(values))
		for i, a := range values {
			for j, b := range values {
				want[i*len(values)+j] = referenceCompare(c, a, b)
			}
		}
		var workers sync.WaitGroup
		for n := 0; n < 8; n++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				for i, a := range values {
					for j, b := range values {
						expected := want[i*len(values)+j]
						if got := c.CompareString(a, b); got != expected {
							t.Errorf("%v CompareString(%q,%q)=%d want %d", tag, a, b, got, expected)
						}
						if got := c.Compare([]byte(a), []byte(b)); got != expected {
							t.Errorf("%v Compare(%q,%q)=%d want %d", tag, a, b, got, expected)
						}
					}
				}
			}()
		}
		workers.Wait()
	}
}

func TestComparisonScratchAllocations(t *testing.T) {
	c := New(language.Danish, Numeric, IgnoreCase)
	for _, pair := range [][2]string{{"account-9", "account-10"}, {"École-123", "école-124"}, {"a\u0327\u030a", "å"}, {"한글", "한국"}, {strings.Repeat("é", 700), strings.Repeat("é", 700) + "x"}} {
		t.Run(pair[0][:min(len(pair[0]), 12)], func(t *testing.T) {
			got := testing.AllocsPerRun(1000, func() { c.CompareString(pair[0], pair[1]) })
			if got != 0 {
				t.Fatalf("CompareString allocated %g objects per comparison", got)
			}
		})
	}
}

func TestKeyMatchesComparisonScratch(t *testing.T) {
	c := New(language.Danish, Numeric, IgnoreCase)
	var buf Buffer
	for _, pair := range [][2]string{{"alpha-9", "alpha-10"}, {"É", "e\u0301"}, {"a\u0327\u030a", "å"}} {
		buf.Reset()
		a := c.KeyFromString(&buf, pair[0])
		b := c.KeyFromString(&buf, pair[1])
		if got, want := bytes.Compare(a, b), c.CompareString(pair[0], pair[1]); got != want {
			t.Fatalf("keys=%d compare=%d", got, want)
		}
	}
}

func BenchmarkComparisonScratch(b *testing.B) {
	c := New(language.Danish, Numeric, IgnoreCase)
	for _, pair := range [][2]string{{"account-9", "account-10"}, {"École-123", "école-124"}, {"a\u0327\u030a", "å"}} {
		b.Run(pair[0], func(b *testing.B) {
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				c.CompareString(pair[0], pair[1])
			}
		})
	}
}

func TestHangulComparisonDecomposition(t *testing.T) {
	c := New(language.Und)
	for r := rune(0xAC00); r <= 0xD7A3; r++ {
		input := string(r)
		if got := c.CompareString(input, norm.NFD.String(input)); got != 0 {
			t.Fatalf("Hangul U+%04X differs from NFD: %d", r, got)
		}
	}
}
