// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slice

import "reflect"

// A Slice is a Go slice value.
//
// This is primarily for documentation. There is no way to statically
// enforce this in Go; however, functions that expect a Slice will
// panic with a *TypeError if passed a non-slice value.
type Slice interface{}

// reflectSlice checks that s is a slice and returns its
// reflect.Value. It panics with a *TypeError if s is not a slice.
func reflectSlice(s Slice) reflect.Value {
	rv := reflect.ValueOf(s)
	if rv.Kind() != reflect.Slice {
		panic(&TypeError{rv.Type(), nil, "is not a slice"})
	}
	return rv
}
