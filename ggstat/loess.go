// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ggstat

import (
	"math"

	"github.com/aclements/go-gg/generic"
	"github.com/aclements/go-gg/table"
	"github.com/aclements/go-moremath/fit"
	"github.com/aclements/go-moremath/stats"
	"github.com/aclements/go-moremath/vec"
)

// LOESS constructs a locally-weighted least squares polynomial
// regression for the data (X, Y).
//
// X and Y are required. All other fields have reasonable default zero
// values.
//
// The result of LOESS has two columns:
//
// - Column X is the points at which the LOESS function is sampled.
//
// - Column "LOESS" is the result of the LEOSS function.
//
// TODO: Confidence internals/bootstrap distributions?
type LOESS struct {
	// X and Y are the names of the columns to use for X and Y
	// values of data points, respectively.
	X, Y string

	// N is the number of points to sample the regression at. If N
	// is 0, a reasonable default is used.
	N int

	// Widen sets the domain of the returned LOESS sample points
	// to Widen times the span of the data. If Widen is 0, it is
	// treated as 1.1 (that is, widen the domain by 10%, or 5% on
	// the left and 5% on the right).
	Widen float64

	// SplitGroups indicates that each group in the table should
	// have separate bounds based on the data in that group alone.
	// The default, false, indicates that the bounds should be
	// based on all of the data in the table combined. This makes
	// it possible to stack LOESS fits and easier to compare them
	// across groups.
	SplitGroups bool

	// Degree specifies the degree of the local fit function. If
	// it is 0, it is treated as 2.
	Degree int

	// Span controls the smoothness of the fit. If it is 0, it is
	// treated as 0.5. The span must be between 0 and 1, where
	// smaller values fit the data more tightly.
	Span float64
}

func (s LOESS) F(g table.Grouping) table.Grouping {
	if s.Degree <= 0 {
		s.Degree = 2
	}
	if s.Span <= 0 {
		s.Span = 0.5
	}

	evals := evalPoints(g, s.X, s.N, s.Widen, s.SplitGroups)

	var xs, ys []float64
	return table.MapTables(func(gid table.GroupID, t *table.Table) *table.Table {
		// TODO: We potentially convert each X column twice,
		// since evalPoints also has to convert them.
		generic.ConvertSlice(&xs, t.MustColumn(s.X))
		generic.ConvertSlice(&ys, t.MustColumn(s.Y))
		eval := evals[gid]

		loess := fit.LOESS(xs, ys, s.Degree, s.Span)
		return new(table.Table).Add(s.X, eval).Add("LOESS", vec.Map(loess, eval))
	}, g)
}

func evalPoints(g table.Grouping, x string, n int, widen float64, splitGroups bool) map[table.GroupID][]float64 {
	var xs []float64
	res := map[table.GroupID][]float64{}

	if n <= 0 {
		n = 200
	}
	if widen <= 0 {
		widen = 1.1
	}

	if !splitGroups {
		// Compute combined bounds.
		min, max := math.NaN(), math.NaN()
		for _, gid := range g.Tables() {
			t := g.Table(gid)
			generic.ConvertSlice(&xs, t.MustColumn(x))
			xmin, xmax := stats.Bounds(xs)
			if xmin < min || math.IsNaN(min) {
				min = xmin
			}
			if xmax > max || math.IsNaN(max) {
				max = xmax
			}
		}

		// Widen bounds.
		span := max - min
		min, max = min-span*(widen-1)/2, max+span*(widen-1)/2

		// Create evaluation points. Careful if there's no data.
		var eval []float64
		if !math.IsNaN(min) {
			eval = vec.Linspace(min, max, n)
		}
		for _, gid := range g.Tables() {
			res[gid] = eval
		}
		return res
	}

	for _, gid := range g.Tables() {
		t := g.Table(gid)

		// Compute bounds.
		generic.ConvertSlice(&xs, t.MustColumn(x))
		min, max := stats.Bounds(xs)

		// Widen bounds.
		span := max - min
		min, max = min-span*(widen-1)/2, max+span*(widen-1)/2

		// Create evaluation points. Careful if there's no data.
		var eval []float64
		if !math.IsNaN(min) {
			eval = vec.Linspace(min, max, n)
		}
		res[gid] = eval
	}
	return res
}
