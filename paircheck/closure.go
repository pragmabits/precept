package paircheck

import (
	"slices"

	"golang.org/x/tools/go/ssa"
)

// closure is the anonymous function a defer statement calls, seen from the
// search of the function that defers it: its free variables stand for the
// values bound to them there.
type closure struct {
	search   search
	function *ssa.Function
	bindings []ssa.Value
}

// closureOf is the anonymous function deferred calls, if it calls one.
func closureOf(current search, deferred *ssa.Defer) (closure, bool) {
	switch callee := deferred.Call.Value.(type) {
	case *ssa.MakeClosure:
		function, ok := callee.Fn.(*ssa.Function)
		if ok && function.Parent() != nil {
			return closure{search: current, function: function, bindings: callee.Bindings}, true
		}
	case *ssa.Function:
		if callee.Parent() != nil {
			return closure{search: current, function: callee}, true
		}
	}
	return closure{}, false
}

// discharges reports whether running the closure at the exit discharges the
// obligation, as the protocol's coverage asks.
func (c closure) discharges() bool {
	switch c.search.binding.protocol.coverage {
	case CoverageNone:
		return false
	case CoverageEveryPath:
		return c.coversEveryPath()
	}
	return c.coversAnyPath()
}

func (c closure) coversAnyPath() bool {
	for _, block := range c.function.Blocks {
		if slices.ContainsFunc(block.Instrs, c.satisfies) {
			return true
		}
	}
	return false
}

// coversEveryPath reports whether no path from the closure's entry leaves it
// without calling a satisfier.
func (c closure) coversEveryPath() bool {
	if len(c.function.Blocks) == 0 {
		return false
	}
	pending := []*ssa.BasicBlock{c.function.Blocks[0]}
	visited := make(map[*ssa.BasicBlock]bool)
	for len(pending) > 0 {
		last := len(pending) - 1
		block := pending[last]
		pending = pending[:last]
		if visited[block] {
			continue
		}
		visited[block] = true
		covered, leaves := c.scan(block)
		if leaves {
			return false
		}
		if !covered {
			pending = append(pending, block.Succs...)
		}
	}
	return true
}

// scan reports whether block calls a satisfier before anything else ends the
// path, or leaves the closure first.
func (c closure) scan(block *ssa.BasicBlock) (covered, leaves bool) {
	for _, instruction := range block.Instrs {
		if c.satisfies(instruction) {
			return true, false
		}
		switch instruction.(type) {
		case *ssa.Return, *ssa.Panic:
			return false, true
		}
	}
	return false, false
}

func (c closure) satisfies(instruction ssa.Instruction) bool {
	call, ok := instruction.(*ssa.Call)
	if !ok {
		return false
	}
	current := c.search.binding
	index, ok := current.satisfierOf(call.Common())
	if !ok {
		return false
	}
	value := current.satisfierSlots[index].valueAt(call)
	if value == nil || c.search.opened.value == nil {
		return true
	}
	inside := c.translate(pathOf(value))
	return current.satisfied(index, inside, pathOf(c.search.opened.value)) != sameNo
}

// decides reports whether the closure calls a satisfier on every path where
// the error its function returns is not a failure: result, the variable that
// error is read from, is what a check in the closure reads through a free
// variable. A path that finds it not nil needs no call, and a path that ends in
// a panic is abandoned.
func (c closure) decides(result ssa.Value) bool {
	if len(c.function.Blocks) == 0 {
		return false
	}
	captured := c.capturing(result)
	type point struct {
		block  *ssa.BasicBlock
		failed bool
	}
	pending := []point{{block: c.function.Blocks[0]}}
	visited := make(map[point]bool)
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if visited[current] {
			continue
		}
		visited[current] = true
		covered, leaves := c.scan(current.block)
		switch {
		case covered, current.failed && leaves:
			continue
		case leaves:
			if _, panics := current.block.Instrs[len(current.block.Instrs)-1].(*ssa.Panic); !panics {
				return false
			}
			continue
		}
		for position, successor := range current.block.Succs {
			pending = append(pending, point{
				block:  successor,
				failed: c.failsAlong(current.block, position, captured, current.failed),
			})
		}
	}
	return true
}

// capturing is the free variable of the closure bound to variable, or nil.
func (c closure) capturing(variable ssa.Value) ssa.Value {
	if variable == nil {
		return nil
	}
	index := slices.Index(c.bindings, variable)
	if index < 0 || index >= len(c.function.FreeVars) {
		return nil
	}
	return c.function.FreeVars[index]
}

// failsAlong is whether the error the function returns is known to be a failure
// on the successor at position of block: a check of it there says so, or says it
// is nil; any other branch keeps what was known.
func (c closure) failsAlong(
	block *ssa.BasicBlock,
	position int,
	captured ssa.Value,
	failed bool,
) bool {
	branch, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If)
	if !ok || captured == nil {
		return failed
	}
	found, ok := c.search.proofOf(branch.Cond)
	if !ok {
		return failed
	}
	if variable, read := variableRead(found.checked); !read || variable != captured {
		return failed
	}
	if position == found.failing {
		return true
	}
	return failed && !found.nilOtherwise
}

// translate is reached as seen from the function that defers the closure: a
// free variable becomes the value bound to it.
func (c closure) translate(reached path) path {
	free, ok := reached.root.(*ssa.FreeVar)
	if !ok {
		return reached
	}
	index := slices.Index(c.function.FreeVars, free)
	if index < 0 || index >= len(c.bindings) {
		return reached
	}
	outer := pathOf(c.bindings[index])
	outer.steps += reached.steps
	outer.loads = outer.loads || reached.loads
	return outer
}
