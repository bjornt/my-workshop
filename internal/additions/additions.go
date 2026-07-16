// Package additions loads and parses optional external configuration for my-workshop.
package additions

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bjornt/my-workshop/internal/workshop"
	"github.com/bjornt/my-workshop/internal/yamlconfig"
)

// Config holds optional additions to a workshop configuration.
// An empty/zero Config is a noop, matching the Python DEFAULT_ADDITIONS == {} semantics.
type Config struct {
	Base      string
	SDKs      []yamlconfig.SDKSpec
	Provision workshop.ProvisionSpec
}

// DefaultAdditions returns the empty/noop config.
func DefaultAdditions() Config {
	return Config{
		SDKs: []yamlconfig.SDKSpec{},
		Provision: workshop.ProvisionSpec{
			Copy:    []workshop.CopyEntry{},
			Connect: []workshop.ConnectEntry{},
		},
	}
}

// FindAdditions returns the path to the additions file, or "" if none exists.
// Preference: workshop.my.yaml next to workshopYAMLPath, then
// $HOME/.config/my-workshop/my.yaml when HOME is set.
func FindAdditions(workshopYAMLPath string) string {
	dir := filepath.Dir(workshopYAMLPath)
	local := filepath.Join(dir, "workshop.my.yaml")
	if _, err := os.Stat(local); err == nil {
		return local
	}

	home := os.Getenv("HOME")
	if home != "" {
		global := filepath.Join(home, ".config", "my-workshop", "my.yaml")
		if _, err := os.Stat(global); err == nil {
			return global
		}
	}

	return ""
}

// ParseAdditions parses additions config text into a Config.
// Missing sections are filled with empty defaults.
func ParseAdditions(text string) Config {
	lines := splitKeepEnds(text)
	result := DefaultAdditions()

	// --- base ---
	result.Base = readScalar(lines, "base")

	// --- sdks ---
	for _, block := range yamlconfig.IterSDKBlocks(lines) {
		spec := yamlconfig.SDKSpec{Name: block.Name}
		for _, kind := range []string{"plugs", "slots"} {
			header, entryNames := yamlconfig.FindSubblock(lines, block.Start, block.End, kind)
			if header == -1 {
				continue
			}
			entries := scanSDKEntries(lines, header+1, block.End, entryNames)
			if kind == "plugs" {
				spec.Plugs = entries
			} else {
				spec.Slots = entries
			}
		}
		result.SDKs = append(result.SDKs, spec)
	}

	// --- provision ---
	if provStart, provEnd, ok := subblockRange(lines, 0, len(lines), "provision"); ok {
		if copyStart, copyEnd, ok := subblockRange(lines, provStart, provEnd, "copy"); ok {
			for _, item := range parseListItems(lines, copyStart, copyEnd, map[string]bool{"source": true, "target": true}) {
				result.Provision.Copy = append(result.Provision.Copy, workshop.CopyEntry{
					Source: item["source"],
					Target: item["target"],
				})
			}
		}
		if connStart, connEnd, ok := subblockRange(lines, provStart, provEnd, "connect"); ok {
			for _, item := range parseListItems(lines, connStart, connEnd, map[string]bool{"plug": true, "slot": true}) {
				result.Provision.Connect = append(result.Provision.Connect, workshop.ConnectEntry{
					Plug: item["plug"],
					Slot: item["slot"],
				})
			}
		}
	}

	return result
}

// LoadAdditions loads the additions config for the given workshop YAML path.
// The returned bool is false when no additions file is found; the Config is
// then the empty/noop config.
func LoadAdditions(workshopYAMLPath string) (Config, bool) {
	path := FindAdditions(workshopYAMLPath)
	if path == "" {
		return DefaultAdditions(), false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultAdditions(), false
	}
	return ParseAdditions(string(data)), true
}

// splitKeepEnds splits text into lines while keeping each trailing "\n".
// It mirrors Python's splitlines(keepends=True) for "\n"-delimited text.
func splitKeepEnds(text string) []string {
	var lines []string
	start := 0
	for i := range len(text) {
		if text[i] == '\n' {
			lines = append(lines, text[start:i+1])
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

// readScalar returns the value of a top-level "key: value" line, or "" when absent.
func readScalar(lines []string, key string) string {
	prefix := key + ":"
	for _, line := range lines {
		s := strings.TrimRight(line, "\r\n")
		if s == "" {
			continue
		}
		if strings.TrimLeft(s, " ") == s && strings.HasPrefix(s, prefix) {
			val := strings.TrimSpace(s[len(prefix):])
			if val == "" {
				return ""
			}
			return val
		}
	}
	return ""
}

// subblockRange finds (start, end) for "key:" between start and end.
// start is the line after the key header; end is one past the last line that
// belongs to the block. It returns ok=false when key is not found.
func subblockRange(lines []string, start, end int, key string) (int, int, bool) {
	for i := start; i < end; i++ {
		s := strings.TrimRight(lines[i], "\r\n")
		if s == "" {
			continue
		}
		indent := len(s) - len(strings.TrimLeft(s, " "))
		stripped := strings.TrimSpace(s)
		if stripped == key+":" {
			subStart := i + 1
			subEnd := subStart
			for j := subStart; j < end; j++ {
				sj := strings.TrimRight(lines[j], "\r\n")
				if sj == "" {
					subEnd = j + 1
					continue
				}
				if len(sj)-len(strings.TrimLeft(sj, " ")) > indent {
					subEnd = j + 1
				} else {
					break
				}
			}
			return subStart, subEnd, true
		}
	}
	return 0, 0, false
}

// scanSDKEntries collects named plug/slot entries in file order between start
// and end, using the membership set returned by yamlconfig.FindSubblock.
func scanSDKEntries(lines []string, start, end int, names map[string]bool) yamlconfig.Entries {
	var entries yamlconfig.Entries
	for i := start; i < end; i++ {
		s := strings.TrimRight(lines[i], "\r\n")
		if s == "" {
			continue
		}
		indent := len(s) - len(strings.TrimLeft(s, " "))
		if indent <= 4 {
			break
		}
		stripped := strings.TrimSpace(s)
		if indent == 6 && strings.HasSuffix(stripped, ":") {
			name := strings.TrimSuffix(stripped, ":")
			if names[name] {
				entries = append(entries, yamlconfig.Entry{
					Name:  name,
					Attrs: parseSubblockAttrs(lines, i),
				})
			}
		}
	}
	return entries
}

// parseSubblockAttrs parses "key: value" pairs indented directly under the
// header line at headerIdx. Attribute order follows file order.
func parseSubblockAttrs(lines []string, headerIdx int) yamlconfig.Attrs {
	var attrs yamlconfig.Attrs
	s := strings.TrimRight(lines[headerIdx], "\r\n")
	baseIndent := len(s) - len(strings.TrimLeft(s, " "))
	for j := headerIdx + 1; j < len(lines); j++ {
		sj := strings.TrimRight(lines[j], "\r\n")
		if sj == "" {
			continue
		}
		indent := len(sj) - len(strings.TrimLeft(sj, " "))
		if indent <= baseIndent {
			break
		}
		if indent == baseIndent+2 && strings.Contains(sj, ":") {
			k, v, _ := strings.Cut(strings.TrimSpace(sj), ":")
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			if k != "" && v != "" {
				attrs = append(attrs, yamlconfig.KV{Key: k, Value: v})
			}
		}
	}
	return attrs
}

// parseListItems yields dicts for YAML list items between start and end.
// Fields is the set of scalar field names to extract. Item order follows
// file order.
func parseListItems(lines []string, start, end int, fields map[string]bool) []map[string]string {
	var items []map[string]string
	baseIndent := -1
	for i := start; i < end; i++ {
		s := strings.TrimRight(lines[i], "\r\n")
		if s != "" {
			baseIndent = len(s) - len(strings.TrimLeft(s, " "))
			break
		}
	}
	if baseIndent == -1 {
		return items
	}

	listIndent := baseIndent
	fieldIndent := baseIndent + 2
	var item map[string]string

	for i := start; i < end; i++ {
		s := strings.TrimRight(lines[i], "\r\n")
		if s == "" {
			continue
		}
		indent := len(s) - len(strings.TrimLeft(s, " "))
		stripped := strings.TrimSpace(s)

		if indent == listIndent && strings.HasPrefix(stripped, "- ") {
			if item != nil {
				items = append(items, item)
			}
			item = make(map[string]string)
			rest := strings.TrimSpace(stripped[2:])
			if strings.Contains(rest, ":") {
				k, v, _ := strings.Cut(rest, ":")
				k = strings.TrimSpace(k)
				v = strings.TrimSpace(v)
				if fields[k] && v != "" {
					item[k] = v
				}
			}
		} else if indent == fieldIndent && item != nil && strings.Contains(stripped, ":") {
			k, v, _ := strings.Cut(stripped, ":")
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			if fields[k] && v != "" {
				item[k] = v
			}
		}
	}
	if item != nil {
		items = append(items, item)
	}
	return items
}
