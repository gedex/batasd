package language

import "testing"

func TestLoadCatalogIncludesExpectedLanguages(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]struct {
		version    string
		sourceFile string
		compiled   bool
	}{
		"c":      {version: "gcc", sourceFile: "main.c", compiled: true},
		"cpp":    {version: "gcc", sourceFile: "main.cpp", compiled: true},
		"go":     {version: "1.24", sourceFile: "main.go", compiled: true},
		"java":   {version: "21", sourceFile: "Main.java", compiled: true},
		"node":   {version: "22", sourceFile: "main.js"},
		"php":    {version: "8.3", sourceFile: "main.php"},
		"python": {version: "3.12", sourceFile: "main.py"},
		"rust":   {version: "1.88", sourceFile: "main.rs", compiled: true},
	}

	for slug, want := range expected {
		t.Run(slug, func(t *testing.T) {
			lang, ok := registry.Get(slug)
			if !ok {
				t.Fatalf("%s language not found", slug)
			}
			if lang.DefaultVersion != want.version {
				t.Fatalf("DefaultVersion = %q, want %q", lang.DefaultVersion, want.version)
			}
			resolved, ok := registry.Resolve(slug, "")
			if !ok {
				t.Fatalf("%s default version did not resolve", slug)
			}
			if resolved.Version != want.version {
				t.Fatalf("resolved version = %q, want %q", resolved.Version, want.version)
			}
			if resolved.SourceFile != want.sourceFile {
				t.Fatalf("SourceFile = %q, want %q", resolved.SourceFile, want.sourceFile)
			}
			if len(resolved.Run) == 0 {
				t.Fatal("Run command is empty")
			}
			if want.compiled && len(resolved.Compile) == 0 {
				t.Fatal("Compile command is empty")
			}
			if !want.compiled && len(resolved.Compile) != 0 {
				t.Fatalf("Compile command = %v, want empty", resolved.Compile)
			}
		})
	}
}

func TestLoadCatalogIncludesNodeDefaultLimits(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	node, ok := registry.Resolve("node", "")
	if !ok {
		t.Fatal("node language not found")
	}
	if node.DefaultLimits == nil {
		t.Fatal("node default limits are nil")
	}
	if node.DefaultLimits.MemoryKB != 2097152 {
		t.Fatalf("MemoryKB = %d, want 2097152", node.DefaultLimits.MemoryKB)
	}
	if node.DefaultLimits.MaxProcesses != 256 {
		t.Fatalf("MaxProcesses = %d, want 256", node.DefaultLimits.MaxProcesses)
	}
}

func TestLoadCatalogUsesPortableJavaCommands(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	java, ok := registry.Resolve("java", "21")
	if !ok {
		t.Fatal("java 21 language not found")
	}
	if got := java.Compile[0]; got != "javac" {
		t.Fatalf("Compile[0] = %q, want javac", got)
	}
	if got := java.Run[0]; got != "java" {
		t.Fatalf("Run[0] = %q, want java", got)
	}
}

func TestResolveUsesShortSlugsAndVersions(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	python, ok := registry.Resolve("python", "3.12")
	if !ok {
		t.Fatal("python 3.12 did not resolve")
	}
	if python.Slug != "python" || python.Version != "3.12" {
		t.Fatalf("resolved python = (%q, %q), want (python, 3.12)", python.Slug, python.Version)
	}
	if _, ok := registry.Resolve("python-3.12", ""); ok {
		t.Fatal("old explicit python-3.12 slug resolved unexpectedly")
	}
}
