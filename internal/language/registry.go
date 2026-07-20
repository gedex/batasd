// Package language loads and serves the embedded language catalog.
package language

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

//go:embed catalog/languages.json
var catalogFS embed.FS

// Language describes a supported language family and its available versions.
type Language struct {
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	DefaultVersion string    `json:"default_version"`
	Versions       []Version `json:"versions"`
	Enabled        bool      `json:"enabled"`
}

// Version describes how one language version is compiled and run.
type Version struct {
	Version       string          `json:"version"`
	SourceFile    string          `json:"source_file"`
	Compile       []string        `json:"compile"`
	Run           []string        `json:"run"`
	DefaultLimits *LimitOverrides `json:"default_limits,omitempty"`
	Enabled       bool            `json:"enabled"`
}

// Runtime is a resolved language version ready for execution.
type Runtime struct {
	Slug          string
	Name          string
	Version       string
	SourceFile    string
	Compile       []string
	Run           []string
	DefaultLimits *LimitOverrides
}

// LimitOverrides describes language-specific execution limit defaults.
type LimitOverrides struct {
	CPUTimeMS    int64 `json:"cpu_time_ms,omitempty"`
	CPUExtraMS   int64 `json:"cpu_extra_time_ms,omitempty"`
	WallTimeMS   int64 `json:"wall_time_ms,omitempty"`
	MemoryKB     int64 `json:"memory_kb,omitempty"`
	StackKB      int64 `json:"stack_kb,omitempty"`
	MaxProcesses int   `json:"max_processes,omitempty"`
	MaxOutputKB  int64 `json:"max_output_kb,omitempty"`
	MaxFileKB    int64 `json:"max_file_kb,omitempty"`
	Network      *bool `json:"network,omitempty"`
	Runs         int   `json:"runs,omitempty"`
}

// Registry stores languages by slug.
type Registry struct {
	bySlug map[string]Language
	list   []Language
}

// LoadCatalog loads the embedded language catalog and validates its entries.
func LoadCatalog() (*Registry, error) {
	data, err := catalogFS.ReadFile("catalog/languages.json")
	if err != nil {
		return nil, err
	}

	var languages []Language
	if err := json.Unmarshal(data, &languages); err != nil {
		return nil, err
	}

	bySlug := make(map[string]Language, len(languages))
	for _, lang := range languages {
		if lang.Slug == "" {
			return nil, errors.New("language slug cannot be empty")
		}
		if lang.Name == "" {
			return nil, fmt.Errorf("language %q name cannot be empty", lang.Slug)
		}
		if lang.DefaultVersion == "" {
			return nil, fmt.Errorf("language %q default_version cannot be empty", lang.Slug)
		}
		if len(lang.Versions) == 0 {
			return nil, fmt.Errorf("language %q versions cannot be empty", lang.Slug)
		}

		versions := make(map[string]Version, len(lang.Versions))
		for _, version := range lang.Versions {
			if version.Version == "" {
				return nil, fmt.Errorf("language %q version cannot be empty", lang.Slug)
			}
			if version.SourceFile == "" {
				return nil, fmt.Errorf("language %q version %q source_file cannot be empty", lang.Slug, version.Version)
			}
			if len(version.Run) == 0 {
				return nil, fmt.Errorf("language %q version %q run command cannot be empty", lang.Slug, version.Version)
			}
			if err := validateLimitOverrides(version.DefaultLimits); err != nil {
				return nil, fmt.Errorf("language %q version %q default_limits: %w", lang.Slug, version.Version, err)
			}
			if _, exists := versions[version.Version]; exists {
				return nil, fmt.Errorf("duplicate language %q version %q", lang.Slug, version.Version)
			}
			versions[version.Version] = version
		}
		defaultVersion, ok := versions[lang.DefaultVersion]
		if !ok {
			return nil, fmt.Errorf("language %q default_version %q is not defined", lang.Slug, lang.DefaultVersion)
		}
		if !defaultVersion.Enabled {
			return nil, fmt.Errorf("language %q default_version %q is not enabled", lang.Slug, lang.DefaultVersion)
		}
		if _, exists := bySlug[lang.Slug]; exists {
			return nil, fmt.Errorf("duplicate language slug %q", lang.Slug)
		}
		sort.Slice(lang.Versions, func(i, j int) bool {
			return lang.Versions[i].Version < lang.Versions[j].Version
		})
		bySlug[lang.Slug] = lang
	}

	sort.Slice(languages, func(i, j int) bool {
		return languages[i].Slug < languages[j].Slug
	})

	return &Registry{bySlug: bySlug, list: languages}, nil
}

func validateLimitOverrides(limits *LimitOverrides) error {
	if limits == nil {
		return nil
	}
	if limits.CPUTimeMS < 0 {
		return errors.New("cpu_time_ms cannot be negative")
	}
	if limits.CPUExtraMS < 0 {
		return errors.New("cpu_extra_time_ms cannot be negative")
	}
	if limits.WallTimeMS < 0 {
		return errors.New("wall_time_ms cannot be negative")
	}
	if limits.MemoryKB < 0 {
		return errors.New("memory_kb cannot be negative")
	}
	if limits.StackKB < 0 {
		return errors.New("stack_kb cannot be negative")
	}
	if limits.MaxProcesses < 0 {
		return errors.New("max_processes cannot be negative")
	}
	if limits.MaxOutputKB < 0 {
		return errors.New("max_output_kb cannot be negative")
	}
	if limits.MaxFileKB < 0 {
		return errors.New("max_file_kb cannot be negative")
	}
	if limits.Runs < 0 {
		return errors.New("runs cannot be negative")
	}
	return nil
}

// List returns enabled languages sorted by slug.
func (r *Registry) List() []Language {
	out := make([]Language, 0, len(r.list))
	for _, lang := range r.list {
		if lang.Enabled {
			out = append(out, enabledLanguage(lang))
		}
	}
	return out
}

// Get returns the enabled language family identified by slug.
func (r *Registry) Get(slug string) (Language, bool) {
	lang, ok := r.bySlug[strings.TrimSpace(slug)]
	if !ok || !lang.Enabled {
		return Language{}, false
	}
	return enabledLanguage(lang), true
}

// Resolve returns a concrete enabled language version for execution.
func (r *Registry) Resolve(slug, version string) (Runtime, bool) {
	lang, ok := r.bySlug[strings.TrimSpace(slug)]
	if !ok || !lang.Enabled {
		return Runtime{}, false
	}

	version = strings.TrimSpace(version)
	if version == "" {
		version = lang.DefaultVersion
	}
	for _, candidate := range lang.Versions {
		if candidate.Version == version && candidate.Enabled {
			return Runtime{
				Slug:          lang.Slug,
				Name:          lang.Name,
				Version:       candidate.Version,
				SourceFile:    candidate.SourceFile,
				Compile:       candidate.Compile,
				Run:           candidate.Run,
				DefaultLimits: candidate.DefaultLimits,
			}, true
		}
	}
	return Runtime{}, false
}

func enabledLanguage(lang Language) Language {
	versions := make([]Version, 0, len(lang.Versions))
	for _, version := range lang.Versions {
		if version.Enabled {
			versions = append(versions, version)
		}
	}
	lang.Versions = versions
	return lang
}
