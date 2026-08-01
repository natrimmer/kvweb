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

func TestScanMatchPattern(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
		ok   bool
	}{
		{"NoOptions", []string{"SCAN", "0"}, "", false},
		{"Match", []string{"SCAN", "0", "MATCH", "app:*"}, "app:*", true},
		{"Lowercase", []string{"SCAN", "0", "match", "app:*"}, "app:*", true},
		{"AfterCount", []string{"SCAN", "0", "COUNT", "10", "MATCH", "app:*"}, "app:*", true},
		{"BeforeCount", []string{"SCAN", "0", "MATCH", "app:*", "COUNT", "10"}, "app:*", true},
		{"WithType", []string{"SCAN", "0", "MATCH", "app:*", "TYPE", "string"}, "app:*", true},
		{"DanglingMatch", []string{"SCAN", "0", "MATCH"}, "", false},
		// An option value reading "MATCH" sits at an odd offset, so it is skipped.
		{"ValueNamedMatch", []string{"SCAN", "0", "TYPE", "MATCH", "COUNT", "10"}, "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := scanMatchPattern(tc.args)
			if got != tc.want || ok != tc.ok {
				t.Errorf("scanMatchPattern(%v) = %q, %v, want %q, %v", tc.args, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestPatternWithinPrefix(t *testing.T) {
	const prefix = "app:"

	cases := []struct {
		name    string
		pattern string
		want    bool
	}{
		{"PrefixWildcard", "app:*", true},
		{"DeeperWildcard", "app:user:*", true},
		{"ExactKey", "app:key", true},
		{"QuestionMark", "app:key?", true},
		{"MatchEverything", "*", false},
		{"OtherNamespace", "other:*", false},
		{"WildcardInsideThePrefix", "ap*", false},
		{"CharacterClassHead", "[a]pp:*", false},
		{"EscapeHead", `\app:*`, false},
		{"PrefixIsNotASubstringMatch", "not-app:*", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := patternWithinPrefix(tc.pattern, prefix); got != tc.want {
				t.Errorf("patternWithinPrefix(%q, %q) = %v, want %v", tc.pattern, prefix, got, tc.want)
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
