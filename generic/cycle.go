// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package generic

import "reflect"

// Cycle constructs a sequence of length length by repeatedly
// concatenating seq to itself. If len(seq) >= length, it will slice
// seq to length. Otherwise, it will allocate a new slice. Regardless
// of seq's type, Cycle always returns a slice. If len(seq) == 0 and
// length != 0, Cycle panics.
func Cycle(seq interface{}, length int) interface{} {
	rv := sequence(seq)
	if rv.Len() >= length {
		return rv.Slice(0, length).Interface()
	}

	if rv.Len() == 0 {
		panic("empty slice")
	}

	// Allocate a new slice of the appropriate length.
	ot := rv.Type()
	if rv.Kind() == reflect.Array {
		ot = reflect.SliceOf(ot.Elem())
	}
	out := reflect.MakeSlice(ot, length, length)

	// Copy elements to out.
	for pos := 0; pos < length; {
		pos += reflect.Copy(out.Slice(pos, length), rv)
	}

	return out.Interface()
}
