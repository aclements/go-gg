// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

import (
	"reflect"

	"github.com/aclements/go-gg/generic"
)

// FilterEq filters g to only rows where the value in col equals val.
func FilterEq(g Grouping, col string, val interface{}) Grouping {
	match := make([]int, 0)
	return MapTables(func(_ GroupID, t *Table) *Table {
		// Find the set of row indexes that match val.
		seq := t.MustColumn(col)
		match = match[:0]
		rv := reflect.ValueOf(seq)
		for i, len := 0, rv.Len(); i < len; i++ {
			if rv.Index(i).Interface() == val {
				match = append(match, i)
			}
		}

		nt := new(Table)
		for _, col := range t.Columns() {
			nt = nt.Add(col, generic.MultiIndex(t.Column(col), match))
		}
		return nt
	}, g)
}
