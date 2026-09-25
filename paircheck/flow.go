package paircheck

import "golang.org/x/tools/go/ssa"

// saturated is the count from which obligations on one value are no longer
// told apart: two or more. It keeps the search finite through loops.
const saturated = 2

type event int

const (
	eventNone event = iota
	eventTrigger
	eventSatisfier
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
// the value are open, how many deferred satisfiers will run at the exit, and
// whether a check the search does not read left it unknown if the trigger
// failed.
type state struct {
	block     *ssa.BasicBlock
	index     int
	open      int
	deferred  int
	uncertain bool
}

func (s search) leaks() bool {
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
		next, leaked := s.advance(current)
		if leaked {
			return true
		}
		pending = append(pending, next...)
	}
	return false
}

// advance runs the instructions of a block from where current resumes. It
// returns the states to continue from, or whether an exit is reached with the
// obligation open.
func (s search) advance(current state) ([]state, bool) {
	instructions := current.block.Instrs
	for index := current.index; index < len(instructions); index++ {
		next, ended, leaked := current.after(s.classify(instructions, index))
		if ended {
			return nil, leaked
		}
		current = next
	}
	return s.successors(current), false
}

// successors continues into the blocks after current. Past a check of the
// trigger's error, only the branch where the trigger succeeded carries the
// obligation.
func (s search) successors(current state) []state {
	block := current.block
	branch, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If)
	if ok && s.opened.failure.value != nil {
		switch s.opened.failure.verdictOf(branch.Cond) {
		case verdictFailed:
			return []state{current.into(block.Succs[1], false)}
		case verdictSucceeded:
			return []state{current.into(block.Succs[0], false)}
		case verdictUnknown:
			return []state{current.into(block.Succs[0], true), current.into(block.Succs[1], true)}
		}
	}
	next := make([]state, 0, len(block.Succs))
	for _, successor := range block.Succs {
		next = append(next, current.into(successor, current.uncertain))
	}
	return next
}

// after is the state once happened took place, and whether the path ended
// there with the obligation open.
func (s state) after(happened event) (next state, ended, leaked bool) {
	switch happened {
	case eventTrigger:
		s.open = min(s.open+1, saturated)
	case eventSatisfier:
		s.open--
		return s, s.open == 0, false
	case eventDeferred:
		s.deferred = min(s.deferred+1, saturated)
	case eventExit:
		return s, true, !s.uncertain && s.open > s.deferred
	case eventAbandon, eventTransfer:
		return s, true, false
	}
	return s, false, false
}

func (s state) into(block *ssa.BasicBlock, uncertain bool) state {
	return state{block: block, open: s.open, deferred: s.deferred, uncertain: uncertain}
}

func (s search) classify(instructions []ssa.Instruction, index int) event {
	switch typed := instructions[index].(type) {
	case *ssa.Call:
		return s.called(typed)
	case *ssa.Defer:
		if s.discharges(typed) || s.closureDischarges(typed) {
			return eventDeferred
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
		return eventSatisfier
	}
	if s.reopens(call) {
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
