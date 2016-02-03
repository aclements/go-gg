// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package generic

import "reflect"

// TODO: This would all be *much* simpler if I just said everything
// had to be a slice. Arrays also have the annoying property that I
// often can't retain their true type, whereas if a slice is wrapped
// in a named type, I can make a new slice with the same named type.
// OTOH, in-place operations like sorting make perfect sense on
// arrays.

// sequence returns the reflect.Value of x that is safe to Index. x
// must be an array, slice, or pointer to an array. If x is a pointer
// to an array, sequence returned the dereferenced pointer. Otherwise,
// sequence panics with a *TypeError.
func sequence(x interface{}) reflect.Value {
	rv := reflect.ValueOf(x)

	switch rv.Kind() {
	case reflect.Ptr:
		if elem := rv.Elem(); elem.Kind() == reflect.Array {
			return elem
		}

	case reflect.Array, reflect.Slice:
		return rv
	}

	panic(&TypeError{rv.Type(), nil, "is not a sequence type"})
}

// newSequence returns a new slice with the specified length and cap.
// If typ is a slice type, the returned slice will have the same type.
// If typ is an array type, the returned slice will have the same
// element type.
func newSequence(typ reflect.Type, len, cap int) reflect.Value {
	switch typ.Kind() {
	case reflect.Slice:
		// Retain the slice's actual type.
		return reflect.MakeSlice(typ, len, cap)

	case reflect.Array:
		// Create a new slice type with the same element type.
		typ = reflect.SliceOf(typ.Elem())
		return reflect.MakeSlice(typ, len, cap)

	default:
		panic(&TypeError{typ, nil, "not a slice or array type"})
	}
}
