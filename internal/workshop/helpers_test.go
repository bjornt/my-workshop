package workshop

import (
	"path/filepath"
	"testing"
)

func TestExpandUser(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got := expandUser("~"); got != home {
		t.Errorf("expandUser(~) = %q, want %q", got, home)
	}
	wantFoo := filepath.Join(home, "foo")
	if got := expandUser("~/foo"); got != wantFoo {
		t.Errorf("expandUser(~/foo) = %q, want %q", got, wantFoo)
	}
	if got := expandUser("/bar/baz"); got != "/bar/baz" {
		t.Errorf("expandUser(/bar/baz) = %q, want /bar/baz", got)
	}
}

func TestHomeDir_prefersHomeEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got := homeDir(); got != home {
		t.Errorf("homeDir() = %q, want %q", got, home)
	}
}

func TestHomeDir_fallsBackToUserHomeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	withHome := homeDir()

	t.Setenv("HOME", "")
	withoutHome := homeDir()

	if withHome != home {
		t.Fatalf("homeDir with HOME = %q, want %q", withHome, home)
	}
	if withoutHome == withHome {
		t.Errorf("homeDir ignored empty HOME and returned %q", withoutHome)
	}
}
