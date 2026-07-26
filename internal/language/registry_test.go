package language

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		"c":          {version: "gcc", sourceFile: "main.c", compiled: true},
		"cpp":        {version: "gcc", sourceFile: "main.cpp", compiled: true},
		"go":         {version: "1.24", sourceFile: "main.go", compiled: true},
		"java":       {version: "21", sourceFile: "Main.java", compiled: true},
		"javascript": {version: "22", sourceFile: "main.js"},
		"node":       {version: "22", sourceFile: "main.js"},
		"php":        {version: "8.3", sourceFile: "main.php"},
		"python":     {version: "3.12", sourceFile: "main.py"},
		"rust":       {version: "1.88", sourceFile: "main.rs", compiled: true},
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

func TestLoadCatalogIncludesJavaScriptDefaultLimits(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	javascript, ok := registry.Resolve("javascript", "")
	if !ok {
		t.Fatal("javascript language not found")
	}
	if javascript.DefaultLimits == nil {
		t.Fatal("javascript default limits are nil")
	}
	if javascript.DefaultLimits.MemoryKB != 2097152 {
		t.Fatalf("MemoryKB = %d, want 2097152", javascript.DefaultLimits.MemoryKB)
	}
	if javascript.DefaultLimits.MaxProcesses != 256 {
		t.Fatalf("MaxProcesses = %d, want 256", javascript.DefaultLimits.MaxProcesses)
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
	if got := java.Compile[1]; got != "-J-Xmx256m" {
		t.Fatalf("Compile[1] = %q, want -J-Xmx256m", got)
	}
	if got := java.Run[0]; got != "java" {
		t.Fatalf("Run[0] = %q, want java", got)
	}
	if got := java.Run[1]; got != "-Xmx256m" {
		t.Fatalf("Run[1] = %q, want -Xmx256m", got)
	}
}

func TestLoadCatalogCompiledLanguagesUseLargerFileLimit(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	for _, slug := range []string{"c", "cpp", "go", "java", "rust"} {
		t.Run(slug, func(t *testing.T) {
			lang, ok := registry.Resolve(slug, "")
			if !ok {
				t.Fatalf("%s language not found", slug)
			}
			if lang.DefaultLimits == nil {
				t.Fatal("default limits are nil")
			}
			if lang.DefaultLimits.MaxFileKB != 65536 {
				t.Fatalf("MaxFileKB = %d, want 65536", lang.DefaultLimits.MaxFileKB)
			}
		})
	}
}

func TestLoadCatalogGoAllowsColdCompileCPU(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	goLang, ok := registry.Resolve("go", "")
	if !ok {
		t.Fatal("go language not found")
	}
	if goLang.DefaultLimits == nil {
		t.Fatal("go default limits are nil")
	}
	if goLang.DefaultLimits.CPUTimeMS != 15000 {
		t.Fatalf("CPUTimeMS = %d, want 15000", goLang.DefaultLimits.CPUTimeMS)
	}
	if goLang.DefaultLimits.CPUExtraMS != 3000 {
		t.Fatalf("CPUExtraMS = %d, want 3000", goLang.DefaultLimits.CPUExtraMS)
	}
}

func TestLoadCatalogGoCompileWrapperPassesCompilerOptionsAsArgs(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	goLang, ok := registry.Resolve("go", "")
	if !ok {
		t.Fatal("go language not found")
	}
	if len(goLang.Compile) < 4 {
		t.Fatalf("Compile = %v, want sh -c wrapper", goLang.Compile)
	}
	if goLang.Compile[0] != "sh" || goLang.Compile[1] != "-c" {
		t.Fatalf("Compile prefix = %v, want sh -c", goLang.Compile[:2])
	}
	if !strings.Contains(goLang.Compile[2], "\"$@\"") {
		t.Fatalf("Compile script = %q, want quoted positional args", goLang.Compile[2])
	}
	if goLang.Compile[3] != "batasd-go-build" {
		t.Fatalf("Compile[3] = %q, want batasd-go-build", goLang.Compile[3])
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

func TestLoadCatalogIncludesExercismTracks(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	expected := exercismTracks()
	if len(expected) != 83 {
		t.Fatalf("len(expected) = %d, want 83", len(expected))
	}

	for slug, name := range expected {
		t.Run(slug, func(t *testing.T) {
			lang, ok := registry.Get(slug)
			if !ok {
				t.Fatalf("%s track not found", slug)
			}
			if lang.Name != name {
				t.Fatalf("Name = %q, want %q", lang.Name, name)
			}
			if len(lang.Versions) == 0 {
				t.Fatal("Versions is empty")
			}
		})
	}
}

func TestListReturnsFullCatalog(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	languages := registry.List()
	if len(languages) != 84 {
		t.Fatalf("len(List()) = %d, want 84", len(languages))
	}
	foundDisabled := false
	for _, lang := range languages {
		if lang.Slug == "ruby" && !lang.Enabled {
			foundDisabled = true
		}
	}
	if !foundDisabled {
		t.Fatal("List did not include disabled ruby catalog entry")
	}
}

func TestCatalogEntriesHaveE2EFixtures(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	fixturesDir := filepath.Join("..", "..", "testdata", "e2e", "programs", "languages")
	for _, lang := range registry.List() {
		for _, version := range lang.Versions {
			t.Run(lang.Slug+"/"+version.Version, func(t *testing.T) {
				path := filepath.Join(fixturesDir, lang.Slug, version.SourceFile)
				info, err := os.Stat(path)
				if err != nil {
					t.Fatalf("fixture %s: %v", path, err)
				}
				if info.IsDir() {
					t.Fatalf("fixture %s is a directory", path)
				}
				if info.Size() == 0 {
					t.Fatalf("fixture %s is empty", path)
				}
			})
		}
	}
}

func TestLoadCatalogRetainsNodeAlias(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	node, ok := registry.Get("node")
	if !ok {
		t.Fatal("node alias not found")
	}
	if !node.Enabled {
		t.Fatal("node alias is disabled")
	}
	if _, ok := registry.Resolve("node", "22"); !ok {
		t.Fatal("node alias did not resolve")
	}
}

func TestDisabledCatalogTracksAreDiscoverableButNotExecutable(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	ruby, ok := registry.Get("ruby")
	if !ok {
		t.Fatal("ruby track not found")
	}
	if ruby.Enabled {
		t.Fatal("ruby is enabled, want disabled until its runtime is configured")
	}
	if len(ruby.Versions) != 1 {
		t.Fatalf("len(Versions) = %d, want 1", len(ruby.Versions))
	}
	if ruby.Versions[0].Enabled {
		t.Fatal("ruby catalog version is enabled")
	}
	if len(ruby.Versions[0].Run) != 0 {
		t.Fatalf("ruby run command = %v, want empty disabled command", ruby.Versions[0].Run)
	}
	if _, ok := registry.Resolve("ruby", ""); ok {
		t.Fatal("disabled ruby track resolved unexpectedly")
	}
}

func exercismTracks() map[string]string {
	return map[string]string{
		"8th":             "8th",
		"abap":            "ABAP",
		"arm64-assembly":  "ARM64 Assembly",
		"arturo":          "Arturo",
		"awk":             "AWK",
		"ballerina":       "Ballerina",
		"bash":            "Bash",
		"batch":           "Batch Script",
		"c":               "C",
		"cairo":           "Cairo",
		"cfml":            "CFML",
		"clojure":         "Clojure",
		"cobol":           "COBOL",
		"coffeescript":    "CoffeeScript",
		"common-lisp":     "Common Lisp",
		"cpp":             "C++",
		"crystal":         "Crystal",
		"csharp":          "C#",
		"d":               "D",
		"dart":            "Dart",
		"delphi":          "Delphi Pascal",
		"elixir":          "Elixir",
		"elm":             "Elm",
		"emacs-lisp":      "Emacs Lisp",
		"erlang":          "Erlang",
		"euphoria":        "Euphoria",
		"factor":          "Factor",
		"fortran":         "Fortran",
		"free-pascal":     "Free Pascal",
		"fsharp":          "F#",
		"futhark":         "Futhark",
		"gleam":           "Gleam",
		"go":              "Go",
		"groovy":          "Groovy",
		"haskell":         "Haskell",
		"idris":           "Idris",
		"java":            "Java",
		"javascript":      "JavaScript",
		"jq":              "jq",
		"julia":           "Julia",
		"kotlin":          "Kotlin",
		"lean":            "Lean",
		"lfe":             "Lisp Flavoured Erlang",
		"lua":             "Lua",
		"mips":            "MIPS Assembly",
		"moonscript":      "MoonScript",
		"nim":             "Nim",
		"objective-c":     "Objective-C",
		"ocaml":           "OCaml",
		"odin":            "Odin",
		"perl5":           "Perl",
		"pharo-smalltalk": "Pharo",
		"php":             "PHP",
		"powershell":      "PowerShell",
		"prolog":          "Prolog",
		"purescript":      "PureScript",
		"pyret":           "Pyret",
		"python":          "Python",
		"r":               "R",
		"racket":          "Racket",
		"raku":            "Raku",
		"reasonml":        "ReasonML",
		"red":             "Red",
		"roc":             "Roc",
		"ruby":            "Ruby",
		"rust":            "Rust",
		"scala":           "Scala",
		"scheme":          "Scheme",
		"sqlite":          "SQLite",
		"sml":             "Standard ML",
		"swift":           "Swift",
		"tcl":             "Tcl",
		"typescript":      "TypeScript",
		"uiua":            "Uiua",
		"unison":          "Unison",
		"vbnet":           "Visual Basic",
		"vimscript":       "Vim script",
		"vlang":           "V",
		"wasm":            "WebAssembly",
		"wren":            "Wren",
		"x86-64-assembly": "x86-64 Assembly",
		"yamlscript":      "YAMLScript",
		"zig":             "Zig",
	}
}
