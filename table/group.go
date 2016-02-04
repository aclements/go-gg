// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

import (
	"reflect"
	"strconv"

	"github.com/aclements/go-gg/generic"
)

// GroupID identifies a group. GroupIDs form a tree, rooted at
// RootGroupID (which is also the zero GroupID).
type GroupID struct {
	*groupNode
}

// RootGroupID is the root of the GroupID tree.
var RootGroupID = GroupID{}

type groupNode struct {
	parent GroupID
	label  string
}

// String returns the labels of the path to GroupID g, separated by
// "/"s. If g is RootGroupID, it returns "/". Note that this is purely
// diagnostic; this string may not uniquely identify g.
func (g GroupID) String() string {
	if g == RootGroupID {
		return "/"
	}
	buflen := -1
	for p := g; p != RootGroupID; p = p.parent {
		buflen += len(p.label) + 1
	}
	buf := make([]byte, buflen)
	bufpos := len(buf)
	for p := g; p != RootGroupID; p = p.parent {
		if p != g {
			bufpos--
			buf[bufpos] = '/'
		}
		bufpos -= len(p.label)
		copy(buf[bufpos:], p.label)
	}
	return string(buf)
}

// Extend returns a new GroupID that is a child of GroupID g. The
// returned GroupID will not be equal to any existing GroupID. The
// label is purely diagnostic; nothing enforces the uniqueness of this
// label within g.
func (g GroupID) Extend(label string) GroupID {
	return GroupID{&groupNode{g, label}}
}

// Parent returns the parent of g. The parent of RootGroupID is
// RootGroupID.
func (g GroupID) Parent() GroupID {
	if g == RootGroupID {
		return RootGroupID
	}
	return g.parent
}

// GroupBy sub-divides all groups such that all of the rows in each
// group have equal values for all of the named columns. The relative
// order of rows with equal values for the named columns is
// maintained.
func GroupBy(g Grouped, cols ...string) Grouped {
	// TODO: This would generate much less garbage if we grouped
	// all of cols in one pass.

	if len(cols) == 0 {
		return g
	}

	out := Grouped(new(Table))
	for _, gid := range g.Groups() {
		t := g.Table(gid)
		c := t.MustColumn(cols[0])

		// Create an index on c.
		subgroups := make(map[interface{}]GroupID)
		rowsMap := make(map[GroupID][]int)
		seq := reflect.ValueOf(c)
		for i := 0; i < seq.Len(); i++ {
			x := seq.Index(i).Interface()
			subgid, ok := subgroups[x]
			if !ok {
				subgid = gid.Extend(strconv.Itoa(len(subgroups)))
				subgroups[x] = subgid
				rowsMap[subgid] = []int{}
			}
			rowsMap[subgid] = append(rowsMap[subgid], i)
		}

		// Split this group in all columns.
		for subgid, rows := range rowsMap {
			// Construct this new group.
			subgroup := new(Table)
			for _, name := range t.Columns() {
				seq := t.Column(name)
				seq = generic.MultiIndex(seq, rows)
				subgroup = subgroup.Add(name, seq)
			}
			out = out.AddTable(subgid, subgroup)
		}
	}

	return GroupBy(out, cols[1:]...)
}

// Flatten concatenates all of the groups in g into a single Table.
func Flatten(g Grouped) *Table {
	groups := g.Groups()
	switch len(groups) {
	case 0:
		return new(Table)

	case 1:
		return g.Table(groups[0])
	}

	// Construct each column.
	out := new(Table)
	seqs := make([]reflect.Value, len(g.Groups()))
	for _, col := range g.Columns() {
		seqs = seqs[:0]
		for _, gid := range groups {
			seqs = append(seqs, reflect.ValueOf(g.Table(gid).Column(col)))
		}
		nilSeq := reflect.Zero(reflect.TypeOf(seqs[0]))
		newSeq := reflect.Append(nilSeq, seqs...)
		out.Add(col, newSeq)
	}

	return out
}
