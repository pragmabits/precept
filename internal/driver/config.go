package driver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

var (
	errRunTests  = errors.New("run.tests is neither true nor false")
	errCaseKeys  = errors.New("keys differ only in case")
	errBuildTags = errors.New("run.build-tags is not a list of build tags")
)

// Setup is what a golangci-lint configuration says of how a command runs,
// besides the rules: which findings in generated files it drops, decoded where
// the file is read, and the run section, kept as the file wrote it, since a
// flag written on the command line replaces its key, which is then not read
// (LoadingOf). The command's own file says nothing: its setup drops the findings
// in generated files as strict does.
type Setup struct {
	Generated Generated
	section   map[string]any
}

// Read decodes the file into T the way a module plugin decodes its settings:
// through JSON, refusing a key T does not have. The file is the command's own,
// T at its top, or a golangci-lint configuration carrying T in the settings of
// linter, native or as a module plugin. Read also returns what the
// golangci-lint configuration says of how the command runs.
func Read[T any](path, linter string) (T, Setup, error) {
	var config T
	content, err := os.ReadFile(path)
	if err != nil {
		return config, Setup{}, err
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		return config, Setup{}, fmt.Errorf("%s: %w", path, err)
	}
	document, err = folded(document)
	if err != nil {
		return config, Setup{}, fmt.Errorf("%s: %w", path, err)
	}
	settings, golangci, err := settingsOf(document, linter)
	if err != nil {
		return config, Setup{}, fmt.Errorf("%s: %w", path, err)
	}
	generated, err := generatedOf(golangci)
	if err != nil {
		return config, Setup{}, fmt.Errorf("%s: %w", path, err)
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		return config, Setup{}, fmt.Errorf("%s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return config, Setup{}, fmt.Errorf("%s: %w", path, err)
	}
	section, _ := golangci["run"].(map[string]any)
	return config, Setup{Generated: generated, section: section}, nil
}

// folded is section with the keys of its maps in lower case, at any depth but
// inside a list, as viper folds the configuration golangci-lint reads. Two
// keys that fold alike are refused: viper keeps either, as its map happens to
// iterate.
func folded(section map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(section))
	written := make(map[string]string, len(section))
	for _, key := range slices.Sorted(maps.Keys(section)) {
		lower := strings.ToLower(key)
		if other, taken := written[lower]; taken {
			return nil, fmt.Errorf("%w: %s and %s", errCaseKeys, other, key)
		}
		value := section[key]
		if nested, isMap := value.(map[string]any); isMap {
			inner, err := folded(nested)
			if err != nil {
				return nil, err
			}
			value = inner
		}
		written[lower] = key
		result[lower] = value
	}
	return result, nil
}

// settingsOf finds what the command reads in a document: the rules, in the
// document itself or in the settings of linter in a golangci-lint
// configuration, native or as a module plugin, and the golangci-lint
// configuration itself, when the document is one. A configuration carrying
// both is refused: golangci-lint applies the plugin's, and the command would
// have to pick one the file does not name.
func settingsOf(document map[string]any, linter string) (any, map[string]any, error) {
	linters, isGolangci := document["linters"].(map[string]any)
	if !isGolangci {
		return document, nil, nil
	}
	settings, _ := linters["settings"].(map[string]any)
	native, isNative := settings[linter]
	custom, _ := settings["custom"].(map[string]any)
	plugin, _ := custom[linter].(map[string]any)
	pluginSettings, isPlugin := plugin["settings"]
	switch {
	case isNative && isPlugin:
		return nil, nil, fmt.Errorf(
			"the golangci-lint configuration has both native and module plugin %s settings",
			linter,
		)
	case isNative:
		return native, document, nil
	case isPlugin:
		return pluginSettings, document, nil
	}
	return nil, nil, fmt.Errorf("the golangci-lint configuration has no %s settings", linter)
}

// testsOf reads run.tests as golangci-lint decodes it, weakly typed: text as
// strconv.ParseBool reads it, with the empty text false, a number as whether
// it is not zero, and a key left empty as unwritten, which is true.
func testsOf(value any) (bool, error) {
	switch typed := value.(type) {
	case nil:
		return true, nil
	case bool:
		return typed, nil
	case int:
		return typed != 0, nil
	case int64:
		return typed != 0, nil
	case uint64:
		return typed != 0, nil
	case float64:
		return typed != 0, nil
	case string:
		if typed == "" {
			return false, nil
		}
		parsed, err := strconv.ParseBool(typed)
		if err != nil {
			return false, errRunTests
		}
		return parsed, nil
	}
	return false, errRunTests
}

// tagsOf reads run.build-tags as golangci-lint decodes it, weakly typed: a list
// of values read as text, a single value as a list of one, an empty element as
// the empty tag, and an empty map, or a key left empty, as no tag.
func tagsOf(value any) ([]string, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case map[string]any:
		if len(typed) > 0 {
			return nil, errBuildTags
		}
		return nil, nil
	case []any:
		tags := make([]string, 0, len(typed))
		for _, element := range typed {
			tag, isText := text(element)
			if element != nil && !isText {
				return nil, errBuildTags
			}
			tags = append(tags, tag)
		}
		return tags, nil
	}
	tag, isText := text(value)
	if !isText {
		return nil, errBuildTags
	}
	return []string{tag}, nil
}

// modeOf reads run.modules-download-mode as golangci-lint decodes it: a text,
// weakly typed, or no mode when the key is left empty.
func modeOf(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	mode, isText := text(value)
	if !isText {
		return "", errMode
	}
	return mode, nil
}

// text reads value as golangci-lint's decoder reads a text, weakly typed: a
// boolean as 1 or 0, and a number by its decimal digits.
func text(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case bool:
		if typed {
			return "1", true
		}
		return "0", true
	case int:
		return strconv.Itoa(typed), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case uint64:
		return strconv.FormatUint(typed, 10), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	}
	return "", false
}
