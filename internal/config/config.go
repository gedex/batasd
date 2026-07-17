// Package config loads and validates batasd runtime configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gedex/batasd/internal/submission"
)

// Config contains all runtime settings needed to start batasd.
type Config struct {
	AppEnv      string
	HTTP        HTTPConfig
	Database    DatabaseConfig
	Auth        AuthConfig
	Sandbox     SandboxConfig
	Queue       QueueConfig
	Submissions SubmissionConfig
}

// HTTPConfig controls the public HTTP server.
type HTTPConfig struct {
	Addr string
}

// DatabaseConfig controls PostgreSQL connectivity and migration behavior.
type DatabaseConfig struct {
	URL           string
	RunMigrations bool
}

// AuthConfig describes bearer token authentication for API requests.
type AuthConfig struct {
	Tokens map[string]struct{}
}

// SandboxConfig selects the execution sandbox and its working directory.
type SandboxConfig struct {
	Driver              string
	WorkDir             string
	DockerImage         string
	DockerBinary        string
	IsolateBinary       string
	IsolateBoxIDStart   int
	IsolateBoxIDCount   int
	IsolateControlGroup bool
}

// QueueConfig controls worker concurrency.
type QueueConfig struct {
	Workers int
}

// SubmissionConfig contains default submission execution limits.
type SubmissionConfig struct {
	DefaultLimits submission.Limits
}

// Load reads configuration from environment variables and applies defaults.
func Load() (Config, error) {
	cfg := Config{
		AppEnv: envString("APP_ENV", "local"),
		HTTP: HTTPConfig{
			Addr: envString("HTTP_ADDR", ":18080"),
		},
		Database: DatabaseConfig{
			URL:           envString("DATABASE_URL", "postgres://sandbox:sandbox@localhost:5432/sandbox?sslmode=disable"),
			RunMigrations: envBool("DATABASE_RUN_MIGRATIONS", true),
		},
		Auth: AuthConfig{
			Tokens: envTokenSet("AUTHN_TOKENS", "dev-token"),
		},
		Sandbox: SandboxConfig{
			Driver:              envString("SANDBOX_DRIVER", "direct"),
			WorkDir:             envString("SANDBOX_WORK_DIR", os.TempDir()+"/batasd-work"),
			DockerImage:         envString("SANDBOX_DOCKER_IMAGE", "batasd-runner:local"),
			DockerBinary:        envString("SANDBOX_DOCKER_BINARY", "docker"),
			IsolateBinary:       envString("SANDBOX_ISOLATE_BINARY", "isolate"),
			IsolateBoxIDStart:   envInt("SANDBOX_ISOLATE_BOX_ID_START", 0),
			IsolateBoxIDCount:   envInt("SANDBOX_ISOLATE_BOX_ID_COUNT", 16),
			IsolateControlGroup: envBool("SANDBOX_ISOLATE_CGROUP", true),
		},
		Queue: QueueConfig{
			Workers: envInt("QUEUE_WORKERS", 1),
		},
		Submissions: SubmissionConfig{
			DefaultLimits: submission.Limits{
				CPUTimeMS:    envInt64("LIMIT_CPU_TIME_MS", 5000),
				CPUExtraMS:   envInt64("LIMIT_CPU_EXTRA_MS", 1000),
				WallTimeMS:   envInt64("LIMIT_WALL_TIME_MS", 10000),
				MemoryKB:     envInt64("LIMIT_MEMORY_KB", 128000),
				StackKB:      envInt64("LIMIT_STACK_KB", 64000),
				MaxProcesses: envInt("LIMIT_MAX_PROCESSES", 60),
				MaxOutputKB:  envInt64("LIMIT_MAX_OUTPUT_KB", 1024),
				MaxFileKB:    envInt64("LIMIT_MAX_FILE_KB", 1024),
				Network:      envBool("LIMIT_NETWORK", false),
				Runs:         envInt("LIMIT_RUNS", 1),
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate reports whether c is internally consistent and safe to start.
func (c Config) Validate() error {
	if c.AppEnv == "" {
		return errors.New("APP_ENV cannot be empty")
	}

	switch c.Sandbox.Driver {
	case "direct", "docker", "isolate":
	default:
		return fmt.Errorf("unsupported SANDBOX_DRIVER %q", c.Sandbox.Driver)
	}

	if c.AppEnv == "production" && c.Sandbox.Driver != "isolate" {
		return errors.New("production requires SANDBOX_DRIVER=isolate")
	}

	if c.Sandbox.Driver == "isolate" {
		if c.Sandbox.IsolateBoxIDStart < 0 {
			return errors.New("SANDBOX_ISOLATE_BOX_ID_START cannot be negative")
		}
		if c.Sandbox.IsolateBoxIDCount < 1 {
			return errors.New("SANDBOX_ISOLATE_BOX_ID_COUNT must be at least 1")
		}
	}

	if c.Queue.Workers < 1 {
		return errors.New("QUEUE_WORKERS must be at least 1")
	}

	if c.Sandbox.Driver == "isolate" && c.Sandbox.IsolateBoxIDCount < c.Queue.Workers {
		return errors.New("SANDBOX_ISOLATE_BOX_ID_COUNT must be greater than or equal to QUEUE_WORKERS")
	}

	return nil
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envTokenSet(key, fallback string) map[string]struct{} {
	raw := envString(key, fallback)
	tokens := map[string]struct{}{}
	for _, token := range strings.Split(raw, ",") {
		token = strings.TrimSpace(token)
		if token != "" {
			tokens[token] = struct{}{}
		}
	}
	return tokens
}
