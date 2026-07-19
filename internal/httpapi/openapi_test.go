package httpapi

import (
	"bufio"
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type routeSpec struct {
	Method string
	Path   string
}

func TestOpenAPIRoutesMatchChiRoutes(t *testing.T) {
	documented := openAPIRoutes(t)
	actual := chiRoutes(t)

	if diff := missingRoutes(documented, actual); len(diff) > 0 {
		t.Fatalf("OpenAPI is missing routes: %s", strings.Join(diff, ", "))
	}
	if diff := missingRoutes(actual, documented); len(diff) > 0 {
		t.Fatalf("OpenAPI documents routes not registered in chi: %s", strings.Join(diff, ", "))
	}
}

func openAPIRoutes(t *testing.T) map[routeSpec]struct{} {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	data, err := os.ReadFile(filepath.Join(root, "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	routes := make(map[routeSpec]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(data))
	inPaths := false
	currentPath := ""
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "paths:" {
			inPaths = true
			continue
		}
		if !inPaths {
			continue
		}
		if trimmed == "components:" {
			break
		}
		if strings.HasPrefix(line, "  /") && strings.HasSuffix(trimmed, ":") {
			currentPath = strings.TrimSuffix(trimmed, ":")
			continue
		}
		if currentPath == "" || !strings.HasPrefix(line, "    ") || !strings.HasSuffix(trimmed, ":") {
			continue
		}
		method := strings.TrimSuffix(trimmed, ":")
		if isHTTPMethod(method) {
			routes[routeSpec{Method: strings.ToUpper(method), Path: currentPath}] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(routes) == 0 {
		t.Fatal("no OpenAPI routes found")
	}
	return routes
}

func chiRoutes(t *testing.T) map[routeSpec]struct{} {
	t.Helper()

	router := NewRouter(Dependencies{})
	routes, ok := router.(chi.Routes)
	if !ok {
		t.Fatalf("router = %T, want chi.Routes", router)
	}

	out := make(map[routeSpec]struct{})
	if err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		out[routeSpec{Method: method, Path: route}] = struct{}{}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func missingRoutes(want, got map[routeSpec]struct{}) []string {
	var missing []string
	for route := range want {
		if _, ok := got[route]; !ok {
			missing = append(missing, route.Method+" "+route.Path)
		}
	}
	sort.Strings(missing)
	return missing
}

func isHTTPMethod(value string) bool {
	switch strings.ToLower(value) {
	case "connect", "delete", "get", "head", "options", "patch", "post", "put", "trace":
		return true
	default:
		return false
	}
}
