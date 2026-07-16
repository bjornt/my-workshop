package yamlconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Logger is the side-effect seam used for observable status messages.
type Logger func(string)

// KV is an ordered attribute pair.
type KV struct{ Key, Value string }

// Attrs preserves insertion order of attribute pairs.
type Attrs []KV

// Entry is a named plug or slot with its attributes.
type Entry struct {
	Name  string
	Attrs Attrs
}

// Entries preserves insertion order of plug/slot entries.
type Entries []Entry

// Names returns a membership set of entry names.
func (e Entries) Names() map[string]bool {
	m := make(map[string]bool, len(e))
	for _, entry := range e {
		m[entry.Name] = true
	}
	return m
}

// SDKSpec describes a single SDK block to render or merge.
type SDKSpec struct {
	Name  string
	Plugs Entries
	Slots Entries
}

// SDKBlock bounds an SDK item in a line-oriented YAML file.
// End is the one-past-last line index.
type SDKBlock struct {
	Name       string
	Start, End int
}

// IterSDKBlocks returns each SDK item under the sdks: block.
// Start/End bound the item; End is one past its last line.
func IterSDKBlocks(lines []string) []SDKBlock {
	var blocks []SDKBlock
	inSdks := false
	var name string
	start := -1

	for i, raw := range lines {
		s := strings.TrimRight(raw, "\r\n")
		if !inSdks {
			if s == "sdks:" {
				inSdks = true
			}
			continue
		}

		if s != "" && !strings.HasPrefix(s, " ") {
			if start != -1 {
				blocks = append(blocks, SDKBlock{Name: name, Start: start, End: i})
				start = -1
			}
			inSdks = false
			continue
		}

		stripped := strings.TrimSpace(s)
		if strings.HasPrefix(s, "  -") && strings.HasPrefix(stripped, "- name:") {
			if start != -1 {
				blocks = append(blocks, SDKBlock{Name: name, Start: start, End: i})
			}
			parts := strings.SplitN(stripped, ":", 2)
			name = strings.TrimSpace(parts[1])
			start = i
		}
	}

	if start != -1 {
		blocks = append(blocks, SDKBlock{Name: name, Start: start, End: len(lines)})
	}
	return blocks
}

// SDKBounds returns the line bounds for the SDK item with the given name.
func SDKBounds(lines []string, name string) (int, int, bool) {
	for _, block := range IterSDKBlocks(lines) {
		if block.Name == name {
			return block.Start, block.End, true
		}
	}
	return 0, 0, false
}

// SDKsEnd returns the index at which to append a new SDK item.
func SDKsEnd(lines []string) int {
	end := len(lines)
	inSdks := false
	for i, raw := range lines {
		s := strings.TrimRight(raw, "\r\n")
		if !inSdks {
			if s == "sdks:" {
				inSdks = true
			}
			continue
		}
		if s != "" && !strings.HasPrefix(s, " ") {
			end = i
			break
		}
	}
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return end
}

// FindSubblock locates a 'plugs:' or 'slots:' sub-block within an SDK item.
// header is -1 when the sub-block is absent.
func FindSubblock(lines []string, start, end int, key string) (int, map[string]bool) {
	header := -1
	entries := make(map[string]bool)

	for i := start; i < end; i++ {
		s := strings.TrimRight(lines[i], "\r\n")
		if s == "" {
			continue
		}
		indent := len(s) - len(strings.TrimLeft(s, " "))
		stripped := strings.TrimSpace(s)

		if header == -1 {
			if indent == 4 && stripped == key+":" {
				header = i
			}
			continue
		}

		if indent <= 4 {
			break
		}
		if indent == 6 && strings.HasSuffix(stripped, ":") {
			entries[strings.TrimSuffix(stripped, ":")] = true
		}
	}

	return header, entries
}

// RenderEntry renders a single plug/slot entry at the requested indentation.
func RenderEntry(name string, attrs Attrs, indent int) string {
	pad := strings.Repeat(" ", indent)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s%s:\n", pad, name))
	for _, kv := range attrs {
		sb.WriteString(fmt.Sprintf("%s  %s: %s\n", pad, kv.Key, kv.Value))
	}
	return sb.String()
}

// RenderSDK renders a single SDK block.
func RenderSDK(spec SDKSpec) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  - name: %s\n", spec.Name))
	if len(spec.Plugs) > 0 {
		sb.WriteString("    plugs:\n")
		for _, entry := range spec.Plugs {
			sb.WriteString(RenderEntry(entry.Name, entry.Attrs, 6))
		}
	}
	if len(spec.Slots) > 0 {
		sb.WriteString("    slots:\n")
		for _, entry := range spec.Slots {
			sb.WriteString(RenderEntry(entry.Name, entry.Attrs, 6))
		}
	}
	return sb.String()
}

// RenderTemplate renders a complete workshop YAML file body.
func RenderTemplate(base string, sdks []SDKSpec) string {
	var sb strings.Builder
	for _, spec := range sdks {
		sb.WriteString(RenderSDK(spec))
	}
	return fmt.Sprintf("name: dev\nbase: %s\nsdks:\n%s", base, sb.String())
}

// AddMissing merges any missing kind (plugs/slots) entries into an existing SDK.
// It mutates *lines and returns true if modifications were made.
func AddMissing(lines *[]string, name, kind string, wanted Entries) bool {
	if len(wanted) == 0 {
		return false
	}

	start, end, ok := SDKBounds(*lines, name)
	if !ok {
		return false
	}

	header, existing := FindSubblock(*lines, start, end, kind)

	var missing []Entry
	for _, entry := range wanted {
		if !existing[entry.Name] {
			missing = append(missing, entry)
		}
	}
	if len(missing) == 0 {
		return false
	}

	var blob strings.Builder
	for _, entry := range missing {
		blob.WriteString(RenderEntry(entry.Name, entry.Attrs, 6))
	}

	var insertAt int
	var blobLines []string
	if header == -1 {
		last := start
		for i := start; i < end; i++ {
			if strings.TrimSpace((*lines)[i]) != "" {
				last = i
			}
		}
		ensureNewline(lines, last)
		insertAt = last + 1
		blobLines = splitKeepEnds(fmt.Sprintf("    %s:\n%s", kind, blob.String()))
	} else {
		insertAt = header + 1
		blobLines = splitKeepEnds(blob.String())
	}

	*lines = append((*lines)[:insertAt], append(blobLines, (*lines)[insertAt:]...)...)
	return true
}

// FindYAML locates the workshop YAML file to use.
// Preference: explicit path, then workshop.yaml, then a single file under
// .workshop/. Multiple .workshop/ candidates are reported as an error.
func FindYAML(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	if _, err := os.Stat("workshop.yaml"); err == nil {
		return "workshop.yaml", nil
	}

	var candidates []string
	for _, pattern := range []string{filepath.Join(".workshop", "*.yaml"), filepath.Join(".workshop", "*.yml")} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return "", err
		}
		candidates = append(candidates, matches...)
	}
	sort.Strings(candidates)

	if len(candidates) > 1 {
		return "", fmt.Errorf(
			"Multiple YAML files found in .workshop/: %s\nPass the path explicitly.",
			strings.Join(candidates, ", "),
		)
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	return "workshop.yaml", nil
}

// EnsureYAML creates path from the template, or merges in missing required SDKs.
// It logs via log and preserves exact bytes when nothing changes.
func EnsureYAML(path, base string, sdks []SDKSpec, log Logger) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(RenderTemplate(base, sdks)), 0644); err != nil {
			return err
		}
		log(fmt.Sprintf("Created %s", path))
		return nil
	} else if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := splitKeepEnds(string(data))

	present := make(map[string]bool)
	for _, block := range IterSDKBlocks(lines) {
		present[block.Name] = true
	}

	var added []string
	var merged []string

	for _, spec := range sdks {
		if !present[spec.Name] {
			insertAt := SDKsEnd(lines)
			ensureNewline(&lines, insertAt-1)
			sdkLines := splitKeepEnds(RenderSDK(spec))
			lines = append(lines[:insertAt], append(sdkLines, lines[insertAt:]...)...)
			added = append(added, spec.Name)
		} else {
			var fields []string
			if AddMissing(&lines, spec.Name, "plugs", spec.Plugs) {
				fields = append(fields, "plugs")
			}
			if AddMissing(&lines, spec.Name, "slots", spec.Slots) {
				fields = append(fields, "slots")
			}
			if len(fields) > 0 {
				merged = append(merged, fmt.Sprintf("%s (%s)", spec.Name, strings.Join(fields, "+")))
			}
		}
	}

	if len(added) == 0 && len(merged) == 0 {
		return nil
	}

	if err := os.WriteFile(path, []byte(joinLines(lines)), 0644); err != nil {
		return err
	}

	var parts []string
	if len(added) > 0 {
		parts = append(parts, "added "+strings.Join(added, ", "))
	}
	if len(merged) > 0 {
		parts = append(parts, "merged into "+strings.Join(merged, ", "))
	}
	log(fmt.Sprintf("Updated SDKs in %s: %s", path, strings.Join(parts, "; ")))
	return nil
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

// joinLines concatenates lines without adding separators; each line keeps its
// own trailing newline.
func joinLines(lines []string) string {
	return strings.Join(lines, "")
}

// ensureNewline guarantees that the line at index i ends with "\n".
func ensureNewline(lines *[]string, i int) {
	if 0 <= i && i < len(*lines) && !strings.HasSuffix((*lines)[i], "\n") {
		(*lines)[i] += "\n"
	}
}
