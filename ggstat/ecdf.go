// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ggstat

import (
	"github.com/aclements/go-gg/table"
	"github.com/aclements/go-moremath/vec"
)

// ECDF constructs an empirical CDF. xcol names the column in g for
// data points. wcol may be "", in which case all points are equally
// weighted, or name a column in g for weights. ECDF returns a
// table.Grouped with the same groups as g and with two columns:
//
// - Column xcol corresponds to the input data points.
//
// - Column "cumulative density" corresponds to the cumulative density
// at that data point.
//
// ECDF adds two more points 5% before the first point and 5% after
// the last point to make the 0 and 1 levels clear.
//
// TODO: These columns names make automatic labeling work out, but are
// rather verbose. Should we do this a different way? If layers used
// the 1st and 2nd columns by default for X and Y, you may never have
// to mention the column names in most situations.
//
// TODO: Since this operates on a table.Grouping, it can't account for
// scale transforms. Should it operate on a Plot instead?
func ECDF(g table.Grouping, xcol, wcol string) table.Grouping {
	g = table.SortBy(g, xcol)
	return table.MapTables(func(_ table.GroupID, t *table.Table) *table.Table {
		// Get input columns.
		// TODO: Coerce to []float64
		xs := t.MustColumn(xcol).([]float64)
		ws := ([]float64)(nil)
		if wcol != "" {
			ws = t.MustColumn(wcol).([]float64)
		}

		// Ignore empty tables.
		if len(xs) == 0 {
			return new(table.Table).Add(xcol, []float64{}).Add("cumulative density", []float64{})
		}

		// Create output columns.
		xo, yo := make([]float64, 1), make([]float64, 1)

		// Compute total weight.
		var total float64
		if ws == nil {
			total = float64(t.Len())
		} else {
			total = vec.Sum(ws)
		}

		// Create ECDF.
		cum := 0.0
		for i := 0; i < len(xs); {
			j := i
			for j < len(xs) && xs[i] == xs[j] {
				if ws == nil {
					cum += 1
				} else {
					cum += ws[j]
				}
				j++
			}

			xo = append(xo, xs[i])
			yo = append(yo, cum/total)

			i = j
		}

		// Extend to the left and right a bit.
		//
		// TODO: If ECDF was a struct, the margin could be
		// controllable.
		const margin = 0.05
		span := xs[len(xs)-1] - xs[0]
		xo[0], yo[0] = xs[0]-(margin*span), 0
		xo, yo = append(xo, xs[len(xs)-1]+(margin*span)), append(yo, 1)

		return new(table.Table).Add(xcol, xo).Add("cumulative density", yo)
	}, g)
}
