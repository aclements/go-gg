// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

import (
	"sort"

	"github.com/aclements/go-gg/generic"
)

// SortBy sorts each group of g by the named column. If the column's
// type implements sort.Interface, rows will be sorted according to
// that order. Otherwise, the values in the column must be naturally
// ordered (their types must be orderable by the Go specification). If
// neither is true, SortBy panics with a *generic.TypeError.
func SortBy(g Grouping, col string) Grouping {
	// TODO: Consider a generic MapConcatGroups.

	// Sort each group.
	out := Grouping(new(Table))
	for _, gid := range g.Groups() {
		t := g.Table(gid)
		seq := t.MustColumn(col)
		sorter := generic.Sorter(seq)

		if sort.IsSorted(sorter) {
			// Avoid shuffling everything by the identity
			// permutation.
			out = out.AddTable(gid, t)
			continue
		}

		// Generate an initial permutation sequence.
		perm := make([]int, t.Len())
		for i := range perm {
			perm[i] = i
		}

		// Sort the permutation sequence using b as the key.
		sort.Stable(&permSort{perm, sorter})

		// Permute all columns.
		nt := new(Table)
		for _, name := range t.Columns() {
			seq := t.Column(name)
			seq = generic.MultiIndex(seq, perm)
			nt = nt.Add(name, seq)
		}
		out = out.AddTable(gid, nt)
	}

	return out
}

type permSort struct {
	perm []int
	key  sort.Interface
}

func (s *permSort) Len() int {
	return len(s.perm)
}

func (s *permSort) Less(i, j int) bool {
	return s.key.Less(s.perm[i], s.perm[j])
}

func (s *permSort) Swap(i, j int) {
	s.perm[i], s.perm[j] = s.perm[j], s.perm[i]
}
