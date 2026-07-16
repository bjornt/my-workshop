// Package workshop drives the `workshop` CLI launch flow.
package workshop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

// CopyEntry is a host-to-workshop copy specification.
type CopyEntry struct{ Source, Target string }

// ConnectEntry is a plug/slot connection specification.
type ConnectEntry struct{ Plug, Slot string }

// ProvisionSpec describes copies and connections to perform after launch.
type ProvisionSpec struct {
	Copy    []CopyEntry
	Connect []ConnectEntry
}

// Workshop is the backend abstraction used by the launch orchestration.
// Implementations may shell out to the real `workshop` binary or simulate one
// in memory.
type Workshop interface {
	Launch() error
	CopyTo(source, dest string) error
	Connect(plug, slot string) error
	Info() (string, bool) // (output, ok); ok=false == Python None
	Exec(cmd ...string) (string, error)
}

// Logger is the side-effect seam used to emit command trace lines.
type Logger func(string)

// DefaultLogger prints messages to stdout with a trailing newline.
func DefaultLogger(msg string) { fmt.Println(msg) }

// ParseHostname extracts the top-level 'hostname:' value from `workshop info` output.
// It skips indented lines and returns "" (Python None) when no top-level, non-empty
// hostname line is present.
func ParseHostname(info string) string {
	for _, line := range lines(info) {
		if len(line) > 0 && unicode.IsSpace(rune(line[0])) {
			continue
		}
		key, value, ok := partition(line, ":")
		if ok && strings.TrimSpace(key) == "hostname" && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// ParseWorkshopName extracts the top-level 'name:' value from `workshop info` output.
// It returns "" (Python None) when no top-level, non-empty name line is present.
func ParseWorkshopName(info string) string {
	for _, line := range lines(info) {
		if len(line) > 0 && unicode.IsSpace(rune(line[0])) {
			continue
		}
		key, value, ok := partition(line, ":")
		if ok && strings.TrimSpace(key) == "name" && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// ParseMountTarget extracts the workshop-target path for a named SDK mount.
// SDKs are expected at indent 2, mount names at indent 6, and workshop-target at
// indent 8. It returns "" (Python None) when the SDK or mount is not found.
func ParseMountTarget(info, sdk, mount string) string {
	currentSDK := ""
	currentMount := ""
	for _, line := range lines(info) {
		indent := len(line) - len(strings.TrimLeftFunc(line, unicode.IsSpace))
		stripped := strings.TrimSpace(line)
		switch {
		case indent == 2 && !strings.HasPrefix(stripped, "mounts:") &&
			!strings.HasPrefix(stripped, "tracking:") &&
			!strings.HasPrefix(stripped, "installed:"):
			currentSDK = strings.TrimSuffix(stripped, ":")
			currentMount = ""
		case indent == 6 && currentSDK == sdk:
			currentMount = strings.TrimSuffix(stripped, ":")
		case indent == 8 && currentMount == mount:
			key, value, ok := partition(stripped, ":")
			if ok && key == "workshop-target" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

// Hostname returns the best-effort hostname for a launched workshop.
// It prefers the DNS name from `workshop info` and falls back to the first IP
// address from `hostname -I` run inside the workshop.
func Hostname(ws Workshop) string {
	info, ok := ws.Info()
	return hostnameWith(ws, info, ok)
}

func hostnameWith(ws Workshop, info string, ok bool) string {
	if ok {
		if host := ParseHostname(info); host != "" {
			return host
		}
	}
	out, err := ws.Exec("hostname", "-I")
	if err != nil {
		return ""
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// Provision runs the launch/copy/connect sequence and returns the workshop's
// hostname once it is up.
func Provision(ws Workshop, spec ProvisionSpec) (string, error) {
	if err := ws.Launch(); err != nil {
		return "", err
	}
	info, ok := ws.Info()
	name := ""
	if ok {
		name = ParseWorkshopName(info)
	}

	for _, entry := range spec.Copy {
		source := expandUser(entry.Source)
		sdk, mount, _ := strings.Cut(entry.Target, ":")
		dest := ParseMountTarget(info, sdk, mount)
		if err := ws.CopyTo(source, dest); err != nil {
			return "", err
		}
	}

	for _, entry := range spec.Connect {
		plug := name + "/" + entry.Plug
		slot := name + "/" + entry.Slot
		if err := ws.Connect(plug, slot); err != nil {
			return "", err
		}
	}

	return hostnameWith(ws, info, ok), nil
}

// RealWorkshop is a Workshop implementation that shells out to the real
// `workshop` and `tar` binaries.
type RealWorkshop struct {
	log Logger
}

// NewRealWorkshop returns a RealWorkshop using the supplied logger.
// A nil logger defaults to DefaultLogger.
func NewRealWorkshop(log Logger) *RealWorkshop {
	if log == nil {
		log = DefaultLogger
	}
	return &RealWorkshop{log: log}
}

func (r *RealWorkshop) run(args ...string) error {
	r.log("+ workshop " + strings.Join(args, " "))
	return exec.Command("workshop", args...).Run()
}

// Launch runs `workshop launch` and waits for it to finish.
func (r *RealWorkshop) Launch() error { return r.run("launch") }

// Connect runs `workshop connect <plug> <slot>`.
func (r *RealWorkshop) Connect(plug, slot string) error { return r.run("connect", plug, slot) }

// Info returns the output of `workshop info`, or ("", false) on failure.
func (r *RealWorkshop) Info() (string, bool) {
	out, err := exec.Command("workshop", "info").Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// Exec runs a command inside the workshop and returns its stdout.
func (r *RealWorkshop) Exec(cmd ...string) (string, error) {
	args := append([]string{"exec"}, cmd...)
	out, err := exec.Command("workshop", args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// CopyTo copies a host directory into the workshop via a tar pipe.
func (r *RealWorkshop) CopyTo(source, dest string) error {
	r.log("+ tar -cf - -C " + source + " . | workshop exec -- tar -xf - -C " + dest)

	tarCmd := exec.Command("tar", "-cf", "-", "-C", source, ".")
	recvCmd := exec.Command("workshop", "exec", "--", "tar", "-xf", "-", "-C", dest)

	pipe, err := tarCmd.StdoutPipe()
	if err != nil {
		return err
	}
	recvCmd.Stdin = pipe

	if err := tarCmd.Start(); err != nil {
		return err
	}
	if err := recvCmd.Start(); err != nil {
		_ = tarCmd.Process.Kill()
		_ = tarCmd.Wait()
		return err
	}

	recvErr := recvCmd.Wait()
	tarErr := tarCmd.Wait()

	if tarErr != nil {
		return tarErr
	}
	if recvErr != nil {
		return recvErr
	}
	return nil
}

// expandUser mirrors Python's os.path.expanduser for leading `~` and `~/...`.
func expandUser(p string) string {
	if p == "~" {
		return homeDir()
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(homeDir(), p[2:])
	}
	return p
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

func lines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

func partition(s, sep string) (string, string, bool) {
	idx := strings.Index(s, sep)
	if idx == -1 {
		return s, "", false
	}
	return s[:idx], s[idx+len(sep):], true
}
