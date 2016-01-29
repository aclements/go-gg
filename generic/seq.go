// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package generic

import "reflect"

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
