package contract

import (
	"regexp"
	"strings"
	"testing"
)

var (
	goStructRE  = regexp.MustCompile(`(?m)^type\s+([A-Za-z_][A-Za-z0-9_]*)\s+struct\s*\{`)
	goJSONTagRE = regexp.MustCompile("`json:\"([^\"]+)\"`")
)

func extractGoJSONStructFields(t *testing.T, filename string) map[string]map[string]bool {
	t.Helper()
	source := readFile(t, filename)
	structs := map[string]map[string]bool{}
	matches := goStructRE.FindAllStringSubmatchIndex(source, -1)
	for _, match := range matches {
		structName := source[match[2]:match[3]]
		openBrace := strings.LastIndex(source[match[0]:match[1]], "{")
		if openBrace < 0 {
			t.Fatalf("Go struct %s has no opening brace", structName)
		}
		openBrace += match[0]
		closeBrace := matchingBrace(source, openBrace)
		if closeBrace < 0 {
			t.Fatalf("Go struct %s has unmatched braces", structName)
		}
		fields := map[string]bool{}
		for _, tagMatch := range goJSONTagRE.FindAllStringSubmatch(source[openBrace+1:closeBrace], -1) {
			tag := strings.Split(tagMatch[1], ",")[0]
			if tag != "" && tag != "-" {
				fields[tag] = true
			}
		}
		structs[structName] = fields
	}
	return structs
}

func matchingBrace(source string, openIndex int) int {
	depth := 0
	var quote rune
	escaped := false
	for index, char := range source[openIndex:] {
		absolute := openIndex + index
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"', '`':
			quote = char
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return absolute
			}
		}
	}
	return -1
}
