package language

import "testing"

func TestLoadCatalogIncludesExpectedLanguages(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]struct {
		sourceFile string
		compiled   bool
	}{
		"c-gcc":       {sourceFile: "main.c", compiled: true},
		"cpp-gcc":     {sourceFile: "main.cpp", compiled: true},
		"go-1.24":     {sourceFile: "main.go", compiled: true},
		"java-21":     {sourceFile: "Main.java", compiled: true},
		"node-22":     {sourceFile: "main.js"},
		"php-8.3":     {sourceFile: "main.php"},
		"python-3.12": {sourceFile: "main.py"},
		"rust-1.88":   {sourceFile: "main.rs", compiled: true},
	}

	for slug, want := range expected {
		t.Run(slug, func(t *testing.T) {
			lang, ok := registry.Get(slug)
			if !ok {
				t.Fatalf("%s language not found", slug)
			}
			if lang.SourceFile != want.sourceFile {
				t.Fatalf("SourceFile = %q, want %q", lang.SourceFile, want.sourceFile)
			}
			if len(lang.Run) == 0 {
				t.Fatal("Run command is empty")
			}
			if want.compiled && len(lang.Compile) == 0 {
				t.Fatal("Compile command is empty")
			}
			if !want.compiled && len(lang.Compile) != 0 {
				t.Fatalf("Compile command = %v, want empty", lang.Compile)
			}
		})
	}
}

func TestLoadCatalogIncludesNodeDefaultLimits(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	node, ok := registry.Get("node-22")
	if !ok {
		t.Fatal("node-22 language not found")
	}
	if node.DefaultLimits == nil {
		t.Fatal("node-22 default limits are nil")
	}
	if node.DefaultLimits.MemoryKB != 2097152 {
		t.Fatalf("MemoryKB = %d, want 2097152", node.DefaultLimits.MemoryKB)
	}
	if node.DefaultLimits.MaxProcesses != 256 {
		t.Fatalf("MaxProcesses = %d, want 256", node.DefaultLimits.MaxProcesses)
	}
}

func TestLoadCatalogUsesJavaRunnerWrappers(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	java, ok := registry.Get("java-21")
	if !ok {
		t.Fatal("java-21 language not found")
	}
	if got := java.Compile[0]; got != "batasd-javac" {
		t.Fatalf("Compile[0] = %q, want batasd-javac", got)
	}
	if got := java.Run[0]; got != "batasd-java" {
		t.Fatalf("Run[0] = %q, want batasd-java", got)
	}
}
