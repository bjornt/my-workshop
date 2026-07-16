package yamlconfig

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const defaultBase = "ubuntu@24.04"

var gateway = Attrs{
	{Key: "interface", Value: "tunnel"},
	{Key: "endpoint", Value: "localhost:4000"},
}

var requiredSDKs = []SDKSpec{
	{Name: "try-zed-remote"},
	{Name: "try-omp", Plugs: Entries{{Name: "pi-auth-gateway", Attrs: gateway}}},
	{Name: "system", Slots: Entries{{Name: "pi-auth-gateway", Attrs: gateway}}},
}

func linesOf(text string) []string {
	return splitKeepEnds(text)
}

func chdirTmp(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	})
	return tmp
}

func blocksEqual(a, b []SDKBlock) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func setsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// A multi-SDK document whose last item (gamma) runs to EOF; alpha owns a
// plugs: sub-block, so its bounds must span past the nested entries.
var multiEOF = ("name: dev\n" +
	"base: ubuntu@24.04\n" +
	"sdks:\n" +
	"  - name: alpha\n" +
	"    plugs:\n" +
	"      foo:\n" +
	"        interface: tunnel\n" +
	"  - name: beta\n" +
	"  - name: gamma\n")

// Same shape but a trailing top-level key (extra:) terminates the sdks section,
// so the final SDK must NOT extend to EOF.
var multiTrailingKey = ("name: dev\n" +
	"base: ubuntu@24.04\n" +
	"sdks:\n" +
	"  - name: alpha\n" +
	"  - name: beta\n" +
	"    slots:\n" +
	"      bar:\n" +
	"        interface: tunnel\n" +
	"extra: trailing\n")

func TestIterSDKBlocks_LastItemEndsAtEOF(t *testing.T) {
	lines := linesOf(multiEOF)
	blocks := IterSDKBlocks(lines)
	want := []SDKBlock{
		{Name: "alpha", Start: 3, End: 7},
		{Name: "beta", Start: 7, End: 8},
		{Name: "gamma", Start: 8, End: 9},
	}
	if !blocksEqual(blocks, want) {
		t.Fatalf("blocks = %v, want %v", blocks, want)
	}
	if blocks[len(blocks)-1].End != len(lines) {
		t.Fatalf("last block End = %d, want %d", blocks[len(blocks)-1].End, len(lines))
	}
}

func TestIterSDKBlocks_TrailingTopLevelKeyEndsSection(t *testing.T) {
	lines := linesOf(multiTrailingKey)
	blocks := IterSDKBlocks(lines)
	want := []SDKBlock{
		{Name: "alpha", Start: 3, End: 4},
		{Name: "beta", Start: 4, End: 8},
	}
	if !blocksEqual(blocks, want) {
		t.Fatalf("blocks = %v, want %v", blocks, want)
	}
	if lines[blocks[len(blocks)-1].End] != "extra: trailing\n" {
		t.Fatalf("line at section end = %q", lines[blocks[len(blocks)-1].End])
	}
}

func TestSDKBounds_PresentAndAbsent(t *testing.T) {
	lines := linesOf(multiEOF)

	s, e, ok := SDKBounds(lines, "beta")
	if !ok || s != 7 || e != 8 {
		t.Fatalf("beta bounds = (%d, %d, %v), want (7, 8, true)", s, e, ok)
	}

	s, e, ok = SDKBounds(lines, "alpha")
	if !ok || s != 3 || e != 7 {
		t.Fatalf("alpha bounds = (%d, %d, %v), want (3, 7, true)", s, e, ok)
	}

	_, _, ok = SDKBounds(lines, "nonexistent")
	if ok {
		t.Fatal("expected nonexistent SDK to be absent")
	}
}

func TestSDKsEnd_StopsBeforeTrailingTopLevelKey(t *testing.T) {
	lines := linesOf(multiTrailingKey)
	end := SDKsEnd(lines)
	if end != 8 {
		t.Fatalf("sdks_end = %d, want 8", end)
	}
	if lines[end] != "extra: trailing\n" {
		t.Fatalf("line at end = %q", lines[end])
	}
}

func TestSDKsEnd_SkipsTrailingBlankLines(t *testing.T) {
	lines := linesOf("name: dev\nsdks:\n  - name: alpha\n\n\n")
	end := SDKsEnd(lines)
	if end != 3 {
		t.Fatalf("sdks_end = %d, want 3", end)
	}
	if lines[end-1] != "  - name: alpha\n" {
		t.Fatalf("line before end = %q", lines[end-1])
	}
}

var sdkWithPlugs = ("  - name: try-omp\n" +
	"    plugs:\n" +
	"      pi-auth-gateway:\n" +
	"        interface: tunnel\n" +
	"      other-plug:\n" +
	"        interface: tunnel\n")

func TestFindSubblock_PresentExtractsEntryNames(t *testing.T) {
	lines := linesOf(sdkWithPlugs)
	header, entries := FindSubblock(lines, 0, len(lines), "plugs")
	if header != 1 {
		t.Fatalf("header = %d, want 1", header)
	}
	want := map[string]bool{"pi-auth-gateway": true, "other-plug": true}
	if !setsEqual(entries, want) {
		t.Fatalf("entries = %v, want %v", entries, want)
	}
}

func TestFindSubblock_AbsentReturnsNoneAndEmptySet(t *testing.T) {
	lines := linesOf(sdkWithPlugs)
	header, entries := FindSubblock(lines, 0, len(lines), "slots")
	if header != -1 {
		t.Fatalf("header = %d, want -1", header)
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %v, want empty", entries)
	}
}

func TestRenderTemplate_RoundTripsAllRequiredSDKs(t *testing.T) {
	text := RenderTemplate(defaultBase, requiredSDKs)
	wantPrefix := "name: dev\nbase: " + defaultBase + "\nsdks:\n"
	if !strings.HasPrefix(text, wantPrefix) {
		t.Fatalf("template prefix mismatch:\n%s", text)
	}
	names := []string{}
	for _, block := range IterSDKBlocks(linesOf(text)) {
		names = append(names, block.Name)
	}
	wantNames := []string{"try-zed-remote", "try-omp", "system"}
	if !slicesEqual(names, wantNames) {
		t.Fatalf("names = %v, want %v", names, wantNames)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var bareSDKs = ("name: dev\n" +
	"base: ubuntu@24.04\n" +
	"sdks:\n" +
	"  - name: try-zed-remote\n" +
	"  - name: try-omp\n" +
	"  - name: system\n")

func TestAddMissing_SplicesPlugIntoSDKLackingSubblock(t *testing.T) {
	lines := linesOf(bareSDKs)
	changed := AddMissing(&lines, "try-omp", "plugs", Entries{{Name: "pi-auth-gateway", Attrs: gateway}})
	if !changed {
		t.Fatal("expected lines to be modified")
	}

	start, end, ok := SDKBounds(lines, "try-omp")
	if !ok {
		t.Fatal("try-omp SDK missing after splice")
	}
	header, entries := FindSubblock(lines, start, end, "plugs")
	if header == -1 {
		t.Fatal("plugs sub-block missing after splice")
	}
	if !setsEqual(entries, map[string]bool{"pi-auth-gateway": true}) {
		t.Fatalf("entries = %v", entries)
	}

	joined := strings.Join(lines, "")
	for _, needle := range []string{
		"    plugs:\n",
		"      pi-auth-gateway:\n",
		"        interface: tunnel\n",
		"        endpoint: localhost:4000\n",
	} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("missing %q in:\n%s", needle, joined)
		}
	}

	names := []string{}
	for _, block := range IterSDKBlocks(lines) {
		names = append(names, block.Name)
	}
	if !slicesEqual(names, []string{"try-zed-remote", "try-omp", "system"}) {
		t.Fatalf("sdk order = %v", names)
	}
	if !strings.Contains(joined, "  - name: try-zed-remote\n") {
		t.Fatal("sibling try-zed-remote missing")
	}
	if !strings.Contains(joined, "  - name: system\n") {
		t.Fatal("sibling system missing")
	}
}

func TestAddMissing_AbsentSDKIsNoop(t *testing.T) {
	lines := linesOf(bareSDKs)
	before := append([]string(nil), lines...)
	changed := AddMissing(&lines, "no-such-sdk", "plugs", Entries{{Name: "x", Attrs: gateway}})
	if changed {
		t.Fatal("expected no change")
	}
	if !slicesEqual(lines, before) {
		t.Fatalf("lines changed:\n%v\n%v", lines, before)
	}
}

func TestAddMissing_EntryAlreadyPresentIsNoop(t *testing.T) {
	text := ("name: dev\n" +
		"sdks:\n" +
		"  - name: try-omp\n" +
		"    plugs:\n" +
		"      pi-auth-gateway:\n" +
		"        interface: tunnel\n")
	lines := linesOf(text)
	before := append([]string(nil), lines...)
	changed := AddMissing(&lines, "try-omp", "plugs", Entries{{Name: "pi-auth-gateway", Attrs: gateway}})
	if changed {
		t.Fatal("expected no change")
	}
	if !slicesEqual(lines, before) {
		t.Fatalf("lines changed:\n%v\n%v", lines, before)
	}
}

func TestFindYAML_HonoursExplicitArgument(t *testing.T) {
	_ = chdirTmp(t)
	if err := os.WriteFile("workshop.yaml", []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := FindYAML("chosen.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if got != "chosen.yaml" {
		t.Fatalf("got %q, want chosen.yaml", got)
	}
}

func TestFindYAML_PrefersCwdWorkshopYAML(t *testing.T) {
	tmp := chdirTmp(t)
	if err := os.WriteFile("workshop.yaml", []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	dot := filepath.Join(tmp, ".workshop")
	if err := os.MkdirAll(dot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dot, "other.yaml"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := FindYAML("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "workshop.yaml" {
		t.Fatalf("got %q, want workshop.yaml", got)
	}
}

func TestFindYAML_SingleDotworkshopCandidate(t *testing.T) {
	tmp := chdirTmp(t)
	dot := filepath.Join(tmp, ".workshop")
	if err := os.MkdirAll(dot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dot, "only.yaml"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := FindYAML("")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(".workshop", "only.yaml")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindYAML_MultipleDotworkshopCandidatesRaise(t *testing.T) {
	tmp := chdirTmp(t)
	dot := filepath.Join(tmp, ".workshop")
	if err := os.MkdirAll(dot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dot, "one.yaml"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dot, "two.yml"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := FindYAML("")
	if err == nil {
		t.Fatal("expected error for multiple candidates")
	}
}

func TestFindYAML_FallsBackWhenNothingExists(t *testing.T) {
	_ = chdirTmp(t)
	got, err := FindYAML("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "workshop.yaml" {
		t.Fatalf("got %q, want workshop.yaml", got)
	}
}

func TestEnsureYAML_CreatesFromTemplateAndLogs(t *testing.T) {
	tmp := chdirTmp(t)
	var log []string
	capture := func(s string) { log = append(log, s) }

	if err := EnsureYAML("workshop.yaml", defaultBase, requiredSDKs, capture); err != nil {
		t.Fatal(err)
	}
	if !slicesEqual(log, []string{"Created workshop.yaml"}) {
		t.Fatalf("log = %v", log)
	}
	data, err := os.ReadFile(filepath.Join(tmp, "workshop.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, block := range IterSDKBlocks(linesOf(string(data))) {
		names = append(names, block.Name)
	}
	want := []string{"try-zed-remote", "try-omp", "system"}
	if !slicesEqual(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
}

func TestEnsureYAML_IsIdempotent(t *testing.T) {
	tmp := chdirTmp(t)
	discard := func(string) {}
	if err := EnsureYAML("workshop.yaml", defaultBase, requiredSDKs, discard); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(tmp, "workshop.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	var log []string
	capture := func(s string) { log = append(log, s) }
	if err := EnsureYAML("workshop.yaml", defaultBase, requiredSDKs, capture); err != nil {
		t.Fatal(err)
	}
	if len(log) != 0 {
		t.Fatalf("expected silent re-run, got log = %v", log)
	}
	after, err := os.ReadFile(filepath.Join(tmp, "workshop.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("bytes changed on re-run")
	}
}

func TestEnsureYAML_MergesMissingPlugPreservingLines(t *testing.T) {
	tmp := chdirTmp(t)
	hand := ("name: dev\n" +
		"base: ubuntu@24.04\n" +
		"# keep this comment\n" +
		"sdks:\n" +
		"  - name: try-zed-remote\n" +
		"  - name: try-omp\n" +
		"  - name: system\n" +
		"    slots:\n" +
		"      pi-auth-gateway:\n" +
		"        interface: tunnel\n" +
		"        endpoint: localhost:4000\n")
	if err := os.WriteFile(filepath.Join(tmp, "hand.yaml"), []byte(hand), 0644); err != nil {
		t.Fatal(err)
	}

	var log []string
	capture := func(s string) { log = append(log, s) }
	if err := EnsureYAML("hand.yaml", defaultBase, requiredSDKs, capture); err != nil {
		t.Fatal(err)
	}
	if !slicesEqual(log, []string{"Updated SDKs in hand.yaml: merged into try-omp (plugs)"}) {
		t.Fatalf("log = %v", log)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "hand.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, "# keep this comment\n") {
		t.Fatal("comment was not preserved")
	}
	if !strings.Contains(out, "  - name: try-zed-remote\n") {
		t.Fatal("try-zed-remote line was not preserved")
	}
	start, end, ok := SDKBounds(linesOf(out), "try-omp")
	if !ok {
		t.Fatal("try-omp SDK missing after merge")
	}
	header, entries := FindSubblock(linesOf(out), start, end, "plugs")
	if header == -1 {
		t.Fatal("try-omp plugs sub-block missing")
	}
	if !setsEqual(entries, map[string]bool{"pi-auth-gateway": true}) {
		t.Fatalf("entries = %v", entries)
	}

	var log2 []string
	capture2 := func(s string) { log2 = append(log2, s) }
	if err := EnsureYAML("hand.yaml", defaultBase, requiredSDKs, capture2); err != nil {
		t.Fatal(err)
	}
	if len(log2) != 0 {
		t.Fatalf("expected silent re-run, got log = %v", log2)
	}
}

func TestEnsureYAML_FullySpecifiedFileUntouched(t *testing.T) {
	tmp := chdirTmp(t)
	text := RenderTemplate(defaultBase, requiredSDKs)
	if err := os.WriteFile(filepath.Join(tmp, "full.yaml"), []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(tmp, "full.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	var log []string
	capture := func(s string) { log = append(log, s) }
	if err := EnsureYAML("full.yaml", defaultBase, requiredSDKs, capture); err != nil {
		t.Fatal(err)
	}
	if len(log) != 0 {
		t.Fatalf("expected silent run, got log = %v", log)
	}
	after, err := os.ReadFile(filepath.Join(tmp, "full.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("fully specified file changed")
	}
}

func TestEnsureYAML_CustomSDKsMerged(t *testing.T) {
	tmp := chdirTmp(t)
	custom := []SDKSpec{
		{Name: "alpha"},
		{Name: "beta", Plugs: Entries{{Name: "my-plug", Attrs: Attrs{{Key: "interface", Value: "tunnel"}}}}},
	}
	var log []string
	capture := func(s string) { log = append(log, s) }
	if err := EnsureYAML("workshop.yaml", defaultBase, custom, capture); err != nil {
		t.Fatal(err)
	}
	if !slicesEqual(log, []string{"Created workshop.yaml"}) {
		t.Fatalf("log = %v", log)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "workshop.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, block := range IterSDKBlocks(linesOf(string(data))) {
		names = append(names, block.Name)
	}
	if !slicesEqual(names, []string{"alpha", "beta"}) {
		t.Fatalf("names = %v", names)
	}
	if !strings.Contains(string(data), "my-plug") {
		t.Fatal("my-plug missing from output")
	}
}

func TestEnsureYAML_EmptySDKsCreatesMinimalTemplate(t *testing.T) {
	tmp := chdirTmp(t)
	var log []string
	capture := func(s string) { log = append(log, s) }
	if err := EnsureYAML("workshop.yaml", defaultBase, []SDKSpec{}, capture); err != nil {
		t.Fatal(err)
	}
	if !slicesEqual(log, []string{"Created workshop.yaml"}) {
		t.Fatalf("log = %v", log)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "workshop.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sdks:\n") {
		t.Fatal("sdks header missing")
	}
	names := []string{}
	for _, block := range IterSDKBlocks(linesOf(string(data))) {
		names = append(names, block.Name)
	}
	if len(names) != 0 {
		t.Fatalf("expected no SDKs, got %v", names)
	}
}

func TestEnsureYAML_EmptySDKsNoopOnExisting(t *testing.T) {
	tmp := chdirTmp(t)
	text := RenderTemplate(defaultBase, requiredSDKs)
	if err := os.WriteFile(filepath.Join(tmp, "workshop.yaml"), []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(tmp, "workshop.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	var log []string
	capture := func(s string) { log = append(log, s) }
	if err := EnsureYAML("workshop.yaml", defaultBase, []SDKSpec{}, capture); err != nil {
		t.Fatal(err)
	}
	if len(log) != 0 {
		t.Fatalf("expected silent run, got log = %v", log)
	}
	after, err := os.ReadFile(filepath.Join(tmp, "workshop.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("file changed with empty SDK list")
	}
}
