// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package generic

import "reflect"

// ConvertSlice converts each element in from and assigns it to *to.
// to must be a pointer to a slice. ConvertSlice slices or extends *to
// to len(from) and then assigns to[i] = T(from[i]) where T is the
// type of *to's elements. If from and *to have the same element type,
// it simply assigns *to = from.
func ConvertSlice(to interface{}, from Slice) {
	fv := reflectSlice(from)
	tv := reflect.ValueOf(to)
	if tv.Kind() != reflect.Ptr {
		panic(&TypeError{tv.Type(), nil, "is not a *[]T"})
	}
	tsv := tv.Elem()
	if tsv.Kind() != reflect.Slice {
		panic(&TypeError{tv.Type(), nil, "is not a *[]T"})
	}

	if fv.Type().AssignableTo(tsv.Type()) {
		tv.Elem().Set(fv)
		return
	}

	eltt := tsv.Type().Elem()
	if !fv.Type().Elem().ConvertibleTo(eltt) {
		panic(&TypeError{fv.Type(), tsv.Type(), "cannot be converted"})
	}

	switch to := to.(type) {
	case *[]float64:
		// This is extremely common.
		*to = (*to)[:0]
		for i, len := 0, fv.Len(); i < len; i++ {
			*to = append(*to, fv.Index(i).Convert(eltt).Float())
		}

	default:
		tsv.SetLen(0)
		for i, len := 0, fv.Len(); i < len; i++ {
			tsv = reflect.Append(tsv, fv.Index(i).Convert(eltt))
		}
		tv.Elem().Set(tsv)
	}
}
