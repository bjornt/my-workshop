package additions_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bjornt/my-workshop/internal/additions"
	"github.com/bjornt/my-workshop/internal/testsupport/gitenv"
	"github.com/bjornt/my-workshop/internal/yamlconfig"
)

func TestDefaultAdditions_isEmpty(t *testing.T) {
	got := additions.DefaultAdditions()
	if got.Base != "" || len(got.SDKs) != 0 || len(got.Provision.Copy) != 0 || len(got.Provision.Connect) != 0 {
		t.Fatalf("DefaultAdditions not empty: %+v", got)
	}
}

func TestParseAdditions_fullConfig(t *testing.T) {
	const text = "base: alpine@3.20\n" +
		"\n" +
		"sdks:\n" +
		"  - name: my-sdk\n" +
		"    plugs:\n" +
		"      my-plug:\n" +
		"        interface: tunnel\n" +
		"        endpoint: localhost:5000\n" +
		"  - name: other-sdk\n" +
		"    slots:\n" +
		"      my-slot:\n" +
		"        interface: tunnel\n" +
		"        endpoint: localhost:6000\n" +
		"\n" +
		"provision:\n" +
		"  copy:\n" +
		"    - source: ~/data\n" +
		"      target: my-sdk:data-mount\n" +
		"    - source: ~/extra\n" +
		"      target: other-sdk:extra-mount\n" +
		"  connect:\n" +
		"    - plug: my-sdk:my-plug\n" +
		"      slot: other-sdk:my-slot\n"

	got := additions.ParseAdditions(text)

	if got.Base != "alpine@3.20" {
		t.Errorf("base = %q, want alpine@3.20", got.Base)
	}

	if len(got.SDKs) != 2 {
		t.Fatalf("len(sdks) = %d, want 2", len(got.SDKs))
	}
	if got.SDKs[0].Name != "my-sdk" {
		t.Errorf("sdks[0].name = %q, want my-sdk", got.SDKs[0].Name)
	}
	plugAttrs := entryAttrs(t, got.SDKs[0].Plugs, "my-plug")
	if attrValue(t, plugAttrs, "interface") != "tunnel" {
		t.Errorf("my-plug interface = %q, want tunnel", attrValue(t, plugAttrs, "interface"))
	}
	if attrValue(t, plugAttrs, "endpoint") != "localhost:5000" {
		t.Errorf("my-plug endpoint = %q, want localhost:5000", attrValue(t, plugAttrs, "endpoint"))
	}

	if got.SDKs[1].Name != "other-sdk" {
		t.Errorf("sdks[1].name = %q, want other-sdk", got.SDKs[1].Name)
	}
	slotAttrs := entryAttrs(t, got.SDKs[1].Slots, "my-slot")
	if attrValue(t, slotAttrs, "interface") != "tunnel" {
		t.Errorf("my-slot interface = %q, want tunnel", attrValue(t, slotAttrs, "interface"))
	}

	if len(got.Provision.Copy) != 2 {
		t.Fatalf("len(provision.copy) = %d, want 2", len(got.Provision.Copy))
	}
	if got.Provision.Copy[0].Source != "~/data" || got.Provision.Copy[0].Target != "my-sdk:data-mount" {
		t.Errorf("copy[0] = %+v, want {Source:~/data Target:my-sdk:data-mount}", got.Provision.Copy[0])
	}
	if got.Provision.Copy[1].Source != "~/extra" || got.Provision.Copy[1].Target != "other-sdk:extra-mount" {
		t.Errorf("copy[1] = %+v, want {Source:~/extra Target:other-sdk:extra-mount}", got.Provision.Copy[1])
	}

	if len(got.Provision.Connect) != 1 {
		t.Fatalf("len(provision.connect) = %d, want 1", len(got.Provision.Connect))
	}
	if got.Provision.Connect[0].Plug != "my-sdk:my-plug" || got.Provision.Connect[0].Slot != "other-sdk:my-slot" {
		t.Errorf("connect[0] = %+v, want {Plug:my-sdk:my-plug Slot:other-sdk:my-slot}", got.Provision.Connect[0])
	}
}

func TestParseAdditions_sdksOnly(t *testing.T) {
	const text = "sdks:\n" +
		"  - name: alpha\n" +
		"  - name: beta\n"

	got := additions.ParseAdditions(text)

	if got.Base != "" {
		t.Errorf("base = %q, want empty", got.Base)
	}
	if len(got.SDKs) != 2 || got.SDKs[0].Name != "alpha" || got.SDKs[1].Name != "beta" {
		t.Fatalf("sdks = %+v, want alpha,beta", got.SDKs)
	}
	if len(got.Provision.Copy) != 0 || len(got.Provision.Connect) != 0 {
		t.Errorf("provision not empty: %+v", got.Provision)
	}
}

func TestParseAdditions_provisionOnly(t *testing.T) {
	const text = "provision:\n" +
		"  copy:\n" +
		"    - source: ~/x\n" +
		"      target: sdk:mount\n"

	got := additions.ParseAdditions(text)

	if got.Base != "" {
		t.Errorf("base = %q, want empty", got.Base)
	}
	if len(got.SDKs) != 0 {
		t.Errorf("sdks = %+v, want empty", got.SDKs)
	}
	if len(got.Provision.Copy) != 1 || got.Provision.Copy[0].Source != "~/x" || got.Provision.Copy[0].Target != "sdk:mount" {
		t.Errorf("copy = %+v, want [{~/x sdk:mount}]", got.Provision.Copy)
	}
	if len(got.Provision.Connect) != 0 {
		t.Errorf("connect = %+v, want empty", got.Provision.Connect)
	}
}

func TestParseAdditions_baseOnly(t *testing.T) {
	got := additions.ParseAdditions("base: debian@12\n")

	if got.Base != "debian@12" {
		t.Errorf("base = %q, want debian@12", got.Base)
	}
	if len(got.SDKs) != 0 {
		t.Errorf("sdks = %+v, want empty", got.SDKs)
	}
	if len(got.Provision.Copy) != 0 {
		t.Errorf("copy = %+v, want empty", got.Provision.Copy)
	}
}

func TestParseAdditions_emptyFile(t *testing.T) {
	got := additions.ParseAdditions("")

	if got.Base != "" {
		t.Errorf("base = %q, want empty", got.Base)
	}
	if len(got.SDKs) != 0 {
		t.Errorf("sdks = %+v, want empty", got.SDKs)
	}
	if len(got.Provision.Copy) != 0 || len(got.Provision.Connect) != 0 {
		t.Errorf("provision not empty: %+v", got.Provision)
	}
}

func TestParseAdditions_whitespaceOnly(t *testing.T) {
	got := additions.ParseAdditions("\n\n  \n")

	if got.Base != "" {
		t.Errorf("base = %q, want empty", got.Base)
	}
	if len(got.SDKs) != 0 {
		t.Errorf("sdks = %+v, want empty", got.SDKs)
	}
	if len(got.Provision.Copy) != 0 {
		t.Errorf("copy = %+v, want empty", got.Provision.Copy)
	}
}

func TestParseAdditions_malformedIgnored(t *testing.T) {
	const text = "not: valid: yaml: at all\n---\n  broken\n"
	got := additions.ParseAdditions(text)

	if got.Base != "" {
		t.Errorf("base = %q, want empty for malformed input", got.Base)
	}
}

func TestFindAdditions_localWins(t *testing.T) {
	dir := gitenv.NewTmp(t)

	localPath := filepath.Join(dir, "workshop.my.yaml")
	if err := os.WriteFile(localPath, []byte("base: x\n"), 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}

	globalDir := filepath.Join(dir, ".config", "my-workshop")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("create global dir: %v", err)
	}
	globalPath := filepath.Join(globalDir, "my.yaml")
	if err := os.WriteFile(globalPath, []byte("base: y\n"), 0o644); err != nil {
		t.Fatalf("write global file: %v", err)
	}

	t.Setenv("HOME", dir)

	workshopPath := filepath.Join(dir, "workshop.yaml")
	got := additions.FindAdditions(workshopPath)
	if got != localPath {
		t.Errorf("FindAdditions = %q, want %q", got, localPath)
	}
}

func TestFindAdditions_globalFallback(t *testing.T) {
	dir := gitenv.NewTmp(t)

	globalDir := filepath.Join(dir, ".config", "my-workshop")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("create global dir: %v", err)
	}
	globalPath := filepath.Join(globalDir, "my.yaml")
	if err := os.WriteFile(globalPath, []byte("base: z\n"), 0o644); err != nil {
		t.Fatalf("write global file: %v", err)
	}

	t.Setenv("HOME", dir)

	workshopPath := filepath.Join(dir, "workshop.yaml")
	got := additions.FindAdditions(workshopPath)
	if got != globalPath {
		t.Errorf("FindAdditions = %q, want %q", got, globalPath)
	}
}

func TestFindAdditions_neitherReturnsEmpty(t *testing.T) {
	dir := gitenv.NewTmp(t)
	t.Setenv("HOME", dir)

	workshopPath := filepath.Join(dir, "workshop.yaml")
	got := additions.FindAdditions(workshopPath)
	if got != "" {
		t.Errorf("FindAdditions = %q, want empty", got)
	}
}

func TestFindAdditions_localInSubdir(t *testing.T) {
	dir := gitenv.NewTmp(t)

	subdir := filepath.Join(dir, ".workshop")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	localPath := filepath.Join(subdir, "workshop.my.yaml")
	if err := os.WriteFile(localPath, []byte("base: x\n"), 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}

	workshopPath := filepath.Join(subdir, "foo.yaml")
	got := additions.FindAdditions(workshopPath)
	if got != localPath {
		t.Errorf("FindAdditions = %q, want %q", got, localPath)
	}
}

func TestLoadAdditions_noFileReturnsEmpty(t *testing.T) {
	dir := gitenv.NewTmp(t)
	t.Setenv("HOME", dir)

	workshopPath := filepath.Join(dir, "workshop.yaml")
	cfg, ok := additions.LoadAdditions(workshopPath)
	if ok {
		t.Errorf("LoadAdditions ok = true, want false")
	}
	if cfg.Base != "" || len(cfg.SDKs) != 0 || len(cfg.Provision.Copy) != 0 || len(cfg.Provision.Connect) != 0 {
		t.Fatalf("LoadAdditions not empty: %+v", cfg)
	}
}

func TestLoadAdditions_readsLocalFile(t *testing.T) {
	dir := gitenv.NewTmp(t)

	localPath := filepath.Join(dir, "workshop.my.yaml")
	content := "base: custom-img\nsdks:\n  - name: solo\n"
	if err := os.WriteFile(localPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}

	workshopPath := filepath.Join(dir, "workshop.yaml")
	cfg, ok := additions.LoadAdditions(workshopPath)
	if !ok {
		t.Fatalf("LoadAdditions ok = false, want true")
	}
	if cfg.Base != "custom-img" {
		t.Errorf("base = %q, want custom-img", cfg.Base)
	}
	if len(cfg.SDKs) != 1 || cfg.SDKs[0].Name != "solo" {
		t.Errorf("sdks = %+v, want [{Name:solo}]", cfg.SDKs)
	}
}

// entryAttrs returns the attrs of the named entry, failing the test if absent.
func entryAttrs(t *testing.T, entries yamlconfig.Entries, name string) yamlconfig.Attrs {
	t.Helper()
	for _, e := range entries {
		if e.Name == name {
			return e.Attrs
		}
	}
	t.Fatalf("entry %q not found in %+v", name, entries)
	return nil
}

// attrValue returns the value of the named attribute, failing the test if absent.
func attrValue(t *testing.T, attrs yamlconfig.Attrs, key string) string {
	t.Helper()
	for _, kv := range attrs {
		if kv.Key == key {
			return kv.Value
		}
	}
	t.Fatalf("attribute %q not found in %+v", key, attrs)
	return ""
}
