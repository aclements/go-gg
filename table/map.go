// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

// MapTables applies f to each Table in g and returns a new Grouping
// with the same group structure as g, but with the Tables returned by
// f.
func MapTables(f func(gid GroupID, table *Table) *Table, g Grouping) Grouping {
	var out GroupingBuilder
	for _, gid := range g.Tables() {
		out.Add(gid, f(gid, g.Table(gid)))
	}
	return out.Done()
}
