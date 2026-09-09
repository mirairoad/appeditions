package ai

import "testing"

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name  string
		reply string
		want  string
	}{
		{"bare array", `[{"id":"a"}]`, `[{"id":"a"}]`},
		{"fenced", "```json\n[{\"id\":\"a\"}]\n```", `[{"id":"a"}]`},
		{"preamble", "Here you go:\n\n[{\"id\":\"a\"}]", `[{"id":"a"}]`},
		// A bracket inside a string must not close the value early — which is
		// what a headline like "Lists [and] more" would otherwise do.
		{"bracket in string", `[{"headline":"Lists ] and ["}]`, `[{"headline":"Lists ] and ["}]`},
		{"object", `{"headline":"x"}`, `{"headline":"x"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractJSON(tc.reply)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractJSONRejectsProse(t *testing.T) {
	if _, err := extractJSON("I cannot help with that."); err == nil {
		t.Error("prose should not parse as JSON")
	}
}

func TestUnknownProviderIsNamed(t *testing.T) {
	_, err := Runner{Provider: "gemini"}.Run(t.Context(), "hi")
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); got != `unknown provider "gemini"` {
		t.Errorf("error is %q", got)
	}
}
