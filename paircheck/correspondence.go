package paircheck

import (
	"go/types"
	"slices"
)

// correspondence pairs the type parameters of one function with those of
// another, each with at most one: the type parameters of two functions are
// different types to go/types, so a rule that names a slot on each side is what
// says they stand for one another.
type correspondence struct {
	forward  map[*types.TypeParam]*types.TypeParam
	backward map[*types.TypeParam]*types.TypeParam
}

// correspond reports whether first and second have the same shape, with each
// type parameter of one matched to exactly one of the other.
func correspond(first, second types.Type) bool {
	return correspondence{
		forward:  make(map[*types.TypeParam]*types.TypeParam),
		backward: make(map[*types.TypeParam]*types.TypeParam),
	}.unify(first, second)
}

// unify reports whether first and second have the same shape, pairing each
// type parameter it meets on one side with the one at the same place on the
// other.
func (c correspondence) unify(first, second types.Type) bool {
	first, second = types.Unalias(first), types.Unalias(second)
	if parameter, ok := first.(*types.TypeParam); ok {
		other, isParameter := second.(*types.TypeParam)
		return isParameter && c.pair(parameter, other)
	}
	firstShape, firstParts, composite := decompose(first)
	if !composite {
		return types.Identical(first, second)
	}
	secondShape, secondParts, _ := decompose(second)
	if firstShape != secondShape || len(firstParts) != len(secondParts) {
		return false
	}
	for index := range firstParts {
		if !c.unify(firstParts[index], secondParts[index]) {
			return false
		}
	}
	return true
}

// pair records that first stands for second, and reports whether that agrees
// with every pair recorded so far.
func (c correspondence) pair(first, second *types.TypeParam) bool {
	if other, ok := c.forward[first]; ok {
		return other == second
	}
	if other, ok := c.backward[second]; ok {
		return other == first
	}
	c.forward[first] = second
	c.backward[second] = first
	return true
}

// construction is the kind of composite type a shape describes.
type construction int

const (
	constructionPointer construction = iota + 1
	constructionSlice
	constructionArray
	constructionMap
	constructionChannel
	constructionNamed
	constructionSignature
)

// shape is a composite type apart from the types it is built from: its
// construction, and what else two of them must share to be the same.
type shape struct {
	construction construction
	origin       types.Object
	length       int64
	direction    types.ChanDir
	parameters   int
	variadic     bool
}

// decompose is the shape of a composite type and the types it is built from,
// or false for any other type.
func decompose(valueType types.Type) (shape, []types.Type, bool) {
	switch composite := valueType.(type) {
	case *types.Pointer:
		return shape{construction: constructionPointer}, []types.Type{composite.Elem()}, true
	case *types.Slice:
		return shape{construction: constructionSlice}, []types.Type{composite.Elem()}, true
	case *types.Array:
		built := shape{construction: constructionArray, length: composite.Len()}
		return built, []types.Type{composite.Elem()}, true
	case *types.Map:
		built := shape{construction: constructionMap}
		return built, []types.Type{composite.Key(), composite.Elem()}, true
	case *types.Chan:
		built := shape{construction: constructionChannel, direction: composite.Dir()}
		return built, []types.Type{composite.Elem()}, true
	case *types.Named:
		built := shape{construction: constructionNamed, origin: composite.Origin().Obj()}
		return built, typeArguments(composite), true
	case *types.Signature:
		built := shape{
			construction: constructionSignature,
			parameters:   composite.Params().Len(),
			variadic:     composite.Variadic(),
		}
		return built, signatureTypes(composite), true
	}
	return shape{}, nil, false
}

func typeArguments(named *types.Named) []types.Type {
	arguments := named.TypeArgs()
	parts := make([]types.Type, 0, arguments.Len())
	for index := range arguments.Len() {
		parts = append(parts, arguments.At(index))
	}
	return parts
}

// signatureTypes lists the types of a signature's parameters, then of its
// results.
func signatureTypes(signature *types.Signature) []types.Type {
	var parts []types.Type
	for variable := range signature.Params().Variables() {
		parts = append(parts, variable.Type())
	}
	for variable := range signature.Results().Variables() {
		parts = append(parts, variable.Type())
	}
	return parts
}

// mentionsTypeParameter reports whether valueType is a type parameter or is
// built from one.
func mentionsTypeParameter(valueType types.Type) bool {
	valueType = types.Unalias(valueType)
	if _, ok := valueType.(*types.TypeParam); ok {
		return true
	}
	_, parts, _ := decompose(valueType)
	return slices.ContainsFunc(parts, mentionsTypeParameter)
}
