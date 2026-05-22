package util

import (
	"testing"
)

func TestTruncateTitle(t *testing.T) {
	t.Run("short string unchanged", func(t *testing.T) {
		got := TruncateTitle("hello")
		if got != "hello" {
			t.Errorf("got %q, want %q", got, "hello")
		}
	})

	t.Run("exactly 60 runes unchanged", func(t *testing.T) {
		in := string(make([]rune, 60))
		got := TruncateTitle(in)
		if got != in {
			t.Errorf("got len %d, want 60", len(got))
		}
	})

	t.Run("truncated with ellipsis", func(t *testing.T) {
		in := string(make([]rune, 70))
		got := TruncateTitle(in)
		if len(got) != 63 {
			t.Errorf("got len %d, want 63", len(got))
		}
		if got[60:] != "..." {
			t.Errorf("got suffix %q, want %q", got[60:], "...")
		}
	})

	t.Run("unicode runes counted correctly", func(t *testing.T) {
		in := "日本語あいうえおかきくけこさしすせそたちつてとなにぬねのはひふへほ"
		got := TruncateTitle(in)
		if len(got) > len(in) {
			t.Errorf("truncated string longer than input")
		}
	})

	t.Run("empty string unchanged", func(t *testing.T) {
		got := TruncateTitle("")
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestToJSON(t *testing.T) {
	t.Run("struct serialized correctly", func(t *testing.T) {
		v := struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}{"test", 30}
		got := ToJSON(v)
		want := `{"name":"test","age":30}`
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("nil returns empty object", func(t *testing.T) {
		got := ToJSON(nil)
		if got != "null" {
			t.Errorf("got %q, want %q", got, "null")
		}
	})
}
