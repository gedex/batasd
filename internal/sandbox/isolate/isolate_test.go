package isolate

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/gedex/batasd/internal/sandbox"
	"github.com/gedex/batasd/internal/submission"
)

func TestRunArgsUseDefaultIsolatedNetwork(t *testing.T) {
	runner := NewRunner("isolate", 3, 1, true)
	command := sandbox.Command{
		Args: []string{"python3", "main.py"},
		Limits: submission.Limits{
			CPUTimeMS:    1500,
			CPUExtraMS:   250,
			WallTimeMS:   3000,
			MemoryKB:     64000,
			StackKB:      16000,
			MaxProcesses: 4,
			MaxFileKB:    512,
			Network:      false,
		},
	}

	args := runner.runArgs(3, "/tmp/work", "/tmp/meta", command)

	wantContains := []string{
		"--box-id", "3",
		"--cg",
		"--silent",
		"--meta", "/tmp/meta",
		"--chdir", "/box",
		"--dir", "/box=/tmp/work:rw",
		"--env", "PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin",
		"--env", "HOME=/box",
		"--env", "CGO_ENABLED=0",
		"--env", "CARGO_HOME=/usr/local/cargo",
		"--env", "GOCACHE=/box/.cache/go-build",
		"--env", "GOMODCACHE=/box/.cache/go-mod",
		"--env", "PYTHONDONTWRITEBYTECODE=1",
		"--env", "RUSTUP_HOME=/usr/local/rustup",
		"--time", "1.500",
		"--extra-time", "0.250",
		"--wall-time", "3.000",
		"--mem", "64000",
		"--cg-mem", "64000",
		"--stack", "16000",
		"--fsize", "512",
		"--processes=4",
		"--run",
		"--",
		"python3",
		"main.py",
	}
	for _, value := range wantContains {
		if !contains(args, value) {
			t.Fatalf("args missing %q: %v", value, args)
		}
	}
	if contains(args, "--share-net") {
		t.Fatalf("args include --share-net when network is disabled: %v", args)
	}
}

func TestRunArgsAllowNetworkWhenRequested(t *testing.T) {
	runner := NewRunner("isolate", 0, 1, false)
	command := sandbox.Command{
		Args:   []string{"python3", "main.py"},
		Limits: submission.Limits{Network: true},
	}

	args := runner.runArgs(0, "/tmp/work", "/tmp/meta", command)

	if !contains(args, "--share-net") {
		t.Fatalf("args missing --share-net: %v", args)
	}
	if contains(args, "--cg") {
		t.Fatalf("args include --cg when cgroups are disabled: %v", args)
	}
}

func TestRunArgsKeepCommandAfterSeparator(t *testing.T) {
	runner := NewRunner("isolate", 0, 1, false)
	command := sandbox.Command{Args: []string{"node", "main.js", "--flag"}}

	args := runner.runArgs(0, "/tmp/work", "/tmp/meta", command)
	index := indexOf(args, "--")
	if index == -1 {
		t.Fatalf("args missing separator: %v", args)
	}

	got := args[index+1:]
	want := []string{"node", "main.js", "--flag"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command args = %v, want %v", got, want)
	}
}

func TestApplyMetaMapsRuntimeValues(t *testing.T) {
	result := sandbox.Result{}
	applyMeta(&result, meta{
		"time":      "0.123",
		"time-wall": "0.456",
		"max-rss":   "7890",
		"exitcode":  "7",
		"message":   "runtime failed",
	})

	if result.TimeMS != 123 {
		t.Fatalf("TimeMS = %d, want 123", result.TimeMS)
	}
	if result.WallTimeMS != 456 {
		t.Fatalf("WallTimeMS = %d, want 456", result.WallTimeMS)
	}
	if result.MemoryKB == nil || *result.MemoryKB != 7890 {
		t.Fatalf("MemoryKB = %v, want 7890", result.MemoryKB)
	}
	if result.ExitCode == nil || *result.ExitCode != 7 {
		t.Fatalf("ExitCode = %v, want 7", result.ExitCode)
	}
	if result.Message != "runtime failed" {
		t.Fatalf("Message = %q, want runtime failed", result.Message)
	}
}

func TestResolveExecutableKeepsAbsolutePath(t *testing.T) {
	got, err := resolveExecutable([]string{"/usr/bin/python3", "main.py"})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"/usr/bin/python3", "main.py"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
}

func TestResolveExecutableFindsPathExecutable(t *testing.T) {
	got, err := resolveExecutable([]string{"go", "version"})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(got[0], "/go") {
		t.Fatalf("executable = %q, want path ending in /go", got[0])
	}
	if got[1] != "version" {
		t.Fatalf("arg = %q, want version", got[1])
	}
}

func TestApplyMetaMapsTimeout(t *testing.T) {
	result := sandbox.Result{}
	applyMeta(&result, meta{"status": "TO"})

	if !result.TimedOut {
		t.Fatal("TimedOut = false, want true")
	}
	if result.Message != "time limit exceeded" {
		t.Fatalf("Message = %q, want time limit exceeded", result.Message)
	}
}

func TestApplyMetaMapsCGroupOOM(t *testing.T) {
	result := sandbox.Result{}
	applyMeta(&result, meta{"cg-oom-killed": "1"})

	if !result.MemoryLimit {
		t.Fatal("MemoryLimit = false, want true")
	}
	if result.Message != "memory limit exceeded" {
		t.Fatalf("Message = %q, want memory limit exceeded", result.Message)
	}
}

func TestReadMetaFile(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "meta-*")
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"time:0.001",
		"message:hello:with:colon",
		"invalid",
	}, "\n")
	if _, err := file.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := readMetaFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if got["time"] != "0.001" {
		t.Fatalf("time = %q, want 0.001", got["time"])
	}
	if got["message"] != "hello:with:colon" {
		t.Fatalf("message = %q, want hello:with:colon", got["message"])
	}
	if _, ok := got["invalid"]; ok {
		t.Fatalf("invalid line was parsed: %v", got)
	}
}

func contains(values []string, want string) bool {
	return indexOf(values, want) != -1
}

func indexOf(values []string, want string) int {
	for i, value := range values {
		if value == want {
			return i
		}
	}
	return -1
}
