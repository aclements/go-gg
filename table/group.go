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

// String returns the path to GroupID g in the form "/l1/l2/l3". If g
// is RootGroupID, it returns "/". Note that this is purely
// diagnostic; this string may not uniquely identify g.
func (g GroupID) String() string {
	if g == RootGroupID {
		return "/"
	}
	buflen := 0
	for p := g; p != RootGroupID; p = p.parent {
		buflen += len(p.label) + 1
	}
	buf := make([]byte, buflen)
	bufpos := len(buf)
	for p := g; p != RootGroupID; p = p.parent {
		bufpos -= len(p.label)
		copy(buf[bufpos:], p.label)
		bufpos--
		buf[bufpos] = '/'
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
func GroupBy(g Grouping, cols ...string) Grouping {
	// TODO: This would generate much less garbage if we grouped
	// all of cols in one pass.

	if len(cols) == 0 {
		return g
	}

	out := Grouping(new(Table))
	for _, gid := range g.Tables() {
		t := g.Table(gid)
		c := t.MustColumn(cols[0])

		// Create an index on c.
		subgroups := []GroupID{}
		gidkey := make(map[interface{}]GroupID)
		rowsMap := make(map[GroupID][]int)
		seq := reflect.ValueOf(c)
		for i := 0; i < seq.Len(); i++ {
			x := seq.Index(i).Interface()
			subgid, ok := gidkey[x]
			if !ok {
				subgid = gid.Extend(strconv.Itoa(len(subgroups)))
				subgroups = append(subgroups, subgid)
				gidkey[x] = subgid
				rowsMap[subgid] = []int{}
			}
			rowsMap[subgid] = append(rowsMap[subgid], i)
		}

		// Split this group in all columns.
		for _, subgid := range subgroups {
			// Construct this new group.
			rows := rowsMap[subgid]
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

// Ungroup concatenates adjacent Tables in g that share a group parent
// into a Table identified by the parent, undoing the effects of the
// most recent GroupBy operation.
func Ungroup(g Grouping) Grouping {
	groups := g.Tables()
	if len(groups) == 0 || len(groups) == 1 && groups[0] == RootGroupID {
		return g
	}

	out := Grouping(new(Table))
	runGid := groups[0].Parent()
	runTabs := []*Table{}
	for _, gid := range groups {
		if gid.Parent() != runGid {
			// Flush the run.
			out = out.AddTable(runGid, concatRows(runTabs...))

			runGid = gid.Parent()
			runTabs = runTabs[:0]
		}
		runTabs = append(runTabs, g.Table(gid))
	}
	// Flush the last run.
	out = out.AddTable(runGid, concatRows(runTabs...))

	return out
}

// Flatten concatenates all of the groups in g into a single Table.
// This is equivalent to repeatedly Ungrouping g.
func Flatten(g Grouping) *Table {
	groups := g.Tables()
	switch len(groups) {
	case 0:
		return new(Table)

	case 1:
		return g.Table(groups[0])
	}

	tabs := make([]*Table, len(groups))
	for i, gid := range groups {
		tabs[i] = g.Table(gid)
	}

	return concatRows(tabs...)
}

// concatRows concatenates the rows of tabs into a single Table. All
// Tables in tabs must all have the same column set.
func concatRows(tabs ...*Table) *Table {
	// TODO: Consider making this public. It would have to check
	// the columns, and we would probably also want a concatCols.

	switch len(tabs) {
	case 0:
		return new(Table)

	case 1:
		return tabs[0]
	}

	// Construct each column.
	out := new(Table)
	seqs := make([]reflect.Value, len(tabs))
	for _, col := range tabs[0].Columns() {
		seqs = seqs[:0]
		for _, tab := range tabs {
			seqs = append(seqs, reflect.ValueOf(tab.Column(col)))
		}
		nilSeq := reflect.Zero(reflect.TypeOf(seqs[0]))
		newSeq := reflect.Append(nilSeq, seqs...)
		out = out.Add(col, newSeq)
	}

	return out
}
