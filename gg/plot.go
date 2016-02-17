// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/aclements/go-gg/table"
)

// TODO: Split transforms, scalers, and layers into their own packages
// to clean up the name spaces and un-prefix their names?

// Warning is a logger for reporting conditions that don't prevent the
// production of a plot, but may lead to unexpected results.
var Warning = log.New(os.Stderr, "[gg]", log.Lshortfile)

type Plot struct {
	//facet  facet

	env *plotEnv

	scaledData map[scaledDataKey]*scaledData
	aesMap     map[string]map[*scaledData]bool
	marks      []plotMark
}

func NewPlot(data table.Grouping) *Plot {
	p := &Plot{
		env: &plotEnv{
			data:     data,
			bindings: make(map[string]*binding),
		},
		scaledData: make(map[scaledDataKey]*scaledData),
		aesMap:     make(map[string]map[*scaledData]bool),
	}
	return p
}

type plotEnv struct {
	parent   *plotEnv
	data     table.Grouping
	bindings map[string]*binding
}

// A binding combines a set of data values (such as numeric values or
// factors) with a set of scales for mapping them to visual properties
// (such as position or color).
type binding struct {
	isConstant bool
	col        string
	constant   interface{}
	scales     map[table.GroupID]Scaler
}

// SetData sets p's current data table. The caller must not modify
// data in this table after this point.
func (p *Plot) SetData(data table.Grouping) *Plot {
	// TODO: Check that all bindings are valid in the new table?
	p.env.data = data
	return p
}

// Data returns p's current data table.
func (p *Plot) Data() table.Grouping {
	return p.env.data
}

// TODO: How to bind constants?

// Bind is shorthand for p.BindWithScale(prop, col, nil). It binds the
// data in column col to visual property prop using a default scale.
func (p *Plot) Bind(prop, col string) *Plot {
	return p.BindWithScale(prop, col, nil)
}

// BindWithScale binds the data in column col to visual property prop,
// using the specified scale to map between data values and visual
// values.
//
// If scale is nil, BindWithScale will pick a scale. If prop is of the
// form "𝑎_𝑏" and there is an existing binding named "𝑎", this binding
// will inherit 𝑎's scale. This is used to make properties like "y"
// and "y_2" map to the same scale. Otherwise, BindWithScale will
// chose a default scale based on the type of data in column col.
//
// BindWithScale returns p for ease of method chaining.
func (p *Plot) BindWithScale(prop, col string, scale Scaler) *Plot {
	data := p.env.data

	// Check col.
	for _, col2 := range data.Columns() {
		if col == col2 {
			goto haveCol
		}
	}
	panic("unknown column: " + col)
haveCol:

	// Create a binding.
	b := binding{col: col}
	if scale != nil {
		b.scales = map[table.GroupID]Scaler{table.RootGroupID: scale}
	} else {
		if i := strings.Index(prop, "_"); i >= 0 {
			// Find the scale of the base binding, if any.
			scaleName := prop[:i]
			if b2 := p.getBinding(scaleName); b2 != nil {
				// Copy the scale tree from b2.
				b.scales = b2.scales
			}
		}

		if b.scales == nil {
			// Choose the default scale for this data.
			seq := data.Table(data.Tables()[0]).Column(col)
			scale, err := DefaultScale(seq)
			if err != nil {
				panic(err)
			}
			b.scales = map[table.GroupID]Scaler{table.RootGroupID: scale}
		}
	}

	p.env.bindings[prop] = &b
	return p
}

func (p *Plot) unbindAll() *Plot {
	for _, name := range p.bindings() {
		p.env.bindings[name] = nil
	}
	return p
}

// TODO: Make these public. Might want to just return the name, data,
// and scale rather than the internal struct. OTOH, that would make
// bindings() difficult.

func (p *Plot) getBinding(name string) *binding {
	for e := p.env; e != nil; e = e.parent {
		if b, ok := e.bindings[name]; ok {
			return b
		}
	}
	return nil
}

func (p *Plot) mustGetBinding(name string) *binding {
	if b := p.getBinding(name); b != nil {
		return b
	}
	panic("unknown binding: " + name)
}

func (p *Plot) bindings() []string {
	bs := make([]string, 0, len(p.env.bindings))
	have := make(map[string]bool)
	for e := p.env; e != nil; e = e.parent {
		for name, b := range e.bindings {
			if !have[name] {
				have[name] = true
				if b != nil {
					bs = append(bs, name)
				}
			}
		}
	}
	return bs
}

type scaledDataKey struct {
	data table.Grouping
	b    *binding
}

// use marks a binding as in-use by a mark, adds the binding's data to
// the domain of the binding's scale, attaches it to a rendering
// aesthetic, and returns the scaled data.
//
// b may be nil, in which case it simply returns nil. This is
// convenient in combination with getBinding for an optional binding.
//
// TODO: Should aes be an enum?
func (p *Plot) use(aes string, b *binding) *scaledData {
	if b == nil {
		return nil
	}

	sd := p.scaledData[scaledDataKey{p.Data(), b}]
	if sd == nil {
		// Construct the scaledData.
		sd = &scaledData{
			seqs: make(map[table.GroupID]scaledSeq),
		}

		for _, gid := range p.Data().Tables() {
			// Find the scale.
			var scaler Scaler
			for gid := gid; ; gid = gid.Parent() {
				if s, ok := b.scales[gid]; ok {
					scaler = s
					break
				}
				if gid == table.RootGroupID {
					// TODO: This could happen if
					// an operation that creates
					// non-root scales (faceting)
					// and then flattened the data
					// or set new data. Maybe
					// setting new data needs to
					// check that all scales still
					// apply?
					panic("no scale for group " + gid.String())
				}
			}

			var seq table.Slice
			table := p.Data().Table(gid)
			if b.isConstant {
				// Create the sequence.
				len := table.Len()
				sv := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(b.constant)), len, len)
				cv := reflect.ValueOf(b.constant)
				for i := 0; i < len; i++ {
					sv.Index(i).Set(cv)
				}
				seq = sv.Interface()
			} else {
				// Get the data.
				seq = table.MustColumn(b.col)
			}

			// Train the scale.
			scaler.ExpandDomain(seq)

			// Add it to the scaledData.
			sd.seqs[gid] = scaledSeq{seq, scaler}
		}

		p.scaledData[scaledDataKey{p.Data(), b}] = sd
	}

	// Add it to the aesthetic mappings.
	am := p.aesMap[aes]
	if am == nil {
		am = make(map[*scaledData]bool)
		p.aesMap[aes] = am
	}
	am[sd] = true

	return sd
}

// Save saves the current data table and bindings of p to a stack.
// This creates a new scope for declaring bindings: all of the
// existing bindings continue to be available, but new bindings can be
// created in the scope and existing bindings can be shadowed by new
// bindings of the same name.
func (p *Plot) Save() *Plot {
	p.env = &plotEnv{
		parent:   p.env,
		data:     p.env.data,
		bindings: make(map[string]*binding),
	}
	return p
}

// Restore restores the data table and bindings of p from the save
// stack. The exits the current bindings scope.
func (p *Plot) Restore() *Plot {
	if p.env.parent == nil {
		panic("unbalanced Save/Restore")
	}
	p.env = p.env.parent
	// TODO: Clear unwound bindings from p.used.
	return p
}

// TODO: Maybe Plotter should be an interface so types can satisfy it?
// This might be a nice way to add Bindings to a plot.

type Plotter func(p *Plot)

func (p *Plot) Add(o Plotter) *Plot {
	o(p)
	return p
}
