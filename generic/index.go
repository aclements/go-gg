// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package generic

import "reflect"

// MultiIndex returns a slice w such that w[i] = v[indexes[i]]. v must
// be a slice, array, or pointer to array.
func MultiIndex(v interface{}, indexes []int) interface{} {
	switch v := v.(type) {
	case []int:
		res := make([]int, len(indexes))
		for i, x := range indexes {
			res[i] = v[x]
		}
		return res

	case []float64:
		res := make([]float64, len(indexes))
		for i, x := range indexes {
			res[i] = v[x]
		}
		return res

	case []string:
		res := make([]string, len(indexes))
		for i, x := range indexes {
			res[i] = v[x]
		}
		return res
	}

	rv := sequence(v)
	res := reflect.MakeSlice(rv.Type(), len(indexes), len(indexes))
	for i, x := range indexes {
		res.Index(i).Set(rv.Index(x))
	}
	return res.Interface()
}
