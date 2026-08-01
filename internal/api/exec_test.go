package api

import (
	"errors"
	"slices"
	"testing"
)

func TestParseCommand(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"Empty", "", nil},
		{"OnlySpaces", "   ", nil},
		{"Simple", "GET mykey", []string{"GET", "mykey"}},
		{"CollapsesRuns", "GET    mykey", []string{"GET", "mykey"}},
		{"LeadingAndTrailingSpace", "  PING  ", []string{"PING"}},
		{"QuotedValue", `SET key "hello world"`, []string{"SET", "key", "hello world"}},
		{"QuotedEmpty", `SET key ""`, []string{"SET", "key"}},
		{"AdjacentQuotes", `SET key"suffix" x`, []string{"SET", "keysuffix", "x"}},
		{"EscapedQuote", `SET key "say \"hi\""`, []string{"SET", "key", `say "hi"`}},
		{"EscapedBackslash", `SET key "a\\b"`, []string{"SET", "key", `a\b`}},
		{"EscapedNewline", `SET key "a\nb"`, []string{"SET", "key", "a\nb"}},
		{"EscapedTab", `SET key "a\tb"`, []string{"SET", "key", "a\tb"}},
		{"UnknownEscapeIsKept", `SET key "a\qb"`, []string{"SET", "key", `a\qb`}},
		{"UnterminatedQuote", `SET key "unclosed`, []string{"SET", "key", "unclosed"}},
		{"BackslashOutsideQuotes", `SET key a\b`, []string{"SET", "key", `a\b`}},
		{"Unicode", "SET ключ значение", []string{"SET", "ключ", "значение"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCommand(tc.input)
			if !slices.Equal(got, tc.want) {
				t.Errorf("parseCommand(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestFormatResult(t *testing.T) {
	cases := []struct {
		name      string
		input     any
		wantType  string
		wantValue any
	}{
		{"Nil", nil, "nil", nil},
		{"String", "hello", "string", "hello"},
		{"EmptyString", "", "string", ""},
		{"Integer", int64(42), "integer", int64(42)},
		{"Float", 1.5, "string", 1.5},
		{"True", true, "integer", int64(1)},
		{"False", false, "nil", nil},
		{"Error", errors.New("boom"), "error", "boom"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatResult(tc.input)
			if got["type"] != tc.wantType {
				t.Errorf("type = %v, want %v", got["type"], tc.wantType)
			}
			if got["value"] != tc.wantValue {
				t.Errorf("value = %v, want %v", got["value"], tc.wantValue)
			}
		})
	}

	t.Run("Array", func(t *testing.T) {
		got := formatResult([]any{"a", int64(1), nil})
		if got["type"] != "array" {
			t.Fatalf("type = %v, want array", got["type"])
		}
		items, ok := got["value"].([]map[string]any)
		if !ok {
			t.Fatalf("value = %T, want a slice of formatted items", got["value"])
		}
		if len(items) != 3 {
			t.Fatalf("got %d items, want 3", len(items))
		}
		if items[0]["type"] != "string" || items[1]["type"] != "integer" || items[2]["type"] != "nil" {
			t.Errorf("item types = %v, %v, %v, want string, integer, nil",
				items[0]["type"], items[1]["type"], items[2]["type"])
		}
	})

	t.Run("NestedArray", func(t *testing.T) {
		got := formatResult([]any{[]any{"inner"}})
		items := got["value"].([]map[string]any)
		if items[0]["type"] != "array" {
			t.Errorf("nested type = %v, want array", items[0]["type"])
		}
	})

	t.Run("EmptyArray", func(t *testing.T) {
		got := formatResult([]any{})
		if got["type"] != "array" {
			t.Errorf("type = %v, want array", got["type"])
		}
		if items := got["value"].([]map[string]any); len(items) != 0 {
			t.Errorf("got %d items, want none", len(items))
		}
	})
}

func TestKeyPositions(t *testing.T) {
	cases := []struct {
		name     string
		cmd      string
		argCount int
		want     []int
	}{
		{"NoArgs", "GET", 1, nil},
		{"SingleKey", "GET", 2, []int{1}},
		{"SingleKeyWithValue", "SET", 3, []int{1}},
		{"AdminCommandHasNoKeys", "INFO", 2, nil},
		{"ConfigHasNoKeys", "CONFIG", 3, nil},
		{"MemoryHasNoKeys", "MEMORY", 3, nil},
		{"MultiKeyMGET", "MGET", 4, []int{1, 2, 3}},
		{"MultiKeyDEL", "DEL", 3, []int{1, 2}},
		{"RenameChecksBoth", "RENAME", 3, []int{1, 2}},
		{"RenameWithOneArg", "RENAME", 2, []int{1}},
		{"RenameBare", "RENAME", 1, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := keyPositions(tc.cmd, tc.argCount)
			if !slices.Equal(got, tc.want) {
				t.Errorf("keyPositions(%q, %d) = %v, want %v", tc.cmd, tc.argCount, got, tc.want)
			}
		})
	}
}

func TestCheckPrefixArgs(t *testing.T) {
	const prefix = "app:"

	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"MatchingKey", []string{"GET", "app:key"}, true},
		{"NonMatchingKey", []string{"GET", "other:key"}, false},
		{"NoKeyArgs", []string{"PING"}, true},
		{"AdminCommand", []string{"INFO", "memory"}, true},
		{"ConfigGet", []string{"CONFIG", "GET", "maxmemory"}, true},
		{"AllKeysMatch", []string{"MGET", "app:a", "app:b"}, true},
		{"OneKeyOutside", []string{"MGET", "app:a", "other:b"}, false},
		{"RenameBothInside", []string{"RENAME", "app:a", "app:b"}, true},
		{"RenameDestinationOutside", []string{"RENAME", "app:a", "other:b"}, false},
		{"RenameSourceOutside", []string{"RENAME", "other:a", "app:b"}, false},
		{"PrefixIsNotASubstringMatch", []string{"GET", "not-app:key"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := checkPrefixArgs(tc.args[0], tc.args, prefix); got != tc.want {
				t.Errorf("checkPrefixArgs(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestCommandTablesAreConsistent(t *testing.T) {
	// A command listed as read-only must not also be unconditionally blocked,
	// or read-only mode would advertise something the console always refuses.
	for cmd := range readOnlyCommands {
		if blockedCommands[cmd] {
			t.Errorf("%s is both read-only and blocked", cmd)
		}
	}

	// Every command with a read-only subcommand table must itself be read-only,
	// otherwise the subcommand refinement is unreachable.
	for cmd := range readOnlySubcommands {
		if !readOnlyCommands[cmd] {
			t.Errorf("%s has read-only subcommands but is not a read-only command", cmd)
		}
	}
}
