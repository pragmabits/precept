package paircheck

import (
	"slices"

	"golang.org/x/tools/go/ssa"
)

// saturated is the count from which obligations on one value are no longer
// told apart: two or more. It keeps the search finite through loops.
const saturated = 2

type event int

const (
	eventNone event = iota
	eventTrigger
	eventSatisfier
	// eventUndeferred is a satisfier called on the path under require-defer,
	// where only a deferred one discharges.
	eventUndeferred
	eventDeferred
	// eventExit returns from the function, running its deferred calls.
	eventExit
	// eventAbandon ends the path without a return: a panic, or a call that
	// does not return, which SSA follows with a panic of its own. The path is
	// abandoned, and no obligation is asked of it.
	eventAbandon
	// eventTransfer takes the value, and its obligation, out of the function.
	eventTransfer
)

// leak is how a path leaves an obligation open, if it does.
type leak int

const (
	leakNone leak = iota
	// leakAtExit reaches a return with the obligation open.
	leakAtExit
	// leakBeforeDefer calls something, under defer-first, while no deferred
	// satisfier covers the obligation: a panic there would leave it open.
	leakBeforeDefer
	// leakOnSuccess reaches, under on-success, a return that hands back no
	// failure with no satisfier called on the path.
	leakOnSuccess
)

// obligation is what a trigger call opens on a value: the variables the value
// was kept in, and the error the call returned, which the obligation exists
// only without.
type obligation struct {
	call    *ssa.Call
	index   int
	value   ssa.Value
	homes   []ssa.Value
	failure failure
}

// search looks for a path from a trigger call to an exit of its function along
// which the obligation is still open.
type search struct {
	binding binding
	opened  obligation
}

// state is a point of the search: where it resumes, how many obligations on
// the value are open, how many deferred satisfiers will run at the exit,
// whether a check the search does not read left it unknown if the trigger
// failed, and whether another value displaced the trigger's error from the
// variables it was stored into. Under on-success it also holds whether a
// satisfier was called on the path, and the error value and the variable the
// path knows to hold a failure.
type state struct {
	block          *ssa.BasicBlock
	index          int
	open           int
	deferred       int
	uncertain      bool
	displaced      bool
	called         bool
	failedValue    ssa.Value
	failedVariable ssa.Value
}

func (s search) leaks() leak {
	start := state{block: s.opened.call.Block(), index: s.opened.index + 1, open: 1}
	pending := []state{start}
	visited := make(map[state]bool)
	for len(pending) > 0 {
		last := len(pending) - 1
		current := pending[last]
		pending = pending[:last]
		if visited[current] {
			continue
		}
		visited[current] = true
		next, found := s.advance(current)
		if found != leakNone {
			return found
		}
		pending = append(pending, next...)
	}
	return leakNone
}

// advance runs the instructions of a block from where current resumes. It
// returns the states to continue from, or whether an exit is reached with the
// obligation open.
func (s search) advance(current state) ([]state, leak) {
	instructions := current.block.Instrs
	for index := current.index; index < len(instructions); index++ {
		current = current.stored(s.opened.failure, instructions[index])
		current = s.remembered(current, instructions[index])
		happened := s.classify(instructions, index)
		if s.callsBeforeDefer(current, instructions[index], happened) {
			return nil, leakBeforeDefer
		}
		if ret, ok := instructions[index].(*ssa.Return); ok && happened == eventExit {
			return nil, s.leakAt(current, ret)
		}
		next, ended, leaked := current.after(happened)
		if ended && leaked {
			return nil, leakAtExit
		}
		if ended {
			return nil, leakNone
		}
		current = next
	}
	return s.successors(current), leakNone
}

// callsBeforeDefer reports whether instruction is, under defer-first, a call
// made while no deferred satisfier covers the open obligation. A satisfier, a
// builtin, a call that does not return and the evaluation of a deferred
// satisfier's arguments do not count.
func (s search) callsBeforeDefer(current state, instruction ssa.Instruction, happened event) bool {
	if !s.binding.protocol.deferFirst || current.uncertain || current.deferred >= current.open {
		return false
	}
	call, ok := instruction.(*ssa.Call)
	if !ok || happened == eventAbandon || s.discharges(call) {
		return false
	}
	if _, builtin := call.Call.Value.(*ssa.Builtin); builtin {
		return false
	}
	return !s.feedsDeferredSatisfier(call)
}

// feedsDeferredSatisfier reports whether every use of call's result is a
// deferred satisfier: the call computes that defer's arguments.
func (s search) feedsDeferredSatisfier(call *ssa.Call) bool {
	referrers := call.Referrers()
	if referrers == nil || len(*referrers) == 0 {
		return false
	}
	for _, referrer := range *referrers {
		deferred, ok := referrer.(*ssa.Defer)
		if !ok || !s.discharges(deferred) {
			return false
		}
	}
	return true
}

// successors continues into the blocks after current. Past a check of the
// trigger's error, only the branch where the trigger succeeded carries the
// obligation.
func (s search) successors(current state) []state {
	block := current.block
	branch, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If)
	if ok && s.opened.failure.value != nil {
		switch s.opened.failure.readAt(current.displaced).verdictOf(branch.Cond) {
		case verdictFailed:
			return []state{current.into(block.Succs[1], false)}
		case verdictSucceeded:
			return []state{current.into(block.Succs[0], false)}
		case verdictUnknown:
			return []state{current.into(block.Succs[0], true), current.into(block.Succs[1], true)}
		}
	}
	next := make([]state, 0, len(block.Succs))
	for position, successor := range block.Succs {
		following := current.into(successor, current.uncertain)
		if ok && s.binding.protocol.onSuccess {
			following = following.along(branch, position)
		}
		next = append(next, following)
	}
	return next
}

// leakAt is how the path leaves the obligation open at ret, if it does.
func (s search) leakAt(current state, ret *ssa.Return) leak {
	switch {
	case current.uncertain:
		return leakNone
	case current.open > current.deferred:
		return leakAtExit
	case s.binding.protocol.onSuccess && !current.called && !s.returnsFailure(current, ret):
		return leakOnSuccess
	}
	return leakNone
}

// after is the state once happened took place, and whether the path ended
// there with the obligation open. An exit is read by leakAt, before this.
func (s state) after(happened event) (next state, ended, leaked bool) {
	switch happened {
	case eventTrigger:
		s.open = min(s.open+1, saturated)
	case eventSatisfier:
		s.open--
		s.called = true
		return s, s.open == 0, false
	case eventUndeferred:
		s.called = true
	case eventDeferred:
		s.deferred = min(s.deferred+1, saturated)
	case eventAbandon, eventTransfer:
		return s, true, false
	}
	return s, false, false
}

func (s state) into(block *ssa.BasicBlock, uncertain bool) state {
	return state{
		block:          block,
		open:           s.open,
		deferred:       s.deferred,
		uncertain:      uncertain,
		displaced:      s.displaced,
		called:         s.called,
		failedValue:    s.failedValue,
		failedVariable: s.failedVariable,
	}
}

// stored is s after instruction, which may store into a variable the trigger's
// error was stored into: the error again, or another value that displaces it.
func (s state) stored(failed failure, instruction ssa.Instruction) state {
	store, ok := instruction.(*ssa.Store)
	if !ok || !slices.Contains(failed.homes, store.Addr) {
		return s
	}
	s.displaced = store.Val != failed.value
	return s
}

func (s search) classify(instructions []ssa.Instruction, index int) event {
	switch typed := instructions[index].(type) {
	case *ssa.Call:
		return s.called(typed)
	case *ssa.Defer:
		if s.discharges(typed) || s.closureDischarges(typed) {
			return eventDeferred
		}
		if s.handsOver(typed.Common()) {
			return eventTransfer
		}
	case *ssa.Return:
		if s.returns(typed) {
			return eventTransfer
		}
		return eventExit
	case *ssa.Panic:
		return eventAbandon
	}
	if s.escaped(instructions[index]) {
		return eventTransfer
	}
	return eventNone
}

func (s search) called(call *ssa.Call) event {
	if s.discharges(call) {
		if s.binding.protocol.requireDefer {
			return eventUndeferred
		}
		return eventSatisfier
	}
	if s.reopens(call) {
		if s.binding.protocol.idempotent {
			return eventNone
		}
		return eventTrigger
	}
	if s.handsOver(call.Common()) {
		return eventTransfer
	}
	return eventNone
}

// discharges reports whether call is a satisfier on the value this search
// follows. A value that may be the same one discharges: only one proven to be
// another does not.
func (s search) discharges(call ssa.CallInstruction) bool {
	index, ok := s.binding.satisfierOf(call.Common())
	if !ok {
		return false
	}
	value := s.binding.satisfierSlots[index].valueAt(call)
	return s.binding.compare(value, s.opened.value) != sameNo
}

func (s search) closureDischarges(deferred *ssa.Defer) bool {
	deferredClosure, ok := closureOf(s, deferred)
	return ok && deferredClosure.discharges()
}

// reopens reports whether call is a trigger that opens another obligation on
// the value this search follows, which only a proven same value does.
func (s search) reopens(call *ssa.Call) bool {
	if !s.binding.protocol.opens(call.Common()) {
		return false
	}
	value := s.binding.triggerSlot.valueAt(call)
	return s.binding.compare(value, s.opened.value) == sameYes
}
