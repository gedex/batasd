package httpapi

import (
	"bufio"
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/gedex/batasd/internal/language"
	"github.com/gedex/batasd/internal/status"
	"github.com/gedex/batasd/internal/submission"
)

type routeSpec struct {
	Method string
	Path   string
}

func TestOpenAPIRoutesMatchChiRoutes(t *testing.T) {
	documented := openAPIRoutes(t)
	actual := chiRoutes(t)

	if diff := missingRoutes(actual, documented); len(diff) > 0 {
		t.Errorf("chi routes missing from OpenAPI: %s", strings.Join(diff, ", "))
	}
	if diff := missingRoutes(documented, actual); len(diff) > 0 {
		t.Errorf("OpenAPI routes missing from chi router: %s", strings.Join(diff, ", "))
	}
}

func TestOpenAPIComponentPropertiesMatchPublicDTOs(t *testing.T) {
	tests := []struct {
		schema string
		fields []string
	}{
		{schema: "ErrorResponse", fields: jsonFields(errorResponse{})},
		{schema: "Status", fields: jsonFields(status.Status{})},
		{schema: "Limits", fields: jsonFields(submission.Limits{})},
		{schema: "Language", fields: jsonFields(language.Language{})},
		{schema: "LanguageVersion", fields: jsonFields(language.Version{})},
		{schema: "AdditionalFiles", fields: jsonFields(submission.AdditionalFiles{})},
		{schema: "Callback", fields: jsonFields(submission.Callback{})},
		{schema: "CreateSubmissionRequest", fields: jsonFields(submission.CreateRequest{})},
		{schema: "CreatedSubmissionResponse", fields: []string{"status", "token"}},
		{schema: "Submission", fields: jsonFields(submission.Response{})},
		{schema: "SubmissionListResponse", fields: jsonFields(submission.ListResponse{})},
		{schema: "CallbackAttempt", fields: jsonFields(submission.CallbackAttemptResponse{})},
		{schema: "CallbackAttemptsResponse", fields: jsonFields(submission.CallbackAttemptsResponse{})},
	}

	for _, tt := range tests {
		t.Run(tt.schema, func(t *testing.T) {
			documented := openAPIComponentProperties(t, tt.schema)
			if diff := missingStrings(tt.fields, documented); len(diff) > 0 {
				t.Errorf("Go JSON fields missing from OpenAPI schema %s: %s", tt.schema, strings.Join(diff, ", "))
			}
			if diff := missingStrings(documented, tt.fields); len(diff) > 0 {
				t.Errorf("OpenAPI schema %s properties missing from Go JSON fields: %s", tt.schema, strings.Join(diff, ", "))
			}
		})
	}
}

func TestOpenAPIStatusCodeEnumMatchesStatusCatalog(t *testing.T) {
	var expected []string
	for _, item := range status.List() {
		expected = append(expected, item.Code)
	}
	sort.Strings(expected)

	documented := openAPIComponentEnum(t, "StatusCode")
	if diff := missingStrings(expected, documented); len(diff) > 0 {
		t.Errorf("status codes missing from OpenAPI StatusCode enum: %s", strings.Join(diff, ", "))
	}
	if diff := missingStrings(documented, expected); len(diff) > 0 {
		t.Errorf("OpenAPI StatusCode enum values missing from status catalog: %s", strings.Join(diff, ", "))
	}
}

func openAPIRoutes(t *testing.T) map[routeSpec]struct{} {
	t.Helper()

	data := openAPIData(t)

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

func openAPIData(t *testing.T) []byte {
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
	return data
}

func openAPIComponentProperties(t *testing.T, schema string) []string {
	t.Helper()

	data := openAPIData(t)
	var fields []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	inSchema := false
	inProperties := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if leadingSpaces(line) == 4 && strings.HasSuffix(trimmed, ":") {
			if inSchema {
				break
			}
			inSchema = strings.TrimSuffix(trimmed, ":") == schema
			continue
		}
		if !inSchema {
			continue
		}
		if leadingSpaces(line) == 6 && trimmed == "properties:" {
			inProperties = true
			continue
		}
		if !inProperties {
			continue
		}
		if leadingSpaces(line) == 8 && strings.HasSuffix(trimmed, ":") {
			fields = append(fields, strings.TrimSuffix(trimmed, ":"))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !inSchema {
		t.Fatalf("OpenAPI schema %q not found", schema)
	}
	if len(fields) == 0 {
		t.Fatalf("OpenAPI schema %q has no top-level properties", schema)
	}
	sort.Strings(fields)
	return fields
}

func openAPIComponentEnum(t *testing.T, schema string) []string {
	t.Helper()

	data := openAPIData(t)
	var values []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	inSchema := false
	inEnum := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if leadingSpaces(line) == 4 && strings.HasSuffix(trimmed, ":") {
			if inSchema {
				break
			}
			inSchema = strings.TrimSuffix(trimmed, ":") == schema
			continue
		}
		if !inSchema {
			continue
		}
		if leadingSpaces(line) == 6 && trimmed == "enum:" {
			inEnum = true
			continue
		}
		if !inEnum {
			continue
		}
		if leadingSpaces(line) == 8 && strings.HasPrefix(trimmed, "- ") {
			values = append(values, strings.TrimPrefix(trimmed, "- "))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !inSchema {
		t.Fatalf("OpenAPI schema %q not found", schema)
	}
	if len(values) == 0 {
		t.Fatalf("OpenAPI schema %q has no enum values", schema)
	}
	sort.Strings(values)
	return values
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

func missingStrings(want, got []string) []string {
	gotSet := make(map[string]struct{}, len(got))
	for _, value := range got {
		gotSet[value] = struct{}{}
	}

	var missing []string
	for _, value := range want {
		if _, ok := gotSet[value]; !ok {
			missing = append(missing, value)
		}
	}
	sort.Strings(missing)
	return missing
}

func jsonFields(value any) []string {
	typ := reflect.TypeOf(value)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	var fields []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" {
			name = field.Name
		}
		if name == "-" {
			continue
		}
		fields = append(fields, name)
	}
	sort.Strings(fields)
	return fields
}

func leadingSpaces(value string) int {
	return len(value) - len(strings.TrimLeft(value, " "))
}

func isHTTPMethod(value string) bool {
	switch strings.ToLower(value) {
	case "connect", "delete", "get", "head", "options", "patch", "post", "put", "trace":
		return true
	default:
		return false
	}
}
