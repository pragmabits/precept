package main

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

	"github.com/pragmabits/precept/paircheck"
)

var (
	errNoSettings = errors.New("the golangci-lint configuration has no paircheck settings")
	errRunTests   = errors.New("run.tests is neither true nor false")
	errCaseKeys   = errors.New("keys differ only in case")
	errBuildTags  = errors.New("run.build-tags is not a list of build tags")
)

// readConfig decodes the file the way the module plugin decodes its settings:
// through JSON, refusing a key the configuration does not have. It also says
// how the file asks for the packages to be loaded.
func readConfig(path string) (paircheck.Config, loading, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return paircheck.Config{}, loading{}, err
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		return paircheck.Config{}, loading{}, fmt.Errorf("%s: %w", path, err)
	}
	document, err = folded(document)
	if err != nil {
		return paircheck.Config{}, loading{}, fmt.Errorf("%s: %w", path, err)
	}
	settings, load, err := settingsOf(document)
	if err != nil {
		return paircheck.Config{}, loading{}, fmt.Errorf("%s: %w", path, err)
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		return paircheck.Config{}, loading{}, fmt.Errorf("%s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var config paircheck.Config
	if err := decoder.Decode(&config); err != nil {
		return paircheck.Config{}, loading{}, fmt.Errorf("%s: %w", path, err)
	}
	return config, load, nil
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
// document itself or in the paircheck settings of a golangci-lint
// configuration, native or as a module plugin, and how the packages are
// loaded, which only a golangci-lint configuration says, in its run section.
func settingsOf(document map[string]any) (any, loading, error) {
	linters, isGolangci := document["linters"].(map[string]any)
	if !isGolangci {
		return document, loading{tests: true}, nil
	}
	section, _ := document["run"].(map[string]any)
	load, err := loadingOf(section)
	if err != nil {
		return nil, loading{}, err
	}
	settings, _ := linters["settings"].(map[string]any)
	if native, ok := settings[linterName]; ok {
		return native, load, nil
	}
	custom, _ := settings["custom"].(map[string]any)
	plugin, _ := custom[linterName].(map[string]any)
	if pluginSettings, ok := plugin["settings"]; ok {
		return pluginSettings, load, nil
	}
	return nil, loading{}, errNoSettings
}

// loadingOf reads the run section of a golangci-lint configuration as
// golangci-lint decodes it, weakly typed: run.tests, run.build-tags and
// run.modules-download-mode.
func loadingOf(section map[string]any) (loading, error) {
	tests, err := testsOf(section[flagTests])
	if err != nil {
		return loading{}, err
	}
	tags, err := tagsOf(section[flagBuildTags])
	if err != nil {
		return loading{}, err
	}
	mode := ""
	if value := section[flagDownloadMode]; value != nil {
		written, isText := text(value)
		if !isText {
			return loading{}, errMode
		}
		mode = written
	}
	return loading{tests: tests, buildTags: tags, downloadMode: mode}, nil
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
