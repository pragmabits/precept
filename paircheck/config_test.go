package paircheck_test

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/pragmabits/precept/paircheck"
)

func TestNewRefusesInvalidConfig(t *testing.T) {
	tests := []struct {
		defect string
		want   error
		change func(config *paircheck.Config)
	}{
		{
			defect: "no id",
			want:   paircheck.ErrNoID,
			change: func(config *paircheck.Config) {
				config.Rules[0].ID = ""
			},
		},
		{
			defect: "duplicate id",
			want:   paircheck.ErrDuplicateID,
			change: func(config *paircheck.Config) {
				config.Rules[1].ID = config.Rules[0].ID
			},
		},
		{
			defect: "no trigger",
			want:   paircheck.ErrNoTrigger,
			change: func(config *paircheck.Config) {
				config.Rules[0].Trigger.Name = ""
			},
		},
		{
			defect: "no satisfiers",
			want:   paircheck.ErrNoSatisfiers,
			change: func(config *paircheck.Config) {
				config.Rules[0].Satisfiers = nil
			},
		},
		{
			defect: "empty satisfier",
			want:   paircheck.ErrEmptySatisfier,
			change: func(config *paircheck.Config) {
				config.Rules[0].Satisfiers[0].Name = ""
			},
		},
	}
	for _, test := range tests {
		t.Run(test.defect, func(t *testing.T) {
			config := valid()
			test.change(&config)
			_, err := paircheck.New(config)
			if !errors.Is(err, test.want) {
				t.Errorf("New() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestNewRefusesUnknownCoverage(t *testing.T) {
	config := valid()
	config.Rules[0].DeferredClosure = "some-path"
	_, err := paircheck.New(config)
	if !errors.Is(err, paircheck.ErrUnknownCoverage) {
		t.Errorf("New() error = %v, want %v", err, paircheck.ErrUnknownCoverage)
	}
}

func TestNewRefusesMalformedNames(t *testing.T) {
	names := []string{
		"Open",
		"resource.",
		".Open",
		"resource.Open(",
		"(*resource.Resource.Open",
		"(*resource.Resource)Open",
		"(*resource.Resource).",
		"(*Resource).Open",
		"(*resource.Resource).Open.Close",
		"(**resource.Resource).Open",
		"(*resource.Pool[T).Acquire",
		"resource.1Open",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			config := valid()
			config.Rules[0].Trigger.Name = name
			_, err := paircheck.New(config)
			if !errors.Is(err, paircheck.ErrInvalidName) {
				t.Errorf("New() error = %v, want %v", err, paircheck.ErrInvalidName)
			}
		})
	}
}

func TestNewAcceptsEverySpelling(t *testing.T) {
	names := []string{
		"example.com/project/resource.Open",
		"(*example.com/project/resource.Resource).Open",
		"(example.com/project/resource.Resource).Open",
		"(*example.com/project/pool.Pool[T]).Acquire",
		"(*gopkg.in/yaml.v3.Decoder).Decode",
		"gopkg.in/yaml.v3.Unmarshal",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			config := valid()
			config.Rules[0].Trigger.Name = name
			if _, err := paircheck.New(config); err != nil {
				t.Errorf("New() error = %v, want nil", err)
			}
		})
	}
}

func TestNewRefusesMalformedSlots(t *testing.T) {
	slots := []string{
		"receiver 0",
		"argument",
		"argument x",
		"argument -1",
		"result 1 2",
		"results 0",
		"Result 0",
	}
	for _, slot := range slots {
		t.Run(slot, func(t *testing.T) {
			config := valid()
			config.Rules[0].Satisfiers[0].Slot = slot
			_, err := paircheck.New(config)
			if !errors.Is(err, paircheck.ErrInvalidSlot) {
				t.Errorf("New() error = %v, want %v", err, paircheck.ErrInvalidSlot)
			}
		})
	}
}

func TestNewAcceptsEverySlot(t *testing.T) {
	for _, slot := range []string{"", "receiver", "argument 0", "result 1"} {
		t.Run(slot, func(t *testing.T) {
			config := valid()
			config.Rules[0].Trigger.Slot = slot
			if _, err := paircheck.New(config); err != nil {
				t.Errorf("New() error = %v, want nil", err)
			}
		})
	}
}

func TestCallDecodesBothForms(t *testing.T) {
	const document = `{
		"id": "transaction",
		"trigger": "(*database/sql.DB).Begin",
		"satisfiers": [
			"(*database/sql.Tx).Commit",
			{"name": "(*database/sql.Tx).Rollback", "slot": "receiver"}
		]
	}`
	var decoded paircheck.Rule
	if err := json.Unmarshal([]byte(document), &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	got := []string{
		decoded.Trigger.Name,
		decoded.Satisfiers[0].Name,
		decoded.Satisfiers[1].Name,
		decoded.Satisfiers[1].Slot,
	}
	want := []string{
		"(*database/sql.DB).Begin",
		"(*database/sql.Tx).Commit",
		"(*database/sql.Tx).Rollback",
		"receiver",
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("name %d = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestCallRefusesOtherForms(t *testing.T) {
	documents := map[string]string{
		"number":      `{"trigger": 1}`,
		"list":        `{"trigger": ["a.B"]}`,
		"unknown key": `{"trigger": {"name": "a.B", "unknown": true}}`,
	}
	for defect, document := range documents {
		t.Run(defect, func(t *testing.T) {
			var decoded paircheck.Rule
			if err := json.Unmarshal([]byte(document), &decoded); err == nil {
				t.Errorf("Unmarshal(%s) error = nil, want an error", document)
			}
		})
	}
}

func TestCallDecodesText(t *testing.T) {
	var call paircheck.Call
	if err := call.UnmarshalText([]byte("(*database/sql.DB).Begin")); err != nil {
		t.Fatalf("UnmarshalText: %v", err)
	}
	if call.Name != "(*database/sql.DB).Begin" {
		t.Errorf("Name = %q, want %q", call.Name, "(*database/sql.DB).Begin")
	}
}

func TestTransferDecodes(t *testing.T) {
	tests := map[string]string{
		`"all"`:               "true true true",
		`"none"`:              "false false false",
		`{"argument": false}`: "unset unset false",
		`{}`:                  "unset unset unset",
	}
	for document, want := range tests {
		t.Run(document, func(t *testing.T) {
			var decoded paircheck.Transfer
			if err := json.Unmarshal([]byte(document), &decoded); err != nil {
				t.Fatalf("Unmarshal(%s): %v", document, err)
			}
			got := strings.Join([]string{
				render(decoded.Return),
				render(decoded.Store),
				render(decoded.Argument),
			}, " ")
			if got != want {
				t.Errorf("Unmarshal(%s) = %s, want %s", document, got, want)
			}
		})
	}
}

func TestTransferRefusesOtherForms(t *testing.T) {
	var decoded paircheck.Transfer
	err := json.Unmarshal([]byte(`"some"`), &decoded)
	if !errors.Is(err, paircheck.ErrUnknownTransfer) {
		t.Errorf(`Unmarshal("some") error = %v, want %v`, err, paircheck.ErrUnknownTransfer)
	}
	for _, document := range []string{`true`, `{"everything": true}`, `1`} {
		if err := json.Unmarshal([]byte(document), &decoded); err == nil {
			t.Errorf("Unmarshal(%s) error = nil, want an error", document)
		}
	}
}

func render(flag *bool) string {
	if flag == nil {
		return "unset"
	}
	return strconv.FormatBool(*flag)
}

func TestPackagesListsEveryPathOnce(t *testing.T) {
	config := paircheck.Config{Rules: []paircheck.Rule{
		rule("transaction", "(*database/sql.DB).Begin", "(*database/sql.Tx).Commit"),
		rule("file", "os.Open", "(*os.File).Close", "gopkg.in/yaml.v3.Release"),
	}}
	got, err := paircheck.Packages(config)
	if err != nil {
		t.Fatalf("Packages: %v", err)
	}
	want := []string{"database/sql", "gopkg.in/yaml.v3", "os"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("Packages() = %v, want %v", got, want)
	}
}

func valid() paircheck.Config {
	return paircheck.Config{Rules: []paircheck.Rule{
		rule("resource", "(*resource.Resource).Open", "(*resource.Resource).Close"),
		rule("transaction", "(*resource.Resource).Begin", "(*resource.Resource).Commit"),
	}}
}
