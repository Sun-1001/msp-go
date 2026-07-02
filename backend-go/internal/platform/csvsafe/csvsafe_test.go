package csvsafe

import "testing"

func TestCellEscapesFormulaPrefixes(t *testing.T) {
	cases := map[string]string{
		"=cmd|'/C calc'!A0": "'=cmd|'/C calc'!A0",
		"+SUM(1,2)":         "'+SUM(1,2)",
		"-10+20":            "'-10+20",
		"@HYPERLINK(\"x\")": "'@HYPERLINK(\"x\")",
		" \t=leading-space": "' \t=leading-space",
		"normal":            "normal",
		"2026-07-01":        "2026-07-01",
		"":                  "",
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := Cell(input); got != want {
				t.Fatalf("Cell(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestRowEscapesAllFields(t *testing.T) {
	row := Row("alice", "=evil", "student")
	if row[0] != "alice" || row[1] != "'=evil" || row[2] != "student" {
		t.Fatalf("row = %#v", row)
	}
}
