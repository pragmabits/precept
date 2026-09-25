package paircheck

import (
	"fmt"
	"go/token"
	"go/types"
	"strings"
)

// qualifiedName is a function or method as types.Func.FullName spells it,
// without the receiver's pointer and type parameters: a type cannot declare one
// method name on both its value and its pointer, and an instantiation matches
// through its origin.
type qualifiedName struct {
	path     string
	receiver string
	name     string
}

func parseName(text string) (qualifiedName, error) {
	if strings.HasPrefix(text, "(") {
		return parseMethod(text)
	}
	path, name, found := cutLast(text, ".")
	if !found || !isImportPath(path) || !token.IsIdentifier(name) {
		return qualifiedName{}, fmt.Errorf("%w: %q", ErrInvalidName, text)
	}
	return qualifiedName{path: path, name: name}, nil
}

func parseMethod(text string) (qualifiedName, error) {
	receiver, name, found := strings.Cut(text[1:], ").")
	if !found || !token.IsIdentifier(name) {
		return qualifiedName{}, fmt.Errorf("%w: %q", ErrInvalidName, text)
	}
	receiver = withoutTypeParameters(strings.TrimPrefix(receiver, "*"))
	path, typeName, found := cutLast(receiver, ".")
	if !found || !isImportPath(path) || !token.IsIdentifier(typeName) {
		return qualifiedName{}, fmt.Errorf("%w: %q", ErrInvalidName, text)
	}
	return qualifiedName{path: path, receiver: typeName, name: name}, nil
}

func (q qualifiedName) matches(function *types.Func) bool {
	function = function.Origin()
	if function.Name() != q.name || function.Pkg() == nil || function.Pkg().Path() != q.path {
		return false
	}
	return receiverName(function) == q.receiver
}

// String spells the name as types.Func.FullName does, without the pointer.
func (q qualifiedName) String() string {
	if q.receiver == "" {
		return q.path + "." + q.name
	}
	return "(" + q.path + "." + q.receiver + ")." + q.name
}

func receiverName(function *types.Func) string {
	receiver := function.Signature().Recv()
	if receiver == nil {
		return ""
	}
	receiverType := types.Unalias(receiver.Type())
	if pointer, ok := receiverType.(*types.Pointer); ok {
		receiverType = types.Unalias(pointer.Elem())
	}
	if named, ok := receiverType.(*types.Named); ok {
		return named.Obj().Name()
	}
	return ""
}

func withoutTypeParameters(receiver string) string {
	open := strings.IndexByte(receiver, '[')
	if open < 0 || !strings.HasSuffix(receiver, "]") {
		return receiver
	}
	return receiver[:open]
}

func cutLast(text, separator string) (before, after string, found bool) {
	index := strings.LastIndex(text, separator)
	if index < 0 {
		return "", "", false
	}
	return text[:index], text[index+len(separator):], true
}

func isImportPath(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return false
	}
	return !strings.ContainsFunc(path, func(character rune) bool {
		return !isPathCharacter(character)
	})
}

func isPathCharacter(character rune) bool {
	switch {
	case 'a' <= character && character <= 'z', 'A' <= character && character <= 'Z':
		return true
	case '0' <= character && character <= '9':
		return true
	}
	return strings.ContainsRune("./-_~+", character)
}
