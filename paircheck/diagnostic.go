package paircheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// unnamed stands for the value when no expression of the source spells it.
const unnamed = "the value"

// syntax indexes what a diagnostic needs from the source: the call an SSA call
// instruction came from, found by its opening parenthesis, and the expressions
// a call's results are assigned to.
type syntax struct {
	calls   map[token.Pos]*ast.CallExpr
	targets map[*ast.CallExpr][]ast.Expr
}

func indexSyntax(files []*ast.File) syntax {
	indexed := syntax{
		calls:   make(map[token.Pos]*ast.CallExpr),
		targets: make(map[*ast.CallExpr][]ast.Expr),
	}
	for _, file := range files {
		ast.Inspect(file, indexed.visit)
	}
	return indexed
}

func (s syntax) visit(node ast.Node) bool {
	switch typed := node.(type) {
	case *ast.CallExpr:
		s.calls[typed.Lparen] = typed
	case *ast.AssignStmt:
		s.assign(typed.Lhs, typed.Rhs)
	case *ast.ValueSpec:
		names := make([]ast.Expr, 0, len(typed.Names))
		for _, name := range typed.Names {
			names = append(names, name)
		}
		s.assign(names, typed.Values)
	}
	return true
}

// assign records the targets of calls on the right: all of them for a call
// whose results are spread, one each when every value has its own.
func (s syntax) assign(targets, values []ast.Expr) {
	if len(values) == 1 {
		if call, ok := ast.Unparen(values[0]).(*ast.CallExpr); ok {
			s.targets[call] = targets
		}
		return
	}
	for index, value := range values {
		if call, ok := ast.Unparen(value).(*ast.CallExpr); ok && index < len(targets) {
			s.targets[call] = targets[index : index+1]
		}
	}
}

func (s syntax) report(pass *analysis.Pass, violated binding, opened obligation) {
	position := opened.call.Pos()
	expression := unnamed
	if call, ok := s.calls[position]; ok {
		position = call.Pos()
		expression = s.expression(call, violated.triggerSlot)
	}
	pass.Report(analysis.Diagnostic{Pos: position, Message: message(violated.protocol, expression)})
}

// expression spells the value a call carries in a slot.
func (s syntax) expression(call *ast.CallExpr, position slot) string {
	switch position.kind {
	case slotReceiver:
		if selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			return types.ExprString(selector.X)
		}
	case slotArgument:
		if position.index < len(call.Args) {
			return types.ExprString(call.Args[position.index])
		}
	case slotResult:
		return s.result(call, position.index)
	}
	return unnamed
}

func (s syntax) result(call *ast.CallExpr, index int) string {
	targets, assigned := s.targets[call]
	if !assigned {
		return types.ExprString(call)
	}
	if index >= len(targets) {
		return unnamed
	}
	if identifier, ok := targets[index].(*ast.Ident); ok && identifier.Name == "_" {
		return unnamed
	}
	return types.ExprString(targets[index])
}

func message(violated protocol, expression string) string {
	return fmt.Sprintf(
		"[%s] %s requires %s on %s before function exit",
		violated.id,
		violated.trigger.name.name,
		required(violated.satisfiers),
		expression,
	)
}

func required(satisfiers []side) string {
	if len(satisfiers) == 1 {
		return satisfiers[0].name.name
	}
	names := make([]string, 0, len(satisfiers))
	for _, satisfier := range satisfiers {
		names = append(names, satisfier.name.name)
	}
	return "one of [" + strings.Join(names, ", ") + "]"
}
