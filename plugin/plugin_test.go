package plugin_test

import (
	"testing"

	"github.com/golangci/plugin-module-register/register"

	_ "github.com/pragmabits/precept/plugin"
)

func TestRegistersPaircheck(t *testing.T) {
	constructor, err := register.GetPlugin("paircheck")
	if err != nil {
		t.Fatalf("GetPlugin: %v", err)
	}
	linter, err := constructor(map[string]any{
		"rules": []any{map[string]any{
			"id":         "resource",
			"trigger":    "(*example.com/resource.Resource).Open",
			"satisfiers": []any{"(*example.com/resource.Resource).Close"},
			"transfer":   map[string]any{"argument": false},
		}},
	})
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	analyzers, err := linter.BuildAnalyzers()
	if err != nil {
		t.Fatalf("BuildAnalyzers: %v", err)
	}
	if len(analyzers) != 1 || analyzers[0].Name != "paircheck" {
		t.Errorf("BuildAnalyzers() = %v, want the paircheck analyzer alone", analyzers)
	}
	if mode := linter.GetLoadMode(); mode != register.LoadModeTypesInfo {
		t.Errorf("GetLoadMode() = %q, want %q", mode, register.LoadModeTypesInfo)
	}
}

func TestRefusesInvalidSettings(t *testing.T) {
	constructor, err := register.GetPlugin("paircheck")
	if err != nil {
		t.Fatalf("GetPlugin: %v", err)
	}
	rules := map[string]map[string]any{
		"no id":       {"trigger": "a.Open", "satisfiers": []any{"a.Close"}},
		"unknown key": {"id": "x", "trigger": "a.Open", "satisfiers": []any{"a.Close"}, "when": 1},
		"boolean transfer": {
			"id":         "x",
			"trigger":    "a.Open",
			"satisfiers": []any{"a.Close"},
			"transfer":   true,
		},
	}
	for defect, current := range rules {
		t.Run(defect, func(t *testing.T) {
			if _, err := constructor(map[string]any{"rules": []any{current}}); err == nil {
				t.Errorf("constructor error = nil, want an error")
			}
		})
	}
}

func TestRegistersDefcheck(t *testing.T) {
	constructor, err := register.GetPlugin("defcheck")
	if err != nil {
		t.Fatalf("GetPlugin: %v", err)
	}
	linter, err := constructor(map[string]any{
		"rules": []any{map[string]any{
			"pattern": "^cfg$",
			"kinds":   []any{"local-var", "parameter", "field"},
			"message": "avoid the cfg abbreviation",
		}},
	})
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	analyzers, err := linter.BuildAnalyzers()
	if err != nil {
		t.Fatalf("BuildAnalyzers: %v", err)
	}
	if len(analyzers) != 1 || analyzers[0].Name != "defcheck" {
		t.Errorf("BuildAnalyzers() = %v, want the defcheck analyzer alone", analyzers)
	}
	if mode := linter.GetLoadMode(); mode != register.LoadModeTypesInfo {
		t.Errorf("GetLoadMode() = %q, want %q", mode, register.LoadModeTypesInfo)
	}
}

func TestRefusesInvalidDefcheckSettings(t *testing.T) {
	constructor, err := register.GetPlugin("defcheck")
	if err != nil {
		t.Fatalf("GetPlugin: %v", err)
	}
	rules := map[string]map[string]any{
		"invalid pattern": {"pattern": "(cfg"},
		"unknown kind":    {"pattern": "cfg", "kinds": []any{"variable"}},
		"unknown key":     {"pattern": "cfg", "kind": "field"},
		"kinds as text":   {"pattern": "cfg", "kinds": "field"},
	}
	for defect, current := range rules {
		t.Run(defect, func(t *testing.T) {
			if _, err := constructor(map[string]any{"rules": []any{current}}); err == nil {
				t.Errorf("constructor error = nil, want an error")
			}
		})
	}
}
