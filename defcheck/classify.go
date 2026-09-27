package defcheck

import "go/types"

// variables is the kind of each kind of variable go/types tells apart.
var variables = map[types.VarKind]Kind{
	types.PackageVar: KindPackageVar,
	types.LocalVar:   KindLocalVar,
	types.RecvVar:    KindReceiver,
	types.ParamVar:   KindParameter,
	types.ResultVar:  KindResult,
	types.FieldVar:   KindField,
}

// kindOf is the kind of declaration object is, or false when it is of no kind
// a rule applies to: a package name, a label, or a field embedded without a
// name of its own. A function whose signature has a receiver is a method,
// that of an interface included.
func kindOf(object types.Object) (Kind, bool) {
	switch typed := object.(type) {
	case *types.Func:
		if typed.Signature().Recv() != nil {
			return KindMethod, true
		}
		return KindFunction, true
	case *types.Var:
		if typed.Embedded() {
			return "", false
		}
		kind, found := variables[typed.Kind()]
		return kind, found
	case *types.Const:
		return KindConstant, true
	case *types.TypeName:
		if _, isParameter := typed.Type().(*types.TypeParam); isParameter {
			return KindTypeParameter, true
		}
		return KindType, true
	}
	return "", false
}
