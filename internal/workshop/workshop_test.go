package workshop_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bjornt/my-workshop/internal/testsupport/fakeworkshop"
	"github.com/bjornt/my-workshop/internal/workshop"
)

// DEFAULT_PROVISION matches the built-in defaults for tests that exercise the
// full flow without caring about custom entries.
var defaultProvision = workshop.ProvisionSpec{
	Copy: []workshop.CopyEntry{
		{Source: "~/.omp", Target: "omp:omp-home"},
	},
	Connect: []workshop.ConnectEntry{
		{Plug: "omp:pi-auth-gateway", Slot: "system:pi-auth-gateway"},
	},
}

func TestParseHostname(t *testing.T) {
	tests := []struct {
		name string
		info string
		want string
	}{
		{
			name: "top_level_value",
			info: "name: dev\nbase: ubuntu@24.04\nhostname: dev-box\n",
			want: "dev-box",
		},
		{
			name: "absent",
			info: "name: dev\nbase: ubuntu@24.04\n",
			want: "",
		},
		{
			name: "indented_only_is_ignored",
			info: "name: dev\nsdks:\n  - name: try-omp\n    hostname: nope\n",
			want: "",
		},
		{
			name: "indented_never_shadows_top_level",
			info: "    hostname: nope\nhostname: real-box\n",
			want: "real-box",
		},
		{
			name: "blank_value",
			info: "name: dev\nhostname:\n",
			want: "",
		},
		{
			name: "whitespace_only_value",
			info: "name: dev\nhostname:    \n",
			want: "",
		},
		{
			name: "trailing_sdk_lines_ignored",
			info: "hostname: dev-box\nsdks:\n  - name: try-omp\n" +
				"    hostname: indented-detail-should-be-ignored\n",
			want: "dev-box",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := workshop.ParseHostname(tt.info); got != tt.want {
				t.Errorf("ParseHostname() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseWorkshopName(t *testing.T) {
	tests := []struct {
		name string
		info string
		want string
	}{
		{
			name: "top_level_value",
			info: "name: dev\nbase: ubuntu@24.04\nhostname: dev-box\n",
			want: "dev",
		},
		{
			name: "absent",
			info: "base: ubuntu@24.04\nhostname: dev-box\n",
			want: "",
		},
		{
			name: "indented_name_ignored",
			info: "name: dev\nsdks:\n  - name: try-omp\n",
			want: "dev",
		},
		{
			name: "indented_sdk_named_name_not_matched",
			info: "sdks:\n  - name: name\n",
			want: "",
		},
		{
			name: "extra_whitespace",
			info: "name:   my-workshop  \n",
			want: "my-workshop",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := workshop.ParseWorkshopName(tt.info); got != tt.want {
				t.Errorf("ParseWorkshopName() = %q, want %q", got, tt.want)
			}
		})
	}
}

const realInfo = `name: dev
base: ubuntu@24.04
hostname: dev-box
sdks:
  omp:
    mounts:
      omp-home:
        workshop-target: /home/workshop/.omp
  zed-remote:
    mounts:
      zed-server:
        workshop-target: /home/workshop/.zed_server
`

func TestParseMountTarget(t *testing.T) {
	tests := []struct {
		name  string
		sdk   string
		mount string
		want  string
	}{
		{
			name:  "extracts_workshop_target",
			sdk:   "omp",
			mount: "omp-home",
			want:  "/home/workshop/.omp",
		},
		{
			name:  "picks_correct_sdk_and_mount",
			sdk:   "zed-remote",
			mount: "zed-server",
			want:  "/home/workshop/.zed_server",
		},
		{
			name:  "returns_none_for_missing_sdk",
			sdk:   "nope",
			mount: "omp-home",
			want:  "",
		},
		{
			name:  "returns_none_for_missing_mount",
			sdk:   "omp",
			mount: "nope",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := workshop.ParseMountTarget(realInfo, tt.sdk, tt.mount); got != tt.want {
				t.Errorf("ParseMountTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProvisionRunsLifecycleOpsInOrder(t *testing.T) {
	fake := fakeworkshop.New(fakeworkshop.WithHostname("dev-box"))

	_, _ = workshop.Provision(fake, defaultProvision)

	ops := fake.Ops()
	if len(ops) < 4 {
		t.Fatalf("expected at least 4 ops, got %d: %v", len(ops), ops)
	}
	if ops[0] != "launch" || ops[1] != "info" || ops[2] != "copy_to" || ops[3] != "connect" {
		t.Errorf("ops[:4] = %v, want [launch info copy_to connect]", ops[:4])
	}
}

func TestProvisionCopiesEachSpec(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	spec := workshop.ProvisionSpec{
		Copy: []workshop.CopyEntry{
			{Source: "~/data", Target: "omp:omp-home"},
			{Source: "~/extra", Target: "zed-remote:zed-server"},
		},
		Connect: []workshop.ConnectEntry{},
	}
	fake := fakeworkshop.New(fakeworkshop.WithHostname("dev-box"))

	_, _ = workshop.Provision(fake, spec)

	want := [][2]string{
		{filepath.Join(home, "data"), "/home/workshop/.omp"},
		{filepath.Join(home, "extra"), "/home/workshop/.zed_server"},
	}
	if len(fake.Copies) != len(want) {
		t.Fatalf("Copies = %v, want %v", fake.Copies, want)
	}
	for i := range want {
		if fake.Copies[i] != want[i] {
			t.Errorf("Copies[%d] = %v, want %v", i, fake.Copies[i], want[i])
		}
	}
}

func TestProvisionConnectsEachSpecWithWorkshopPrefix(t *testing.T) {
	spec := workshop.ProvisionSpec{
		Copy: []workshop.CopyEntry{},
		Connect: []workshop.ConnectEntry{
			{Plug: "omp:pi-auth-gateway", Slot: "system:pi-auth-gateway"},
		},
	}
	fake := fakeworkshop.New(fakeworkshop.WithHostname("dev-box"))

	_, _ = workshop.Provision(fake, spec)

	want := [][2]string{
		{"dev/omp:pi-auth-gateway", "dev/system:pi-auth-gateway"},
	}
	if len(fake.Connections) != len(want) {
		t.Fatalf("Connections = %v, want %v", fake.Connections, want)
	}
	for i := range want {
		if fake.Connections[i] != want[i] {
			t.Errorf("Connections[%d] = %v, want %v", i, fake.Connections[i], want[i])
		}
	}
}

func TestProvisionAutodetectsWorkshopName(t *testing.T) {
	spec := workshop.ProvisionSpec{
		Copy: []workshop.CopyEntry{},
		Connect: []workshop.ConnectEntry{
			{Plug: "omp:pi-auth-gateway", Slot: "system:pi-auth-gateway"},
		},
	}
	fake := fakeworkshop.New(
		fakeworkshop.WithHostname("dev-box"),
		fakeworkshop.WithName("myws"),
	)

	_, _ = workshop.Provision(fake, spec)

	want := [][2]string{
		{"myws/omp:pi-auth-gateway", "myws/system:pi-auth-gateway"},
	}
	if len(fake.Connections) != len(want) {
		t.Fatalf("Connections = %v, want %v", fake.Connections, want)
	}
	for i := range want {
		if fake.Connections[i] != want[i] {
			t.Errorf("Connections[%d] = %v, want %v", i, fake.Connections[i], want[i])
		}
	}
}

func TestProvisionReturnsDNSHostnameAndSkipsExec(t *testing.T) {
	fake := fakeworkshop.New(fakeworkshop.WithHostname("dev-box"))

	result, err := workshop.Provision(fake, defaultProvision)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}

	if result != "dev-box" {
		t.Errorf("Provision() = %q, want %q", result, "dev-box")
	}
	for _, op := range fake.Ops() {
		if op == "exec" {
			t.Errorf("expected no exec ops, got %v", fake.Calls)
		}
	}
	for _, call := range fake.Calls {
		if len(call) > 0 && call[0] == "exec" {
			t.Errorf("expected no exec calls, got %v", call)
		}
	}
}

func TestHostnameFallsBackToFirstIP(t *testing.T) {
	tests := []struct {
		name string
		fake *fakeworkshop.FakeWorkshop
	}{
		{name: "no_dns_hostname", fake: fakeworkshop.New(fakeworkshop.WithHostname(""))},
		{name: "info_command_fails", fake: fakeworkshop.New(fakeworkshop.WithInfoFailing())},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := workshop.Hostname(tt.fake)

			if result != "10.0.0.5" {
				t.Errorf("Hostname() = %q, want %q", result, "10.0.0.5")
			}
			found := false
			for _, call := range tt.fake.Calls {
				if len(call) == 3 && call[0] == "exec" && call[1] == "hostname" && call[2] == "-I" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected (exec hostname -I) call, got %v", tt.fake.Calls)
			}
		})
	}
}

func TestProvisionFallsBackToIPWhenNoDNSHostname(t *testing.T) {
	fake := fakeworkshop.New(fakeworkshop.WithHostname(""))

	result, err := workshop.Provision(fake, defaultProvision)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}

	if result != "10.0.0.5" {
		t.Errorf("Provision() = %q, want %q", result, "10.0.0.5")
	}
	found := false
	for _, op := range fake.Ops() {
		if op == "exec" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected exec in ops, got %v", fake.Ops())
	}
}

func TestDefaultLogger(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	workshop.DefaultLogger("hello workshop")
	w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "hello workshop\n" {
		t.Errorf("DefaultLogger output = %q, want %q", got, "hello workshop\n")
	}
}

func installFakeWorkshop(t *testing.T, body string) {
	t.Helper()
	bindir := t.TempDir()
	path := filepath.Join(bindir, "workshop")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("write fake workshop: %v", err)
	}
	t.Setenv("PATH", bindir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestNewRealWorkshop_NilLoggerDefaultsToDefaultLogger(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	installFakeWorkshop(t, "case \"$1\" in\n  launch) exit 0 ;;\nesac\nexit 1")

	real := workshop.NewRealWorkshop(nil)
	if err := real.Launch(); err != nil {
		t.Fatalf("Launch: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "+ workshop launch\n" {
		t.Errorf("output = %q, want %q", got, "+ workshop launch\n")
	}
}

func TestRealWorkshop_Launch(t *testing.T) {
	installFakeWorkshop(t, "case \"$1\" in\n  launch) exit 0 ;;\nesac\nexit 1")

	var logs []string
	real := workshop.NewRealWorkshop(func(s string) { logs = append(logs, s) })
	if err := real.Launch(); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if len(logs) != 1 || logs[0] != "+ workshop launch" {
		t.Errorf("logs = %v, want [+ workshop launch]", logs)
	}
}

func TestRealWorkshop_LaunchError(t *testing.T) {
	installFakeWorkshop(t, "case \"$1\" in\n  launch) exit 1 ;;\nesac\nexit 0")

	real := workshop.NewRealWorkshop(func(string) {})
	if err := real.Launch(); err == nil {
		t.Fatal("expected Launch error")
	}
}

func TestRealWorkshop_Connect(t *testing.T) {
	installFakeWorkshop(t, "case \"$1\" in\n  connect) exit 0 ;;\nesac\nexit 1")

	var logs []string
	real := workshop.NewRealWorkshop(func(s string) { logs = append(logs, s) })
	if err := real.Connect("plug1", "slot1"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	want := "+ workshop connect plug1 slot1"
	if len(logs) != 1 || logs[0] != want {
		t.Errorf("logs = %v, want [%s]", logs, want)
	}
}

func TestRealWorkshop_Info(t *testing.T) {
	installFakeWorkshop(t, "case \"$1\" in\n  info) printf 'name: dev\\nhostname: fake-box\\n'; exit 0 ;;\nesac\nexit 1")

	real := workshop.NewRealWorkshop(func(string) {})
	out, ok := real.Info()
	if !ok {
		t.Fatal("Info returned ok=false")
	}
	if !strings.Contains(out, "hostname: fake-box") {
		t.Errorf("Info output = %q", out)
	}
}

func TestRealWorkshop_InfoError(t *testing.T) {
	installFakeWorkshop(t, "case \"$1\" in\n  info) exit 1 ;;\nesac\nexit 0")

	real := workshop.NewRealWorkshop(func(string) {})
	out, ok := real.Info()
	if ok {
		t.Fatal("expected ok=false")
	}
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

func TestRealWorkshop_Exec(t *testing.T) {
	installFakeWorkshop(t, "case \"$1\" in\n  exec) shift; exec \"$@\" ;;\nesac\nexit 1")

	real := workshop.NewRealWorkshop(func(string) {})
	out, err := real.Exec("echo", "workshop-output")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if strings.TrimSpace(out) != "workshop-output" {
		t.Errorf("Exec output = %q", out)
	}
}

func TestRealWorkshop_ExecError(t *testing.T) {
	installFakeWorkshop(t, "case \"$1\" in\n  exec) exit 42 ;;\nesac\nexit 1")

	real := workshop.NewRealWorkshop(func(string) {})
	_, err := real.Exec("anything")
	if err == nil {
		t.Fatal("expected Exec error")
	}
}

func TestRealWorkshop_CopyTo(t *testing.T) {
	installFakeWorkshop(t, "case \"$1\" in\n  exec) shift; if [ \"$1\" = \"--\" ]; then shift; fi; exec \"$@\" ;;\nesac\nexit 1")

	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "hello.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()

	var logs []string
	real := workshop.NewRealWorkshop(func(s string) { logs = append(logs, s) })
	if err := real.CopyTo(src, dst); err != nil {
		t.Fatalf("CopyTo: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "hello.txt"))
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if string(data) != "world" {
		t.Errorf("copied content = %q, want %q", data, "world")
	}
	if len(logs) != 1 || !strings.HasPrefix(logs[0], "+ tar") {
		t.Errorf("logs = %v, want tar command log", logs)
	}
}

func TestRealWorkshop_CopyToTarFails(t *testing.T) {
	installFakeWorkshop(t, "case \"$1\" in\n  exec) shift; if [ \"$1\" = \"--\" ]; then shift; fi; exec \"$@\" ;;\nesac\nexit 1")

	src := filepath.Join(t.TempDir(), "does-not-exist")
	dst := t.TempDir()

	real := workshop.NewRealWorkshop(func(string) {})
	if err := real.CopyTo(src, dst); err == nil {
		t.Fatal("expected CopyTo error for missing source")
	}
}

// Stub Workshop implementations for exercising error paths.

type errLaunchWorkshop struct{ fakeworkshop.FakeWorkshop }

func (e *errLaunchWorkshop) Launch() error { return os.ErrInvalid }

func TestProvisionReturnsLaunchError(t *testing.T) {
	_, err := workshop.Provision(&errLaunchWorkshop{}, defaultProvision)
	if err == nil {
		t.Fatal("expected error from Launch")
	}
}

type errCopyWorkshop struct{ fakeworkshop.FakeWorkshop }

func (e *errCopyWorkshop) Launch() error               { return nil }
func (e *errCopyWorkshop) Info() (string, bool)        { return fakeworkshop.New().Info() }
func (e *errCopyWorkshop) CopyTo(string, string) error { return os.ErrInvalid }

func TestProvisionReturnsCopyError(t *testing.T) {
	_, err := workshop.Provision(&errCopyWorkshop{}, defaultProvision)
	if err == nil {
		t.Fatal("expected error from CopyTo")
	}
}

type errConnectWorkshop struct{ fakeworkshop.FakeWorkshop }

func (e *errConnectWorkshop) Launch() error                { return nil }
func (e *errConnectWorkshop) Info() (string, bool)         { return fakeworkshop.New().Info() }
func (e *errConnectWorkshop) CopyTo(string, string) error  { return nil }
func (e *errConnectWorkshop) Connect(string, string) error { return os.ErrInvalid }

func TestProvisionReturnsConnectError(t *testing.T) {
	_, err := workshop.Provision(&errConnectWorkshop{}, defaultProvision)
	if err == nil {
		t.Fatal("expected error from Connect")
	}
}

type errExecWorkshop struct{ fakeworkshop.FakeWorkshop }

func (e *errExecWorkshop) Info() (string, bool)           { return "", false }
func (e *errExecWorkshop) Exec(...string) (string, error) { return "", os.ErrInvalid }

func TestHostnameReturnsEmptyWhenExecFails(t *testing.T) {
	if got := workshop.Hostname(&errExecWorkshop{}); got != "" {
		t.Errorf("Hostname = %q, want empty", got)
	}
}

type emptyExecWorkshop struct{ fakeworkshop.FakeWorkshop }

func (e *emptyExecWorkshop) Info() (string, bool)           { return "", false }
func (e *emptyExecWorkshop) Exec(...string) (string, error) { return "   ", nil }

func TestHostnameReturnsEmptyWhenNoIPFields(t *testing.T) {
	if got := workshop.Hostname(&emptyExecWorkshop{}); got != "" {
		t.Errorf("Hostname = %q, want empty", got)
	}
}
