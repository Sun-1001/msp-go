package contract

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type route struct {
	Method string
	Path   string
}

type routeModule struct {
	Name          string
	GoHandlerFile string
}

var routeModules = []routeModule{
	{Name: "/auth", GoHandlerFile: "backend-go/internal/adapter/http/auth/handler.go"},
	{Name: "/session", GoHandlerFile: "backend-go/internal/adapter/http/session/handler.go"},
	{Name: "/exercise", GoHandlerFile: "backend-go/internal/adapter/http/exercise/handler.go"},
	{Name: "/mistakes", GoHandlerFile: "backend-go/internal/adapter/http/mistake/handler.go"},
	{Name: "/questions", GoHandlerFile: "backend-go/internal/adapter/http/question/handler.go"},
	{Name: "/progress", GoHandlerFile: "backend-go/internal/adapter/http/progress/handler.go"},
	{Name: "/resources", GoHandlerFile: "backend-go/internal/adapter/http/resource/handler.go"},
	{Name: "/upload", GoHandlerFile: "backend-go/internal/adapter/http/upload/handler.go"},
	{Name: "/xidian", GoHandlerFile: "backend-go/internal/adapter/http/xidian/handler.go"},
	{Name: "/classes", GoHandlerFile: "backend-go/internal/adapter/http/classroom/handler.go"},
	{Name: "/teacher", GoHandlerFile: "backend-go/internal/adapter/http/teacher/handler.go"},
	{Name: "/portrait", GoHandlerFile: "backend-go/internal/adapter/http/portrait/handler.go"},
	{Name: "/admin/users", GoHandlerFile: "backend-go/internal/adapter/http/adminuser/handler.go"},
	{Name: "/admin/stats", GoHandlerFile: "backend-go/internal/adapter/http/adminstats/handler.go"},
	{Name: "/admin/settings", GoHandlerFile: "backend-go/internal/adapter/http/adminsettings/handler.go"},
	{Name: "/admin/security-logs", GoHandlerFile: "backend-go/internal/adapter/http/securitylog/handler.go"},
	{Name: "/admin/knowledge", GoHandlerFile: "backend-go/internal/adapter/http/knowledge/handler.go"},
	{Name: "/admin/inbox", GoHandlerFile: "backend-go/internal/adapter/http/admininbox/handler.go"},
	{Name: "/admin/ai-config", GoHandlerFile: "backend-go/internal/adapter/http/adminaiconfig/handler.go"},
}

var (
	goRouteRE        = regexp.MustCompile(`mux\.HandleFunc\((.*),`)
	goMethodPrefixRE = regexp.MustCompile(`"([A-Z]+) "\s*\+\s*prefix(?:\s*\+\s*"([^"]*)")?`)
	goPrefixOnlyRE   = regexp.MustCompile(`^prefix(?:\s*\+\s*"([^"]*)")?$`)
	pathParamRE      = regexp.MustCompile(`\{[^}/]+\}`)
)

func TestGoRouteModulesAreRegistered(t *testing.T) {
	root := repoRoot(t)
	for _, module := range routeModules {
		t.Run(module.Name, func(t *testing.T) {
			goRoutes := extractGoRoutes(t, filepath.Join(root, module.GoHandlerFile))

			if len(goRoutes) == 0 {
				t.Fatalf("Go handler %s registered no routes", module.GoHandlerFile)
			}
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

func extractGoRoutes(t *testing.T, filename string) []route {
	t.Helper()
	source := readFile(t, filename)
	routes := []route{}
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		match := goRouteRE.FindStringSubmatch(line)
		if len(match) != 2 {
			continue
		}
		expr := strings.TrimSpace(match[1])
		if routeMatch := goMethodPrefixRE.FindStringSubmatch(expr); len(routeMatch) == 3 {
			routes = append(routes, route{Method: routeMatch[1], Path: normalizePath(routeMatch[2])})
			continue
		}
		if routeMatch := goPrefixOnlyRE.FindStringSubmatch(expr); len(routeMatch) == 2 {
			routes = append(routes, route{Method: "*", Path: normalizePath(routeMatch[1])})
			continue
		}
		t.Fatalf("unsupported Go route expression in %s: %s", filename, expr)
	}
	return routes
}

func matchingParen(source string, openIndex int) int {
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
		case '\'', '"':
			quote = char
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return absolute
			}
		}
	}
	return -1
}

func routeKeys(routes []route) map[string]bool {
	keys := make(map[string]bool, len(routes))
	for _, item := range routes {
		keys[item.Method+" "+item.Path] = true
	}
	return keys
}

func difference(left, right map[string]bool) []string {
	result := []string{}
	for key := range left {
		if !right[key] {
			result = append(result, key)
		}
	}
	sort.Strings(result)
	return result
}

func sortedKeys(items map[string]bool) []string {
	result := make([]string, 0, len(items))
	for key := range items {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return pathParamRE.ReplaceAllString(path, "{}")
}

func readFile(t *testing.T, filename string) string {
	t.Helper()
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read %s: %v", filename, err)
	}
	return string(content)
}
