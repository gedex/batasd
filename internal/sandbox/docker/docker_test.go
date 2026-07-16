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
	for _, want := range []string{"--network\x00none", "--memory\x00128000k", "--pids-limit\x0060", "batasd-runner:local\x00python3\x00main.py"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected docker args to contain %q in %#v", want, args)
		}
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
