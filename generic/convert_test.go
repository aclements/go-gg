// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package generic

import "testing"

func TestConvert(t *testing.T) {
	var is []int
	ConvertSlice(&is, []int{1, 2, 3})
	if w := []int{1, 2, 3}; !de(w, is) {
		t.Errorf("want %v; got %v", w, is)
	}
	ConvertSlice(&is, []float64{1, 2, 3})
	if w := []int{1, 2, 3}; !de(w, is) {
		t.Errorf("want %v; got %v", w, is)
	}

	var fs []float64
	ConvertSlice(&fs, []int{1, 2, 3})
	if w := []float64{1, 2, 3}; !de(w, fs) {
		t.Errorf("want %v; got %v", w, fs)
	}
	ConvertSlice(&fs, []float64{1, 2, 3})
	if w := []float64{1, 2, 3}; !de(w, fs) {
		t.Errorf("want %v; got %v", w, fs)
	}

	shouldPanic(t, "cannot be converted", func() {
		ConvertSlice(&is, []string{"1", "2", "3"})
	})
	shouldPanic(t, `is not a \*\[\]T`, func() {
		ConvertSlice(is, []int{1, 2, 3})
	})
	shouldPanic(t, `is not a \*\[\]T`, func() {
		x := 1
		ConvertSlice(&x, []int{1, 2, 3})
	})
	shouldPanic(t, "is not a slice", func() {
		ConvertSlice(&is, 1)
	})
}
