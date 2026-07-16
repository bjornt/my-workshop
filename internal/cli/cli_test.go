package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/bjornt/my-workshop/internal/testsupport/fakeworkshop"
	"github.com/bjornt/my-workshop/internal/testsupport/gitenv"
	"github.com/bjornt/my-workshop/internal/yamlconfig"
)

func logger() (yamlconfig.Logger, func() []string) {
	var logs []string
	return yamlconfig.Logger(func(msg string) { logs = append(logs, msg) }),
		func() []string { return logs }
}

func TestParseArgsMapsBaseAndPositional(t *testing.T) {
	args, err := ParseArgs([]string{"--base", "custom:img", "target.yaml"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if args.Base != "custom:img" {
		t.Errorf("Base = %q, want %q", args.Base, "custom:img")
	}
	if args.YAML != "target.yaml" {
		t.Errorf("YAML = %q, want %q", args.YAML, "target.yaml")
	}
}

func TestParseArgsRevertIsABoolSwitch(t *testing.T) {
	args, err := ParseArgs([]string{"--revert", "target.yaml"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if !args.Revert {
		t.Errorf("Revert = false, want true")
	}
	if args.YAML != "target.yaml" {
		t.Errorf("YAML = %q, want %q", args.YAML, "target.yaml")
	}
}

func TestParseArgsRevertAfterPositional(t *testing.T) {
	args, err := ParseArgs([]string{"target.yaml", "--revert"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if !args.Revert {
		t.Errorf("Revert = false, want true when flag follows positional")
	}
	if args.YAML != "target.yaml" {
		t.Errorf("YAML = %q, want %q", args.YAML, "target.yaml")
	}
}

func TestMainNoAdditionsIsNoopOnProvision(t *testing.T) {
	repo := gitenv.NewRepo(t)
	log, logs := logger()
	fake := fakeworkshop.New(fakeworkshop.WithHostname("dev-box"))

	if err := Run([]string{"workshop.yaml"}, fake, log); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.Stat("workshop.yaml"); err != nil {
		t.Fatalf("workshop.yaml should exist: %v", err)
	}

	exclude := gitenv.ExcludeLines(repo)
	if !slices.Contains(exclude, "/workshop.yaml") {
		t.Errorf("exclude lines should contain /workshop.yaml, got %v", exclude)
	}

	ops := fake.Ops()
	if len(ops) < 2 || ops[0] != "launch" || ops[1] != "info" {
		t.Errorf("ops[:2] = %v, want [launch info]", ops)
	}
	if len(fake.Copies) != 0 {
		t.Errorf("Copies = %v, want empty", fake.Copies)
	}
	if len(fake.Connections) != 0 {
		t.Errorf("Connections = %v, want empty", fake.Connections)
	}

	out := strings.Join(logs(), "\n")
	if !strings.Contains(out, "No additions config found") {
		t.Errorf("log should contain 'No additions config found', got:\n%s", out)
	}
	if !strings.Contains(out, "ssh workshop@") {
		t.Errorf("log should contain 'ssh workshop@', got:\n%s", out)
	}
}

func TestMainFallsBackToIPWhenNoHostname(t *testing.T) {
	gitenv.NewRepo(t)
	log, logs := logger()

	if err := Run([]string{"workshop.yaml"}, fakeworkshop.New(), log); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := strings.Join(logs(), "\n")
	if !strings.Contains(out, "ssh workshop@10.0.0.5") {
		t.Errorf("log should contain 'ssh workshop@10.0.0.5', got:\n%s", out)
	}
}

func TestMainRevertDoesNotProvision(t *testing.T) {
	repo := gitenv.NewRepo(t)
	setupLog, _ := logger()
	setupFake := fakeworkshop.New(fakeworkshop.WithHostname("dev-box"))
	if err := Run([]string{"workshop.yaml"}, setupFake, setupLog); err != nil {
		t.Fatalf("setup Run: %v", err)
	}
	if !slices.Contains(gitenv.ExcludeLines(repo), "/workshop.yaml") {
		t.Fatalf("setup run should hide workshop.yaml")
	}

	log, logs := logger()
	fake := fakeworkshop.New(fakeworkshop.WithHostname("dev-box"))
	if err := Run([]string{"workshop.yaml", "--revert"}, fake, log); err != nil {
		t.Fatalf("revert Run: %v", err)
	}

	if len(fake.Calls) != 0 {
		t.Errorf("fake.Calls = %v, want empty (backend untouched)", fake.Calls)
	}

	out := strings.Join(logs(), "\n")
	if strings.Contains(out, "ssh workshop@") {
		t.Errorf("revert should not print ssh hint, got:\n%s", out)
	}
	if slices.Contains(gitenv.ExcludeLines(repo), "/workshop.yaml") {
		t.Errorf("workshop.yaml should be unhidden after revert")
	}
}

func TestMainRevertUnhidesAdditionsFileToo(t *testing.T) {
	repo := gitenv.NewRepo(t)
	if err := os.WriteFile("workshop.my.yaml", []byte("base: x\n"), 0o644); err != nil {
		t.Fatalf("write additions: %v", err)
	}

	setupLog, _ := logger()
	setupFake := fakeworkshop.New(fakeworkshop.WithHostname("dev-box"))
	if err := Run([]string{"workshop.yaml"}, setupFake, setupLog); err != nil {
		t.Fatalf("setup Run: %v", err)
	}
	exclude := gitenv.ExcludeLines(repo)
	if !slices.Contains(exclude, "/workshop.yaml") || !slices.Contains(exclude, "/workshop.my.yaml") {
		t.Fatalf("setup should hide both YAML files, got %v", exclude)
	}

	log, _ := logger()
	if err := Run([]string{"workshop.yaml", "--revert"}, fakeworkshop.New(), log); err != nil {
		t.Fatalf("revert Run: %v", err)
	}

	exclude = gitenv.ExcludeLines(repo)
	if slices.Contains(exclude, "/workshop.yaml") {
		t.Errorf("workshop.yaml should be unhidden after revert")
	}
	if slices.Contains(exclude, "/workshop.my.yaml") {
		t.Errorf("workshop.my.yaml should be unhidden after revert")
	}
}

func TestMainHonoursExplicitPath(t *testing.T) {
	gitenv.NewRepo(t)
	log, _ := logger()

	if err := Run([]string{"custom.yaml"}, fakeworkshop.New(fakeworkshop.WithHostname("dev-box")), log); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.Stat("custom.yaml"); err != nil {
		t.Errorf("custom.yaml should exist: %v", err)
	}
	if _, err := os.Stat("workshop.yaml"); !os.IsNotExist(err) {
		t.Errorf("workshop.yaml should not exist")
	}
}

func TestMainWithLocalAdditionsUsesCustomConfig(t *testing.T) {
	repo := gitenv.NewRepo(t)
	additions := "base: alpine@3.20\n" +
		"sdks:\n" +
		"  - name: custom-sdk\n" +
		"    plugs:\n" +
		"      my-plug:\n" +
		"        interface: tunnel\n" +
		"        endpoint: localhost:9000\n" +
		"\n" +
		"provision:\n" +
		"  copy:\n" +
		"    - source: ~/mydata\n" +
		"      target: omp:omp-home\n" +
		"  connect:\n" +
		"    - plug: omp:pi-auth-gateway\n" +
		"      slot: system:pi-auth-gateway\n"
	if err := os.WriteFile("workshop.my.yaml", []byte(additions), 0o644); err != nil {
		t.Fatalf("write additions: %v", err)
	}

	fake := fakeworkshop.New(fakeworkshop.WithHostname("dev-box"))
	log, logs := logger()

	if err := Run([]string{"workshop.yaml"}, fake, log); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := strings.Join(logs(), "\n")
	if !strings.Contains(out, "Using additions config") {
		t.Errorf("log should contain 'Using additions config', got:\n%s", out)
	}
	if !strings.Contains(out, "workshop.my.yaml") {
		t.Errorf("log should contain 'workshop.my.yaml', got:\n%s", out)
	}

	exclude := gitenv.ExcludeLines(repo)
	if !slices.Contains(exclude, "/workshop.yaml") {
		t.Errorf("workshop.yaml should be excluded, got %v", exclude)
	}
	if !slices.Contains(exclude, "/workshop.my.yaml") {
		t.Errorf("workshop.my.yaml should be excluded, got %v", exclude)
	}

	text, err := os.ReadFile("workshop.yaml")
	if err != nil {
		t.Fatalf("read workshop.yaml: %v", err)
	}
	yaml := string(text)
	if !strings.Contains(yaml, "custom-sdk") {
		t.Errorf("workshop.yaml should contain custom-sdk, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "base: alpine@3.20") {
		t.Errorf("workshop.yaml should contain alpine base, got:\n%s", yaml)
	}

	// Mirror Python's os.path.expanduser for the source path.
	wantSource := filepath.Join(os.Getenv("HOME"), "mydata")
	wantCopies := [][2]string{{wantSource, "/home/workshop/.omp"}}
	if !reflect.DeepEqual(fake.Copies, wantCopies) {
		t.Errorf("Copies = %v, want %v", fake.Copies, wantCopies)
	}

	wantConnections := [][2]string{{"dev/omp:pi-auth-gateway", "dev/system:pi-auth-gateway"}}
	if !reflect.DeepEqual(fake.Connections, wantConnections) {
		t.Errorf("Connections = %v, want %v", fake.Connections, wantConnections)
	}
}
