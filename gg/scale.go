// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"fmt"
	"image/color"
	"math"
	"reflect"

	"github.com/aclements/go-moremath/scale"
)

// Continuous -> Interpolatable? Definitely.
//
// Continuous -> Discrete? Can always discretize the input either in
// value order or in index order. In this case the transform (linear,
// log, etc) doesn't matter as long as it's order-preserving.
//
// Discrete -> Interpolatable? Pick evenly spaced values on [0,1].
//
// Discrete -> Discrete? Definitely. Cycle the range if it's not long
// enough. If the input range is a VarNominal, concatenate the
// sequences and use index ordering.
//
// It's not really "continuous", it's more specifically cardinal.

// XXX
//
// A Scaler can be cardinal, discrete, or identity.
//
// A cardinal Scaler has a VarCardinal input domain. If its output
// range is continuous, it maps an interval over the input to an
// interval of the output (possibly through a transformation such as a
// logarithm). If its output range is discrete, the input is
// discretized in value order and it acts like a discrete scale.
//
// XXX The cardinal -> discrete rule means we need to keep all of the
// input data, rather than just its bounds, just in case the range is
// discrete. Maybe it should just be a bucketing rule?
//
// A discrete Scaler has a VarNominal input domain. If the input is
// VarOrdinal, its order is used; otherwise, index order is imposed.
// If the output range is continuous, a discrete Scaler maps its input
// to the centers of equal sub-intervals of [0, 1] and then applies
// the Ranger. If the output range is discrete, the Scaler maps the
// Nth input level to the N%len(range)th output value.
//
// An identity Scaler ignores its input domain and output range and
// uses an identity function for mapping input to output. This is
// useful for specifying aesthetics directly, such as color or size,
// and is especially useful for constant Vars.
//
// XXX Should identity Scalers map numeric types to float64? Maybe it
// should depend on the range type of the ranger?
//
// XXX Arrange documentation as X -> Y?
type Scaler interface {
	// XXX

	ExpandDomain(Var)

	// Ranger sets this Scaler's output range if r is non-nil and
	// returns the previous continuous range. This makes the range
	// continuous and overrides any set discrete range.
	Ranger(r Ranger) Ranger

	// DiscreteRange sets this Scaler's output range if r is
	// non-nil and returns the previous discrete range. r must be
	// a sequence (slice, array, or pointer to array). This makes
	// the range discrete and overrides any set continuous range.
	//
	// XXX This interface makes it annoying to test if a Scaler
	// has a range because you have to check both Ranger and
	// DiscreteRange.
	DiscreteRange(r interface{}) interface{}

	// XXX Should RangeType be implied by the aesthetic?
	//
	// XXX Should this be a method of Ranger instead?
	RangeType() reflect.Type

	// XXX
	//
	// x must be of the same type as the values in the domain Var.
	//
	// XXX Or should this take a slice? Or even a Var? That would
	// also eliminate RangeType(), though then Map would need to
	// know how to make the right type of return slice. Unless we
	// pushed slice mapping all the way to Ranger.
	//
	// XXX We could eliminate ExpandDomain if the caller was
	// required to pass everything to this at once and this did
	// the scale training. That would also make it easy to
	// implement the cardinal -> discrete by value order rule.
	// This would probably also make Map much faster.
	Map(x interface{}) interface{}
}

func DefaultScale(data interface{}) (Scaler, error) {
	// XXX This should probably be based on the Var type.

	typ := reflect.TypeOf(data)
	if typ.Kind() == reflect.Slice {
		typ = typ.Elem()
	}
	if typ.ConvertibleTo(reflect.TypeOf(float64(0))) {
		return NewLinearScaler(), nil
	}
	if typ.Implements(reflect.TypeOf(color.Color(nil))) {
		return NewIdentityScale(), nil
	}
	// TODO: Ordinal scales. Maybe custom default scales.
	return nil, fmt.Errorf("no default scale for data of type %T", data)
}

func NewIdentityScale() Scaler {
	return &identityScale{}
}

type identityScale struct {
	rangeType reflect.Type
}

func (s *identityScale) ExpandDomain(v Var) {
	s.rangeType = reflect.TypeOf(v.Seq()).Elem()
}

func (s *identityScale) RangeType() reflect.Type {
	return s.rangeType
}

func (s *identityScale) Ranger(r Ranger) Ranger                  { return nil }
func (s *identityScale) DiscreteRange(r interface{}) interface{} { return nil }
func (s *identityScale) Map(x interface{}) interface{}           { return x }

// NewLinearScaler returns a continuous linear scale. The domain must
// be a VarCardinal.
//
// XXX If I return a Scaler, I can't have methods for setting fixed
// bounds and such. I don't really want to expose the whole type.
// Maybe a sub-interface for continuous Scalers?
func NewLinearScaler() Scaler {
	return &linearScale{s: scale.Linear{Min: math.NaN(), Max: math.NaN()}}
}

type linearScale struct {
	s scale.Linear
	r Ranger
}

func (s *linearScale) ExpandDomain(v Var) {
	vv, ok := v.(VarCardinal)
	if !ok {
		panic(&TypeError{"gg.linearScale.ExpandDomain", reflect.TypeOf(v), "must be a VarCardinal"})
	}
	if vv.Len() == 0 {
		return
	}
	data := vv.SeqFloat64()
	min, max := s.s.Min, s.s.Max
	for _, v := range data {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		if v < min || math.IsNaN(min) {
			min = v
		}
		if v > max || math.IsNaN(max) {
			max = v
		}
	}
	s.s.Min, s.s.Max = min, max
}

func (s *linearScale) Ranger(r Ranger) Ranger {
	old := s.r
	if r != nil {
		s.r = r
	}
	return old
}

func (s *linearScale) DiscreteRange(r interface{}) interface{} {
	panic("not implemented")
}

func (s *linearScale) RangeType() reflect.Type {
	// XXX Discrete ranges
	return s.r.RangeType()
}

func (s *linearScale) Map(x interface{}) interface{} {
	ls := s.s
	if math.IsNaN(ls.Min) {
		ls.Min, ls.Max = -1, 1
	}
	f64 := reflect.TypeOf(float64(0))
	v := reflect.ValueOf(x).Convert(f64).Float()
	return s.r.Map(ls.Map(v))
}

type Ranger interface {
	Map(x float64) (y interface{})
	Unmap(y interface{}) (x float64)
	RangeType() reflect.Type
}

func NewFloatRanger(lo, hi float64) Ranger {
	return &floatRanger{lo, hi - lo}
}

type floatRanger struct {
	lo, w float64
}

func (r *floatRanger) Map(x float64) interface{} {
	return x*r.w + r.lo
}

func (r *floatRanger) Unmap(y interface{}) float64 {
	return (y.(float64) - r.lo) / r.w
}

func (r *floatRanger) RangeType() reflect.Type {
	return reflect.TypeOf(float64(0))
}
