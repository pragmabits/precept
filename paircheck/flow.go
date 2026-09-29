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

// ending is how a path that is not searched further ended.
type ending int

const (
	endNone ending = iota
	// endDischarged closes the obligation: a satisfier, or a return that runs
	// deferred ones enough.
	endDischarged
	// endOpen returns with the obligation open.
	endOpen
	// endTransferred lets the value, and its obligation, leave.
	endTransferred
	// endAbandoned stops without returning.
	endAbandoned
)

// endings is every way the paths of a search ended.
type endings map[ending]bool

// obligation is what a trigger call opens on a value: the variables the value
// was kept in, and the error the call returned, which the obligation exists
// only without. An obligation a summary follows opens on a parameter, at the
// start of its function, and no call opened it.
type obligation struct {
	start   *ssa.BasicBlock
	call    *ssa.Call
	index   int
	value   ssa.Value
	homes   []ssa.Value
	failure failure
}

// search looks for a path from a trigger call to an exit of its function along
// which the obligation is still open. Under on-success, returned indexes the
// errors the function can return, and closures the deferred closures that call
// a satisfier.
type search struct {
	binding  binding
	opened   obligation
	returned map[ssa.Value]int
	closures map[*ssa.Defer]int
	ended    endings
}

// state is a point of the search: where it resumes, how many obligations on
// the value are open, how many deferred satisfiers will run at the exit,
// whether a check the search does not read left it unknown if the trigger
// failed, and whether another value displaced the trigger's error from the
// variables it was stored into. Under on-success it also holds whether a
// satisfier was called on the path, the errors the path knows to be failures,
// and the closures it deferred that call a satisfier.
type state struct {
	block     *ssa.BasicBlock
	index     int
	open      int
	deferred  int
	uncertain bool
	displaced bool
	called    bool
	failed    proven
	closures  proven
}

// leaks searches every state once. The failures a state knows are the one part
// of it that is not compared as a whole: knowing more only saves a return from
// leaking, so a point already searched knowing some failures is not searched
// again knowing more, and is searched again knowing only those both know.
func (s search) leaks() leak {
	return s.explore(true)
}

// explore searches the paths from the obligation, and returns the first leak
// it finds, stopping there when first says so. Every way a path ends goes to
// ended, when the search has one.
func (s search) explore(first bool) leak {
	block := s.opened.start
	if block == nil {
		block = s.opened.call.Block()
	}
	start := state{block: block, index: s.opened.index + 1, open: 1}
	found := leakNone
	pending := []state{start}
	searched := make(map[state]proven)
	for len(pending) > 0 {
		last := len(pending) - 1
		current := pending[last]
		pending = pending[:last]
		point := current
		point.failed = ""
		if known, seen := searched[point]; seen {
			if current.failed.includes(known) {
				continue
			}
			current.failed = current.failed.common(known)
		}
		searched[point] = current.failed
		next, leaked := s.advance(current)
		if leaked != leakNone && found == leakNone {
			found = leaked
		}
		if found != leakNone && first {
			return found
		}
		pending = append(pending, next...)
	}
	return found
}

// advance runs the instructions of a block from where current resumes. It
// returns the states to continue from, or whether an exit is reached with the
// obligation open.
func (s search) advance(current state) ([]state, leak) {
	instructions := current.block.Instrs
	for index := current.index; index < len(instructions); index++ {
		current = current.stored(s.opened.failure, instructions[index])
		current = s.remembered(current, instructions[index])
		current = s.registered(current, instructions[index])
		happened := s.classify(instructions, index)
		if s.callsBeforeDefer(current, instructions[index], happened) {
			return nil, leakBeforeDefer
		}
		if ret, ok := instructions[index].(*ssa.Return); ok && happened == eventExit {
			return nil, s.exit(current, ret)
		}
		next, ends := current.after(happened)
		if ends != endNone {
			s.end(ends)
			return nil, leakNone
		}
		current = next
	}
	return s.successors(current), leakNone
}

// exit is how the path leaves the obligation at ret, noted as the way it ended.
func (s search) exit(current state, ret *ssa.Return) leak {
	found := s.leakAt(current, ret)
	switch {
	case found == leakAtExit:
		s.end(endOpen)
	case current.open <= current.deferred:
		s.end(endDischarged)
	}
	return found
}

// end notes how a path ended, when the search keeps that.
func (s search) end(how ending) {
	if s.ended != nil {
		s.ended[how] = true
	}
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
			following = s.along(following, branch, position)
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
	case s.binding.protocol.onSuccess && !current.called && !s.returnsFailure(current, ret) &&
		!s.closureDecides(current, ret):
		return leakOnSuccess
	}
	return leakNone
}

// after is the state once happened took place, and how the path ended there,
// if it did. An exit is read by exit, before this.
func (s state) after(happened event) (state, ending) {
	switch happened {
	case eventTrigger:
		s.open = min(s.open+1, saturated)
	case eventSatisfier:
		s.open--
		s.called = true
		if s.open == 0 {
			return s, endDischarged
		}
	case eventUndeferred:
		s.called = true
	case eventDeferred:
		s.deferred = min(s.deferred+1, saturated)
	case eventAbandon:
		return s, endAbandoned
	case eventTransfer:
		return s, endTransferred
	}
	return s, endNone
}

func (s state) into(block *ssa.BasicBlock, uncertain bool) state {
	return state{
		block:     block,
		open:      s.open,
		deferred:  s.deferred,
		uncertain: uncertain,
		displaced: s.displaced,
		called:    s.called,
		failed:    s.failed,
		closures:  s.closures,
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
		if happened := s.deferred(typed); happened != eventNone {
			return happened
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

// deferred is what a defer statement does with the value: register a
// satisfier, or hand the value over.
func (s search) deferred(statement *ssa.Defer) event {
	discharged, kept := s.summarized(statement.Common())
	if discharged || s.discharges(statement) || s.closureDischarges(statement) {
		return eventDeferred
	}
	if s.handsOver(statement.Common(), kept) {
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
	discharged, kept := s.summarized(call.Common())
	switch {
	case discharged && s.binding.protocol.requireDefer:
		return eventUndeferred
	case discharged:
		return eventSatisfier
	case s.handsOver(call.Common(), kept):
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
	if value == nil || s.opened.value == nil {
		return true
	}
	return s.binding.satisfied(index, pathOf(value), pathOf(s.opened.value)) != sameNo
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
