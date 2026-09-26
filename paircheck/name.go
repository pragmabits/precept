package paircheck

import (
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"slices"
	"strings"
)

var ErrNoModule = errors.New("the name is relative to the module, and the package belongs to none")

const (
	// moduleRoot is the path of the module's own package, in a name relative
	// to the module.
	moduleRoot = "."
	// relativePrefix starts the path of a package below the module's own.
	relativePrefix = "./"
)

// qualifiedName is a function or method as types.Func.FullName spells it,
// without the receiver's pointer and type parameters: a type cannot declare one
// method name on both its value and its pointer, and an instantiation matches
// through its origin. A relative name holds its path inside the module, empty
// for the module's own package, until the module is known.
type qualifiedName struct {
	path     string
	receiver string
	name     string
	relative bool
}

func parseName(text string) (qualifiedName, error) {
	if strings.HasPrefix(text, "(") {
		return parseMethod(text)
	}
	written, name, found := cutLast(text, ".")
	path, relative, valid := parsePath(written)
	if !found || !valid || !token.IsIdentifier(name) {
		return qualifiedName{}, fmt.Errorf("%w: %q", ErrInvalidName, text)
	}
	return qualifiedName{path: path, name: name, relative: relative}, nil
}

func parseMethod(text string) (qualifiedName, error) {
	receiver, name, found := strings.Cut(text[1:], ").")
	if !found || !token.IsIdentifier(name) {
		return qualifiedName{}, fmt.Errorf("%w: %q", ErrInvalidName, text)
	}
	receiver = withoutTypeParameters(strings.TrimPrefix(receiver, "*"))
	written, typeName, found := cutLast(receiver, ".")
	path, relative, valid := parsePath(written)
	if !found || !valid || !token.IsIdentifier(typeName) {
		return qualifiedName{}, fmt.Errorf("%w: %q", ErrInvalidName, text)
	}
	return qualifiedName{path: path, receiver: typeName, name: name, relative: relative}, nil
}

// parsePath reads the path of a name: an import path, or a path relative to
// the module of the analyzed package, which is returned without its prefix. No
// import path starts with a dot, so a dot always starts a relative path.
func parsePath(written string) (path string, relative, valid bool) {
	if written == moduleRoot {
		return "", true, true
	}
	inside, relative := strings.CutPrefix(written, relativePrefix)
	if !relative {
		return written, false, isImportPath(written) && !strings.HasPrefix(written, ".")
	}
	elements := strings.Split(inside, "/")
	return inside, true, isImportPath(inside) && !slices.ContainsFunc(elements, isDotElement)
}

// within is q with a path relative to the module resolved against module, the
// path of the module the analyzed package belongs to, which is empty when it
// belongs to none.
func (q qualifiedName) within(module string) (qualifiedName, error) {
	if !q.relative {
		return q, nil
	}
	if module == "" {
		return qualifiedName{}, fmt.Errorf("%s: %w", q, ErrNoModule)
	}
	resolved := q
	resolved.relative = false
	resolved.path = module
	if q.path != "" {
		resolved.path = module + "/" + q.path
	}
	return resolved, nil
}

// namesWithin is every name resolved against module, in a new slice.
func namesWithin(names []qualifiedName, module string) ([]qualifiedName, error) {
	resolved := make([]qualifiedName, 0, len(names))
	for _, name := range names {
		within, err := name.within(module)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, within)
	}
	return resolved, nil
}

func (q qualifiedName) matches(function *types.Func) bool {
	function = function.Origin()
	if function.Name() != q.name || function.Pkg() == nil || function.Pkg().Path() != q.path {
		return false
	}
	return receiverName(function) == q.receiver
}

// String spells the name as types.Func.FullName does, without the pointer, and
// a relative name as the configuration wrote it.
func (q qualifiedName) String() string {
	path := q.path
	switch {
	case q.relative && path == "":
		path = moduleRoot
	case q.relative:
		path = relativePrefix + path
	}
	if q.receiver == "" {
		return path + "." + q.name
	}
	return "(" + path + "." + q.receiver + ")." + q.name
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

// isDotElement reports whether element of a relative path is empty or leads
// to the directory itself or out of it.
func isDotElement(element string) bool {
	return element == "" || element == "." || element == ".."
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
