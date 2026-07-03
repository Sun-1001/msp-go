package contract

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type frontendEndpoint struct {
	Method string
	Path   string
	File   string
	Line   int
}

type goRoutePattern struct {
	Method  string
	Path    string
	Subtree bool
}

var (
	httpClientCallRE       = regexp.MustCompile(`\b(apiClient|axios)\.(get|post|put|patch|delete)\b`)
	fetchCallRE            = regexp.MustCompile(`\bfetch\s*\(`)
	createSSECallRE        = regexp.MustCompile(`\bcreateSSEConnection\s*\(`)
	methodPropertyRE       = regexp.MustCompile(`(?s)\bmethod\s*:\s*["']([A-Za-z]+)["']`)
	stringConstantRE       = regexp.MustCompile(`(?m)\bconst\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*["']([^"']+)["']\s*;`)
	remoteEndpointRE       = regexp.MustCompile(`\bremoteEndpoint\s*:\s*["']([^"']+)["']`)
	templateIdentifierRE   = regexp.MustCompile(`\$\{([A-Za-z_$][A-Za-z0-9_$]*)\}`)
	templateExpressionRE   = regexp.MustCompile(`\$\{[^}]+\}`)
	frontendPathTemplateRE = regexp.MustCompile(`\{[^}/]*\}`)
)

func TestFrontendAPICallsAreCoveredByGoOrExplicitlyClassified(t *testing.T) {
	root := repoRoot(t)
	endpoints := collectFrontendEndpoints(t, root)
	patterns := collectGoRoutePatterns(t, root)
	exceptions := frontendRouteExceptions()
	usedExceptions := map[string]bool{}

	uncovered := []string{}
	for _, endpoint := range endpoints {
		if goRouteCovered(endpoint, patterns) {
			continue
		}
		key := endpoint.Method + " " + endpoint.Path
		if _, ok := exceptions[key]; ok {
			usedExceptions[key] = true
			continue
		}
		uncovered = append(uncovered, fmt.Sprintf("%s (%s:%d)", key, endpoint.File, endpoint.Line))
	}

	if len(uncovered) > 0 {
		t.Fatalf("frontend API calls without Go coverage or explicit classification:\n%s", strings.Join(uncovered, "\n"))
	}

	staleExceptions := []string{}
	for key := range exceptions {
		if !usedExceptions[key] {
			staleExceptions = append(staleExceptions, key)
		}
	}
	sort.Strings(staleExceptions)
	if len(staleExceptions) > 0 {
		t.Fatalf("stale frontend API route classifications; remove or update these entries:\n%s", strings.Join(staleExceptions, "\n"))
	}
}

func frontendRouteExceptions() map[string]string {
	return map[string]string{}
}

func collectFrontendEndpoints(t *testing.T, root string) []frontendEndpoint {
	t.Helper()
	sourceRoot := filepath.Join(root, "frontend", "src")
	endpointsByKey := map[string]frontendEndpoint{}
	err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".tsx") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if isFrontendTestSource(rel) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, endpoint := range extractFrontendEndpointsFromSource(string(content), rel) {
			key := endpoint.Method + " " + endpoint.Path
			if _, exists := endpointsByKey[key]; !exists {
				endpointsByKey[key] = endpoint
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk frontend source: %v", err)
	}

	endpoints := make([]frontendEndpoint, 0, len(endpointsByKey))
	for _, endpoint := range endpointsByKey {
		endpoints = append(endpoints, endpoint)
	}
	sort.Slice(endpoints, func(i, j int) bool {
		left := endpoints[i].Method + " " + endpoints[i].Path
		right := endpoints[j].Method + " " + endpoints[j].Path
		return left < right
	})
	return endpoints
}

func isFrontendTestSource(path string) bool {
	base := filepath.Base(path)
	return strings.Contains(path, "/__tests__/") ||
		strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".test.tsx") ||
		strings.HasSuffix(base, ".spec.ts") ||
		strings.HasSuffix(base, ".spec.tsx")
}

func extractFrontendEndpointsFromSource(source string, filename string) []frontendEndpoint {
	stripped := stripTypeScriptComments(source)
	constants := frontendStringConstants(stripped)
	endpoints := []frontendEndpoint{}

	addEndpoint := func(method, path string, offset int) {
		normalized, ok := normalizeFrontendAPIPath(path)
		if !ok {
			return
		}
		endpoints = append(endpoints, frontendEndpoint{
			Method: strings.ToUpper(method),
			Path:   normalized,
			File:   filename,
			Line:   lineNumber(stripped, offset),
		})
	}

	for _, match := range httpClientCallRE.FindAllStringSubmatchIndex(stripped, -1) {
		method := stripped[match[4]:match[5]]
		open := findNextOpenParen(stripped, match[1])
		if open < 0 {
			continue
		}
		path, ok := parseFirstArgumentPath(stripped, open, constants)
		if ok {
			addEndpoint(method, path, match[0])
		}
	}

	for _, match := range fetchCallRE.FindAllStringIndex(stripped, -1) {
		open := strings.LastIndex(stripped[match[0]:match[1]], "(")
		if open < 0 {
			continue
		}
		open += match[0]
		path, ok := parseFirstArgumentPath(stripped, open, constants)
		if !ok {
			continue
		}
		method := "GET"
		if close := matchingParen(stripped, open); close > open {
			if methodMatch := methodPropertyRE.FindStringSubmatch(stripped[open+1 : close]); len(methodMatch) == 2 {
				method = methodMatch[1]
			}
		}
		addEndpoint(method, path, match[0])
	}

	for _, match := range createSSECallRE.FindAllStringIndex(stripped, -1) {
		open := strings.LastIndex(stripped[match[0]:match[1]], "(")
		if open < 0 {
			continue
		}
		open += match[0]
		path, ok := parseFirstArgumentPath(stripped, open, constants)
		if ok {
			addEndpoint(http.MethodPost, path, match[0])
		}
	}

	for _, match := range remoteEndpointRE.FindAllStringSubmatchIndex(stripped, -1) {
		addEndpoint(http.MethodPost, stripped[match[2]:match[3]], match[0])
	}

	return endpoints
}

func frontendStringConstants(source string) map[string]string {
	constants := map[string]string{}
	for _, match := range stringConstantRE.FindAllStringSubmatch(source, -1) {
		constants[match[1]] = match[2]
	}
	return constants
}

func parseFirstArgumentPath(source string, open int, constants map[string]string) (string, bool) {
	index := open + 1
	for index < len(source) && isWhitespace(source[index]) {
		index++
	}
	if index >= len(source) {
		return "", false
	}
	switch source[index] {
	case '\'', '"', '`':
		value, ok := parseStringLikeLiteral(source, index)
		if !ok {
			return "", false
		}
		if source[index] == '`' {
			value = interpolateTemplate(value, constants)
		}
		return value, true
	default:
		if !isIdentifierStart(source[index]) {
			return "", false
		}
		start := index
		index++
		for index < len(source) && isIdentifierContinue(source[index]) {
			index++
		}
		value, ok := constants[source[start:index]]
		return value, ok
	}
}

func parseStringLikeLiteral(source string, start int) (string, bool) {
	quote := source[start]
	var builder strings.Builder
	escaped := false
	for index := start + 1; index < len(source); index++ {
		char := source[index]
		if escaped {
			builder.WriteByte(char)
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if char == quote {
			return builder.String(), true
		}
		builder.WriteByte(char)
	}
	return "", false
}

func interpolateTemplate(value string, constants map[string]string) string {
	value = templateIdentifierRE.ReplaceAllStringFunc(value, func(match string) string {
		submatches := templateIdentifierRE.FindStringSubmatch(match)
		if len(submatches) == 2 {
			if constantValue, ok := constants[submatches[1]]; ok {
				return constantValue
			}
		}
		return "{}"
	})
	return templateExpressionRE.ReplaceAllString(value, "{}")
}

func normalizeFrontendAPIPath(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	if !strings.HasPrefix(path, "/") {
		return "", false
	}
	if path == "/api/v1" {
		path = "/"
	} else if strings.HasPrefix(path, "/api/v1/") {
		path = strings.TrimPrefix(path, "/api/v1")
	}
	path = frontendPathTemplateRE.ReplaceAllString(path, "{}")
	return normalizePath(path), true
}

func collectGoRoutePatterns(t *testing.T, root string) []goRoutePattern {
	t.Helper()
	patterns := []goRoutePattern{}
	for _, module := range routeModules {
		for _, item := range extractGoRoutes(t, filepath.Join(root, module.GoHandlerFile)) {
			path := joinRoutePath(module.Name, item.Path)
			patterns = append(patterns, goRoutePattern{
				Method:  item.Method,
				Path:    path,
				Subtree: item.Method == "*" && strings.HasSuffix(path, "/"),
			})
		}
	}
	return patterns
}

func joinRoutePath(base, suffix string) string {
	base = normalizePath(base)
	suffix = normalizePath(suffix)
	if suffix == "" {
		return base
	}
	if suffix == "/" {
		return base + "/"
	}
	return base + suffix
}

func goRouteCovered(endpoint frontendEndpoint, patterns []goRoutePattern) bool {
	for _, pattern := range patterns {
		if pattern.Method != "*" && pattern.Method != endpoint.Method {
			continue
		}
		if pattern.Subtree {
			base := strings.TrimSuffix(pattern.Path, "/")
			if endpoint.Path == base || strings.HasPrefix(endpoint.Path, pattern.Path) {
				return true
			}
			continue
		}
		if endpoint.Path == pattern.Path {
			return true
		}
	}
	return false
}

func stripTypeScriptComments(source string) string {
	out := []byte(source)
	var quote byte
	escaped := false
	for i := 0; i < len(out); i++ {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if out[i] == '\\' {
				escaped = true
				continue
			}
			if out[i] == quote {
				quote = 0
			}
			continue
		}
		switch out[i] {
		case '\'', '"', '`':
			quote = out[i]
		case '/':
			if i+1 >= len(out) {
				continue
			}
			switch out[i+1] {
			case '/':
				out[i] = ' '
				out[i+1] = ' '
				i += 2
				for i < len(out) && out[i] != '\n' {
					out[i] = ' '
					i++
				}
				i--
			case '*':
				out[i] = ' '
				out[i+1] = ' '
				i += 2
				for i+1 < len(out) && !(out[i] == '*' && out[i+1] == '/') {
					if out[i] != '\n' {
						out[i] = ' '
					}
					i++
				}
				if i+1 < len(out) {
					out[i] = ' '
					out[i+1] = ' '
					i++
				}
			}
		}
	}
	return string(out)
}

func findNextOpenParen(source string, start int) int {
	index := strings.IndexByte(source[start:], '(')
	if index < 0 {
		return -1
	}
	return start + index
}

func lineNumber(source string, offset int) int {
	if offset < 0 {
		return 1
	}
	if offset > len(source) {
		offset = len(source)
	}
	return strings.Count(source[:offset], "\n") + 1
}

func isWhitespace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\r' || char == '\n'
}

func isIdentifierStart(char byte) bool {
	return char == '_' || char == '$' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isIdentifierContinue(char byte) bool {
	return isIdentifierStart(char) || (char >= '0' && char <= '9')
}
