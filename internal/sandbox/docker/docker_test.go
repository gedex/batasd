package docker

import (
	"strings"
	"testing"

	"github.com/gedex/batasd/internal/sandbox"
	"github.com/gedex/batasd/internal/submission"
)

func TestDockerArgsDisableNetworkByDefault(t *testing.T) {
	runner := NewRunner("docker", "batasd-runner:local")

	args := runner.dockerArgs("batasd-test", "/tmp/work", sandbox.Command{
		Args: []string{"python3", "main.py"},
		Limits: submission.Limits{
			MemoryKB:     128000,
			MaxProcesses: 60,
			Network:      false,
		},
	})

	joined := strings.Join(args, "\x00")
	for _, want := range []string{
		"--interactive",
		"--env\x00CGO_ENABLED=0",
		"--env\x00GOCACHE=/workspace/.cache/go-build",
		"--env\x00GOMODCACHE=/workspace/.cache/go-mod",
		"--network\x00none",
		"--memory\x00128000k",
		"--pids-limit\x0060",
		"batasd-runner:local\x00python3\x00main.py",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected docker args to contain %q in %#v", want, args)
		}
	}
	if strings.Contains(joined, "--rm") {
		t.Fatalf("expected docker args to keep the container inspectable: %#v", args)
	}
}

func TestDockerArgsAllowNetworkWhenRequested(t *testing.T) {
	runner := NewRunner("docker", "batasd-runner:local")

	args := runner.dockerArgs("batasd-test", "/tmp/work", sandbox.Command{
		Args:   []string{"node", "main.js"},
		Limits: submission.Limits{Network: true},
	})

	joined := strings.Join(args, "\x00")
	if strings.Contains(joined, "--network\x00none") {
		t.Fatalf("expected docker args not to disable network when requested: %#v", args)
	}
}

func TestDockerOOMKilled(t *testing.T) {
	if !dockerOOMKilled([]byte("true\n")) {
		t.Fatal("dockerOOMKilled returned false for true output")
	}
	if dockerOOMKilled([]byte("false\n")) {
		t.Fatal("dockerOOMKilled returned true for false output")
	}
}
