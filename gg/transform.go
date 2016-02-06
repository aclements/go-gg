// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import "github.com/aclements/go-gg/table"

// SortBy sorts each group by column col. If the column's type
// implements sort.Interface, rows will be sorted according to that
// order. Otherwise, the values in the column must be naturally
// ordered (their types must be orderable by the Go specification). If
// neither is true, SortBy panics with a *generic.TypeError.
func (p *Plot) SortBy(col string) *Plot {
	return p.SetData(table.SortBy(p.Data(), col))
}
