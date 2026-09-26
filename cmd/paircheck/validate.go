package main

import (
	"fmt"
	"go/types"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"

	"github.com/pragmabits/precept/paircheck"
)

// validate loads the packages the rules name, once, under the build flags of
// the analysis and without their tests, and checks the rules against them: a
// rule on a function declared in a _test.go file is refused, although the
// analysis applies it. A name relative to the module is resolved against the
// module of the current directory.
func validate(config paircheck.Config, load loading) (int, error) {
	module, err := currentModule()
	if err != nil {
		return exitFailed, err
	}
	paths, err := paircheck.Packages(config, module)
	if err != nil {
		return exitFailed, err
	}
	load.tests = false
	mode := packages.NeedName | packages.NeedTypes | packages.NeedImports | packages.NeedDeps
	loaded, err := packages.Load(load.packagesConfig(mode), paths...)
	if err != nil {
		return exitFailed, err
	}
	found := make([]*types.Package, 0, len(loaded))
	for _, current := range loaded {
		if current.Types != nil && len(current.Errors) == 0 {
			found = append(found, current.Types)
		}
	}
	if err := paircheck.Validate(config, module, found); err != nil {
		return exitFailed, err
	}
	return exitClean, nil
}

// currentModule is the path of the module the current directory belongs to,
// as the go command finds its go.mod, or empty outside a module.
func currentModule() (string, error) {
	output, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		return "", fmt.Errorf("go env GOMOD: %w", err)
	}
	file := strings.TrimSpace(string(output))
	if file == "" || file == os.DevNull {
		return "", nil
	}
	content, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return modfile.ModulePath(content), nil
}
