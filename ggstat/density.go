// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ggstat

import (
	"math"

	"github.com/aclements/go-gg/table"
	"github.com/aclements/go-moremath/stats"
	"github.com/aclements/go-moremath/vec"
)

// TODO: I'm experimenting with different representations for stats.
// Make them consistent at some point.

// Density constructs a probability density estimate from a set of
// samples using kernel density estimation.
//
// X is the only required field. All other fields have reasonable
// default zero values.
//
// The result of Density has two columns:
//
// - Column X is the points at which the density estimate is sampled.
//
// - If Cumulative is false, column "probability density" is the
//   density estimate. If Cumulative is true, column "cumulative
//   density" is the cumulative density estimate.
type Density struct {
	// X is the name of the column to use for samples.
	X string

	// W is the optional name of the column to use for sample
	// weights. It may be "" to uniformly weight samples.
	W string

	// N is the number of points to sample the KDE at. If N is 0,
	// a reasonable default is used.
	//
	// TODO: This is particularly sensitive to the scale
	// transform.
	N int

	// Trim controls the domain of the returned density estimate.
	// It will be [min(X)-Trim*Bandwidth, max(X)+Trim*Bandwidth].
	Trim float64

	// Cumulative indicates that Density should produce a
	// cumulative density estimate rather than a probability
	// density estimate.
	Cumulative bool

	// Kernel is the kernel to use for the KDE.
	Kernel stats.KDEKernel

	// Bandwidth is the bandwidth to use for the KDE.
	//
	// If this is zero, the bandwidth is computed from the data
	// using a default bandwidth estimator (currently
	// stats.BandwidthScott).
	Bandwidth float64

	// BoundaryMethod is the boundary correction method to use for
	// the KDE. The default value is BoundaryReflect; however, the
	// default bounds are effectively +/-inf, which is equivalent
	// to performing no boundary correction.
	BoundaryMethod stats.KDEBoundaryMethod

	// [BoundaryMin, BoundaryMax) specify a bounded support for
	// the KDE. If both are 0 (their default values), they are
	// treated as +/-inf.
	//
	// To specify a half-bounded support, set Min to math.Inf(-1)
	// or Max to math.Inf(1).
	BoundaryMin float64
	BoundaryMax float64
}

func (d Density) F(g table.Grouping) table.Grouping {
	kde := stats.KDE{
		Kernel:         d.Kernel,
		Bandwidth:      d.Bandwidth,
		BoundaryMethod: d.BoundaryMethod,
		BoundaryMin:    d.BoundaryMin,
		BoundaryMax:    d.BoundaryMax,
	}
	if d.N == 0 {
		d.N = 200
	}
	resp := "probability density"
	if d.Cumulative {
		resp = "cumulative density"
	}

	return table.MapTables(func(_ table.GroupID, t *table.Table) *table.Table {
		// TODO: Coerce to []float64?
		kde.Sample = stats.Sample{Xs: t.MustColumn(d.X).([]float64)}
		if d.W != "" {
			kde.Sample.Weights = t.MustColumn(d.W).([]float64)
		}

		min, max := kde.Sample.Bounds()
		if math.IsNaN(min) {
			return new(table.Table).Add(d.X, []float64{}).Add(resp, []float64{})
		}

		if d.Bandwidth == 0 {
			kde.Bandwidth = stats.BandwidthScott(kde.Sample)
		}

		min, max = min-d.Trim*kde.Bandwidth, max+d.Trim*kde.Bandwidth
		ss := vec.Linspace(min, max, d.N)
		nt := new(table.Table).Add(d.X, ss)
		if d.Cumulative {
			nt = nt.Add(resp, vec.Map(kde.CDF, ss))
		} else {
			nt = nt.Add(resp, vec.Map(kde.PDF, ss))
		}
		return nt
	}, g)
}
