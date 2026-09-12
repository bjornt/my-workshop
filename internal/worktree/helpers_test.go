package worktree

import (
	"reflect"
	"testing"
)

func TestExcludeLinesFromContent(t *testing.T) {
	if got := excludeLinesFromContent(""); got != nil {
		t.Errorf("empty content = %v, want nil", got)
	}
	if got := excludeLinesFromContent("a\nb\n"); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("two lines = %v, want [a b]", got)
	}
	if got := excludeLinesFromContent("a\nb"); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("no trailing newline = %v, want [a b]", got)
	}
}
