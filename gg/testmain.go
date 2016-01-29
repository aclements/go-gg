// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"math"
	"os"

	"github.com/aclements/go-gg/gg"
	"github.com/aclements/go-moremath/vec"
)

func main() {
	xs := vec.Linspace(-10, 10, 100)
	ys := vec.Map(math.Sin, xs)

	plot := gg.NewPlot()
	plot.Bind("x", xs).Bind("y", ys).Add(gg.LayerPaths())
	plot.WriteSVG(os.Stdout, 200, 100)
}
