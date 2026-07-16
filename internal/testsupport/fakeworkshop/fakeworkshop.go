// Package fakeworkshop provides an in-memory Workshop implementation for tests.
package fakeworkshop

import (
	"strings"

	"github.com/bjornt/my-workshop/internal/workshop"
)

// FakeWorkshop simulates the real `workshop` CLI in memory.
type FakeWorkshop struct {
	Calls       [][]string
	Copies      [][2]string
	Connections [][2]string
	Launched    bool
	Hostname    string
	IP          string
	Name        string
	InfoOK      bool
}

// Option configures a FakeWorkshop at construction time.
type Option func(*FakeWorkshop)

// WithHostname sets the DNS name reported by Info.
func WithHostname(hostname string) Option { return func(f *FakeWorkshop) { f.Hostname = hostname } }

// WithIP sets the first address returned by Exec("hostname", "-I").
func WithIP(ip string) Option { return func(f *FakeWorkshop) { f.IP = ip } }

// WithName sets the workshop name reported by Info.
func WithName(name string) Option { return func(f *FakeWorkshop) { f.Name = name } }

// WithInfoFailing causes Info to return ("", false).
func WithInfoFailing() Option { return func(f *FakeWorkshop) { f.InfoOK = false } }

// New builds a FakeWorkshop with the contract-specified defaults.
func New(opts ...Option) *FakeWorkshop {
	f := &FakeWorkshop{
		IP:     "10.0.0.5",
		Name:   "dev",
		InfoOK: true,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Compile-time assertion that FakeWorkshop implements workshop.Workshop.
var _ workshop.Workshop = (*FakeWorkshop)(nil)

// Launch records a launch operation.
func (f *FakeWorkshop) Launch() error {
	f.Calls = append(f.Calls, []string{"launch"})
	f.Launched = true
	return nil
}

// CopyTo records a copy operation.
func (f *FakeWorkshop) CopyTo(source, dest string) error {
	f.Calls = append(f.Calls, []string{"copy_to", source, dest})
	f.Copies = append(f.Copies, [2]string{source, dest})
	return nil
}

// Connect records a connect operation.
func (f *FakeWorkshop) Connect(plug, slot string) error {
	f.Calls = append(f.Calls, []string{"connect", plug, slot})
	f.Connections = append(f.Connections, [2]string{plug, slot})
	return nil
}

// Info returns simulated `workshop info` output, or ("", false) when InfoOK is false.
func (f *FakeWorkshop) Info() (string, bool) {
	f.Calls = append(f.Calls, []string{"info"})
	if !f.InfoOK {
		return "", false
	}
	return f.infoOutput(), true
}

// Exec returns canned command output.
func (f *FakeWorkshop) Exec(cmd ...string) (string, error) {
	call := append([]string{"exec"}, cmd...)
	f.Calls = append(f.Calls, call)
	if len(cmd) == 2 && cmd[0] == "hostname" && cmd[1] == "-I" {
		return f.IP + " 192.168.0.1 \n", nil
	}
	return "", nil
}

// Ops returns the first element of each recorded call.
func (f *FakeWorkshop) Ops() []string {
	ops := make([]string, len(f.Calls))
	for i, c := range f.Calls {
		if len(c) > 0 {
			ops[i] = c[0]
		}
	}
	return ops
}

func (f *FakeWorkshop) infoOutput() string {
	var b strings.Builder
	b.WriteString("name: " + f.Name + "\n")
	b.WriteString("base: ubuntu@24.04\n")
	if f.Hostname != "" {
		b.WriteString("hostname: " + f.Hostname + "\n")
	}
	b.WriteString("sdks:\n")
	b.WriteString("  omp:\n")
	b.WriteString("    mounts:\n")
	b.WriteString("      omp-home:\n")
	b.WriteString("        workshop-target: /home/workshop/.omp\n")
	b.WriteString("  zed-remote:\n")
	b.WriteString("    mounts:\n")
	b.WriteString("      zed-server:\n")
	b.WriteString("        workshop-target: /home/workshop/.zed_server\n")
	return b.String()
}
