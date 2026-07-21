package pkg

import "testing"

func TestBoolP(t *testing.T) {
	got := BoolP(true)
	if got == nil || *got != true {
		t.Fatalf("BoolP() = %v, want true pointer", got)
	}
}

func TestTypedSlice(t *testing.T) {
	got := TypedSlice[string]([]any{"a", "b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("TypedSlice() = %#v, want %#v", got, []string{"a", "b"})
	}
}

func TestTypedSlicePanicsOnWrongType(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("TypedSlice() did not panic on incompatible type")
		}
	}()

	_ = TypedSlice[string]([]any{1})
}
