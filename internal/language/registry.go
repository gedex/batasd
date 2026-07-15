package language

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

//go:embed catalog/languages.json
var catalogFS embed.FS

type Language struct {
	Slug       string   `json:"slug"`
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	SourceFile string   `json:"source_file"`
	Compile    []string `json:"compile"`
	Run        []string `json:"run"`
	Enabled    bool     `json:"enabled"`
}

type Registry struct {
	bySlug map[string]Language
	list   []Language
}

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
		if lang.SourceFile == "" {
			return nil, fmt.Errorf("language %q source_file cannot be empty", lang.Slug)
		}
		if len(lang.Run) == 0 {
			return nil, fmt.Errorf("language %q run command cannot be empty", lang.Slug)
		}
		if _, exists := bySlug[lang.Slug]; exists {
			return nil, fmt.Errorf("duplicate language slug %q", lang.Slug)
		}
		bySlug[lang.Slug] = lang
	}

	sort.Slice(languages, func(i, j int) bool {
		return languages[i].Slug < languages[j].Slug
	})

	return &Registry{bySlug: bySlug, list: languages}, nil
}

func (r *Registry) List() []Language {
	out := make([]Language, 0, len(r.list))
	for _, lang := range r.list {
		if lang.Enabled {
			out = append(out, lang)
		}
	}
	return out
}

func (r *Registry) Get(slug string) (Language, bool) {
	lang, ok := r.bySlug[slug]
	if !ok || !lang.Enabled {
		return Language{}, false
	}
	return lang, true
}
