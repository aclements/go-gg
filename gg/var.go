// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"fmt"
	"reflect"

	"github.com/aclements/go-gg/generic"
)

// TODO: Use the term "categorical" instead of "nominal"?

// TODO: Use a named type to document when an interface{} must be a
// sequence? How do I distinguish when pointer to array is or is not
// allowed?

// TODO: Build type-optimized variations of these types so we only
// type dispatch on construction in most cases. We might want to
// automate this process, at least partially. OTOH, right now we
// barely use the methods provided by these and just grab the sequence
// and operate on it directly, so this may be wasted effort.

// TODO: This can't express levels with no representative values. For
// example, if you have a VarOrdinal of month values and you bin them,
// there's no way to directly get the binning to produce a zero-size
// bin. One possible solution is to make this part of the binning
// operation rather than the data by optionally supplying a set of
// bins.

// Var is a variable in a data set. It is a sequence of values, where
// values at the same index in different variables correspond to an
// entry in the data set. Vars form a hierarchy: Var is the most basic
// and its elements support no operations; VarNominal variables add
// equality comparison of values; VarOrdinal variables add ordering of
// values; and VarCardinal variables add distance between values.
type Var interface {
	isVar()

	// Seq returns the value of this variable as a slice or array.
	Seq() interface{}

	// Len returns the length of the sequence returned by Seq.
	Len() int
}

type varBase struct {
	// value is a slice or array.
	value interface{}
}

// NewVar returns a Var backed by seq. seq must be a sequence type
// (slice, array, or pointer to array).
func NewVar(seq interface{}) Var {
	rv := sequence("gg.NewVar", seq)
	return varBase{rv.Interface()}
}

func (v varBase) isVar()           {}
func (v varBase) Seq() interface{} { return v.value }
func (v varBase) Len() int         { return reflect.ValueOf(v.value).Len() }

// VarNominal is a sequence of values that support equality. It
// extends Var with equality. In addition to the requirements of Var,
// the values in the sequence returned by Seq must be comparable (as
// defined by the Go specification).
type VarNominal interface {
	Var

	// Equal returns whether the i'th and j'th elements in this
	// variable are equal.
	Equal(i, j int) bool
}

type varNominal struct {
	varBase
}

// NewVarNominal returns a VarNominal backed by seq. seq must be a
// sequence type (slice, array, or pointer to array) and its elements
// must be comparable (as defined by the Go specification); otherwise,
// NewVarNominal will panic with a *TypeError.
func NewVarNominal(seq interface{}) VarNominal {
	rv := sequence("gg.NewVarNominal", seq)
	if !rv.Type().Elem().Comparable() {
		panic(&TypeError{"gg.NewVarNominal", rv.Type(), "elements are not comparable"})
	}
	return varNominal{varBase{rv.Interface()}}
}

func (v varNominal) Equal(i, j int) bool {
	// Common types.
	switch vv := v.value.(type) {
	case []int:
		return vv[i] == vv[j]
	case []uint:
		return vv[i] == vv[j]
	case []float64:
		return vv[i] == vv[j]
	case []string:
		return vv[i] == vv[j]
	}

	// Anything else.
	rv := reflect.ValueOf(v.value)
	return rv.Index(i).Interface() == rv.Index(j).Interface()
}

// VarOrdinal is a sequence of values that support equality and
// ordering. It extends VarNominal with ordering. In addition to the
// requirements of VarNominal, either 1) the values in the sequence
// returned by Seq must be orderable or 2) Levels() must return
// non-nil.
type VarOrdinal interface {
	VarNominal

	// Order returns the order of values at indexes i and j: -1 if
	// element i < element j, 0 if element i == element j, or 1 if
	// element i > element j. If Levels() is non-nil, the ordering
	// is determined by Levels; otherwise, it is the natural Go
	// ordering.
	Order(i, j int) int

	// Levels returns the ordering of values in this variable. If
	// Levels() is nil, values should be ordered by their natural
	// Go ordering.
	//
	// If Levels is non-nil, the order of Seq()[i] and Seq()[j]
	// obeys the order of Levels()[i] and Levels()[j]. Levels()[i]
	// == Levels()[j] if and only if Seq()[i] == Seq()[j].
	Levels() []int
}

type varOrdinal struct {
	varNominal
	levels []int
}

// NewVarOrdinal returns an VarOrdinal backed by seq. seq must be a
// sequence type (slice, array, or pointer to array), its elements
// must be comparable and, if levels is nil, orderable (as defined by
// the Go specification); otherwise NewVarOrdinal will panic with a
// *TypeError. If levels is non-nil, NewVarOrdinal will panic if
// len(seq) != len(levels) and the caller must ensure that levels[i]
// == levels[j] if and only if seq[i] == seq[j].
func NewVarOrdinal(seq interface{}, levels []int) VarOrdinal {
	rv := sequence("gg.NewVarOrdinal", seq)
	et := rv.Type().Elem()
	if !et.Comparable() {
		panic(&TypeError{"gg.NewVarOrdinal", rv.Type(), "elements are not comparable"})
	}
	if levels == nil && !generic.CanOrderR(et.Kind()) {
		panic(&TypeError{"gg.NewVarOrdinal", rv.Type(), "elements are not orderable"})
	}
	if levels != nil && rv.Len() != len(levels) {
		panic(fmt.Sprintf("lengths of sequence and levels must match; %d != %d", rv.Len(), len(levels)))
	}
	return varOrdinal{varNominal{varBase{rv.Interface()}}, levels}
}

func (v varOrdinal) Order(i, j int) int {
	if v.levels != nil {
		if v.levels[i] < v.levels[j] {
			return -1
		} else if v.levels[i] > v.levels[j] {
			return 1
		}
		return 0
	}
	// Use natural order.
	return sliceOrder(v.value, i, j)
}

func (v varOrdinal) Levels() []int {
	return v.levels
}

func sliceOrder(v interface{}, i, j int) int {
	// Common types.
	switch vv := v.(type) {
	case []int:
		if vv[i] < vv[j] {
			return -1
		} else if vv[i] > vv[j] {
			return 1
		}
		return 0
	case []uint:
		if vv[i] < vv[j] {
			return -1
		} else if vv[i] > vv[j] {
			return 1
		}
		return 0
	case []float64:
		if vv[i] < vv[j] {
			return -1
		} else if vv[i] > vv[j] {
			return 1
		}
		return 0
	case []string:
		if vv[i] < vv[j] {
			return -1
		} else if vv[i] > vv[j] {
			return 1
		}
		return 0
	}

	// Anything else.
	rv := reflect.ValueOf(v)
	return generic.OrderR(rv.Index(i), rv.Index(j))
}

// VarCardinal is a sequence of values that support equality,
// ordering, and (one dimensional) distance. Values must be an array
// or slice of integer or floating-point values (specifically, their
// type must be convertible to float64). Ordering is always the
// natural Go order (in contrast with VarOrdinal).
//
// Note that this is somewhat different from "continuous". For one,
// the data may be integral, in which case it is technically discrete,
// though distance still returns a continuous floating point value.
// More importantly, continuous data may be continuous in more than
// one dimension. VarCardinal specifically applies to data that is
// continuous in 𝐑¹.
type VarCardinal interface {
	VarOrdinal

	// Distance returns the distance between the i'th and j'th
	// elements. This is "one dimensional" distance; that is,
	// Distance(i,j) + Distance(j,k) == Distance(i,k) (modulo
	// floating-point rounding errors).
	Distance(i, j int) float64

	// Seq returns the value of this variable as a float64 slice.
	SeqFloat64() []float64
}

type varCardinal struct {
	varNominal
}

// NewVarCardinal returns a VarCardinal backed by seq. seq must be a
// sequence type (slice, array, or pointer to array) and its elements
// must be numeric (integer or floating point); otherwise,
// NewVarCardinal will panic with a *TypeError.
func NewVarCardinal(seq interface{}) VarCardinal {
	rv := sequence("gg.NewVarCardinal", seq)
	ek := rv.Type().Elem().Kind()
	if !canCardinal[ek] {
		panic(&TypeError{"gg.NewVarCardinal", rv.Type(), "elements are not numeric"})
	}
	return varCardinal{varNominal{varBase{rv.Interface()}}}
}

func (v varCardinal) Order(i, j int) int {
	return sliceOrder(v.value, i, j)
}

func (v varCardinal) Levels() []int {
	return nil
}

func (v varCardinal) Distance(i, j int) float64 {
	intDistance := func(x, y int64) float64 {
		// Watch out for subtraction overflow and floating
		// point precision loss.
		if (x >= 0) == (y >= 0) {
			// x and y are on the same side of 0. If their
			// absolute values are large, doing floating
			// point conversion first may lose too much
			// precision, but integer subtraction is safe.
			return float64(y - x)
		} else {
			// x and y are on different sides of 0, so
			// integer subtraction may not be safe, but
			// any precision lost by floating point
			// conversion is okay.
			return float64(y) - float64(x)
		}
	}
	uintDistance := func(x, y uint64) float64 {
		if y >= x {
			return float64(y - x)
		} else {
			return -float64(y - x)
		}
	}

	// Common types.
	switch vv := v.value.(type) {
	case []float64:
		return vv[j] - vv[i]
	case []int:
		return intDistance(int64(vv[i]), int64(vv[j]))
	case []uint:
		return uintDistance(uint64(vv[i]), uint64(vv[j]))
	}

	// Anything else.
	rv := reflect.ValueOf(v.value)
	vi, vj := rv.Index(i), rv.Index(j)
	switch vi.Kind() {
	case reflect.Float32, reflect.Float64:
		return vj.Float() - vi.Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intDistance(vi.Int(), vj.Int())
	case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return uintDistance(vi.Uint(), vj.Uint())
	}

	panic("elements are not numeric")
}

func (v varCardinal) SeqFloat64() []float64 {
	// []float64
	if vv, ok := v.value.([]float64); ok {
		return vv
	}

	// [...]float64
	rv := reflect.ValueOf(v.value)
	if rv.Kind() == reflect.Array && rv.CanAddr() {
		vv := rv.Slice(0, rv.Len()).Interface()
		if vv, ok := vv.([]float64); ok {
			return vv
		}
	}

	// Everything else needs a copy conversion.
	out := make([]float64, rv.Len())
	f64 := reflect.TypeOf(float64(0))
	for i := range out {
		out[i] = rv.Index(i).Convert(f64).Float()
	}
	return out
}

var canCardinal = map[reflect.Kind]bool{
	reflect.Float32: true,
	reflect.Float64: true,
	reflect.Int:     true,
	reflect.Int8:    true,
	reflect.Int16:   true,
	reflect.Int32:   true,
	reflect.Int64:   true,
	reflect.Uint:    true,
	reflect.Uintptr: true,
	reflect.Uint8:   true,
	reflect.Uint16:  true,
	reflect.Uint32:  true,
	reflect.Uint64:  true,
}

// AutoVar returns a Var backed by data. The subtype of Var depends on
// the type of data. If data is already a Var, it is simply returned.
// If data is a boolean, numeric (including complex), string, or
// struct value, AutoVar promotes it to a single element slice of that
// value. Otherwise, data must already be a sequence (slice, array, or
// pointer to array). AutoVar returns the most specific of Var,
// VarNominal, VarOrdinal, or VarCardinal supported by the sequence's
// elements. In any other case, AutoVar panics with a *TypeError.
func AutoVar(data interface{}) Var {
	// This could be implemented entirely in terms of public APIs,
	// but since we're already picking apart the types, we use the
	// private APIs to save doing it again.

	switch data := data.(type) {
	case Var:
		return data

	case []float64, []int, []uint:
		return varCardinal{varNominal{varBase{data}}}

	case []string:
		return varOrdinal{varNominal{varBase{data}}, nil}
	}

retry:
	rt := reflect.TypeOf(data)
	switch rt.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128,
		reflect.String,
		reflect.Struct:
		// Promote constant values to single-valued slices.
		s := reflect.MakeSlice(reflect.SliceOf(rt), 1, 1)
		s.Index(0).Set(reflect.ValueOf(data))
		data = s.Interface()
		goto retry

	case reflect.Ptr:
		et := rt.Elem()
		if et.Kind() != reflect.Array {
			break
		}
		fallthrough

	case reflect.Array, reflect.Slice:
		et := rt.Elem()
		etk := et.Kind()
		switch {
		case canCardinal[etk]:
			return varCardinal{varNominal{varBase{data}}}
		case generic.CanOrderR(etk):
			return varOrdinal{varNominal{varBase{data}}, nil}
		case et.Comparable():
			return varNominal{varBase{data}}
		default:
			return varBase{data}
		}
	}

	panic(&TypeError{"gg.AutoVar", reflect.TypeOf(data), "not a Var, constant, or sequence type"})
}

// sequence returns the reflect.Value of x that is safe to Index. x
// must be an array, slice, or pointer to an array. If x is a pointer
// to an array, sequence returned the dereferenced pointer. Otherwise,
// sequence panics with a *TypeError.
func sequence(method string, x interface{}) reflect.Value {
	rv := reflect.ValueOf(x)

	switch rv.Kind() {
	case reflect.Ptr:
		if elem := rv.Elem(); elem.Kind() == reflect.Array {
			return elem
		}

	case reflect.Array, reflect.Slice:
		return rv
	}

	panic(&TypeError{method, rv.Type(), "not a sequence type"})
}
