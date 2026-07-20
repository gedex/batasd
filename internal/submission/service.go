package submission

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gedex/batasd/internal/language"
	"github.com/gedex/batasd/internal/status"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

// LanguageRegistry resolves enabled languages by slug.
type LanguageRegistry interface {
	Get(slug string) (language.Language, bool)
	Resolve(slug, version string) (language.Runtime, bool)
}

// ServiceConfig provides dependencies for a Service.
type ServiceConfig struct {
	Repository Repository
	Queue      Queue
	Languages  LanguageRegistry
	Limits     Limits
	MaxLimits  Limits
}

// Service coordinates submission validation, persistence, and queueing.
type Service struct {
	repo      Repository
	queue     Queue
	languages LanguageRegistry
	limits    Limits
	maxLimits Limits
}

// NewService creates a submission service from cfg.
func NewService(cfg ServiceConfig) *Service {
	return &Service{
		repo:      cfg.Repository,
		queue:     cfg.Queue,
		languages: cfg.Languages,
		limits:    cfg.Limits,
		maxLimits: cfg.MaxLimits,
	}
}

// Create validates req, persists a queued submission, and enqueues it for execution.
func (s *Service) Create(ctx context.Context, req CreateRequest) (*Submission, error) {
	if strings.TrimSpace(req.Source) == "" {
		return nil, ValidationError{Field: "source", Message: "source is required"}
	}
	languageSlug := strings.TrimSpace(req.Language)
	if languageSlug == "" {
		return nil, ValidationError{Field: "language", Message: "language is required"}
	}
	languageVersion := strings.TrimSpace(req.LanguageVersion)
	if _, ok := s.languages.Get(languageSlug); !ok {
		return nil, ValidationError{Field: "language", Message: "unsupported language"}
	}
	lang, ok := s.languages.Resolve(languageSlug, languageVersion)
	if !ok {
		return nil, ValidationError{Field: "language_version", Message: "unsupported language version"}
	}
	if req.Callback != nil && strings.TrimSpace(req.Callback.URL) != "" {
		if err := validateCallbackURL(req.Callback.URL); err != nil {
			return nil, ValidationError{Field: "callback.url", Message: err.Error()}
		}
	}
	if req.AdditionalFiles != nil {
		if strings.TrimSpace(req.AdditionalFiles.Encoding) != "zip_base64" {
			return nil, ValidationError{Field: "additional_files.encoding", Message: "must be zip_base64"}
		}
		if strings.TrimSpace(req.AdditionalFiles.Content) == "" {
			return nil, ValidationError{Field: "additional_files.content", Message: "content is required"}
		}
	}

	now := time.Now().UTC()
	limits := mergeLanguageLimits(s.limits, lang.DefaultLimits)
	if req.Limits != nil {
		if err := validateLimitOverrides(*req.Limits, s.maxLimits); err != nil {
			return nil, err
		}
		limits = mergeLimits(limits, *req.Limits)
	}

	token, err := newToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	var callbackURL *string
	if req.Callback != nil && strings.TrimSpace(req.Callback.URL) != "" {
		value := strings.TrimSpace(req.Callback.URL)
		callbackURL = &value
	}

	sub := &Submission{
		Token:           token,
		Language:        lang.Slug,
		LanguageVersion: lang.Version,
		Source:          req.Source,
		Input:           req.Input,
		ExpectedOutput:  req.ExpectedOutput,
		Arguments:       safeStringSlice(req.Arguments),
		CompilerOptions: safeStringSlice(req.CompilerOptions),
		Limits:          limits,
		AdditionalFiles: req.AdditionalFiles,
		CallbackURL:     callbackURL,
		StatusCode:      status.Queued,
		CreatedAt:       now,
		QueuedAt:        &now,
		UpdatedAt:       now,
	}

	if err := s.repo.Create(ctx, sub); err != nil {
		return nil, err
	}
	if err := s.queue.Enqueue(ctx, sub.Token); err != nil {
		return nil, err
	}

	return sub, nil
}

// Get returns the submission identified by token.
func (s *Service) Get(ctx context.Context, token string) (*Submission, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ValidationError{Field: "token", Message: "token is required"}
	}
	return s.repo.FindByToken(ctx, token)
}

// WaitForCompletion polls token until it reaches a terminal status or ctx ends.
func (s *Service) WaitForCompletion(ctx context.Context, token string, pollInterval time.Duration) (*Submission, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ValidationError{Field: "token", Message: "token is required"}
	}
	if pollInterval <= 0 {
		pollInterval = 100 * time.Millisecond
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		sub, err := s.repo.FindByToken(ctx, token)
		if err != nil {
			return nil, err
		}
		if status.Terminal(sub.StatusCode) {
			return sub, nil
		}

		select {
		case <-ctx.Done():
			return sub, ctx.Err()
		case <-ticker.C:
		}
	}
}

// List returns a newest-first page of submissions.
func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	query.BeforeToken = strings.TrimSpace(query.BeforeToken)
	query.StatusCode = strings.TrimSpace(query.StatusCode)
	query.Language = strings.TrimSpace(query.Language)

	if query.Limit == 0 {
		query.Limit = defaultListLimit
	}
	if query.Limit < 1 {
		return ListResult{}, ValidationError{Field: "limit", Message: "must be at least 1"}
	}
	if query.Limit > maxListLimit {
		query.Limit = maxListLimit
	}
	if query.StatusCode != "" && !status.Valid(query.StatusCode) {
		return ListResult{}, ValidationError{Field: "status", Message: "unsupported status"}
	}
	if query.Language != "" {
		if _, ok := s.languages.Get(query.Language); !ok {
			return ListResult{}, ValidationError{Field: "language", Message: "unsupported language"}
		}
	}
	if query.BeforeToken != "" {
		if _, err := s.repo.FindByToken(ctx, query.BeforeToken); err != nil {
			return ListResult{}, err
		}
	}

	return s.repo.List(ctx, query)
}

// ListCallbackAttempts returns callback delivery attempts for token.
func (s *Service) ListCallbackAttempts(ctx context.Context, token string) ([]CallbackAttempt, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ValidationError{Field: "token", Message: "token is required"}
	}
	if _, err := s.repo.FindByToken(ctx, token); err != nil {
		return nil, err
	}
	return s.repo.ListCallbackAttempts(ctx, token)
}

func validateLimitOverrides(limits, max Limits) error {
	checkInt64 := func(field string, value, maximum int64) error {
		if value < 0 {
			return ValidationError{Field: field, Message: "cannot be negative"}
		}
		if value > 0 && maximum > 0 && value > maximum {
			return ValidationError{Field: field, Message: fmt.Sprintf("cannot exceed %d", maximum)}
		}
		return nil
	}
	checkInt := func(field string, value, maximum int) error {
		if value < 0 {
			return ValidationError{Field: field, Message: "cannot be negative"}
		}
		if value > 0 && maximum > 0 && value > maximum {
			return ValidationError{Field: field, Message: fmt.Sprintf("cannot exceed %d", maximum)}
		}
		return nil
	}

	if err := checkInt64("limits.cpu_time_ms", limits.CPUTimeMS, max.CPUTimeMS); err != nil {
		return err
	}
	if err := checkInt64("limits.cpu_extra_time_ms", limits.CPUExtraMS, max.CPUExtraMS); err != nil {
		return err
	}
	if err := checkInt64("limits.wall_time_ms", limits.WallTimeMS, max.WallTimeMS); err != nil {
		return err
	}
	if err := checkInt64("limits.memory_kb", limits.MemoryKB, max.MemoryKB); err != nil {
		return err
	}
	if err := checkInt64("limits.stack_kb", limits.StackKB, max.StackKB); err != nil {
		return err
	}
	if err := checkInt("limits.max_processes", limits.MaxProcesses, max.MaxProcesses); err != nil {
		return err
	}
	if err := checkInt64("limits.max_output_kb", limits.MaxOutputKB, max.MaxOutputKB); err != nil {
		return err
	}
	if err := checkInt64("limits.max_file_kb", limits.MaxFileKB, max.MaxFileKB); err != nil {
		return err
	}
	if err := checkInt("limits.runs", limits.Runs, max.Runs); err != nil {
		return err
	}
	if limits.Network && !max.Network {
		return ValidationError{Field: "limits.network", Message: "cannot be enabled"}
	}
	return nil
}

func mergeLanguageLimits(defaults Limits, overrides *language.LimitOverrides) Limits {
	if overrides == nil {
		return defaults
	}

	out := defaults
	if overrides.CPUTimeMS > 0 {
		out.CPUTimeMS = overrides.CPUTimeMS
	}
	if overrides.CPUExtraMS > 0 {
		out.CPUExtraMS = overrides.CPUExtraMS
	}
	if overrides.WallTimeMS > 0 {
		out.WallTimeMS = overrides.WallTimeMS
	}
	if overrides.MemoryKB > 0 {
		out.MemoryKB = overrides.MemoryKB
	}
	if overrides.StackKB > 0 {
		out.StackKB = overrides.StackKB
	}
	if overrides.MaxProcesses > 0 {
		out.MaxProcesses = overrides.MaxProcesses
	}
	if overrides.MaxOutputKB > 0 {
		out.MaxOutputKB = overrides.MaxOutputKB
	}
	if overrides.MaxFileKB > 0 {
		out.MaxFileKB = overrides.MaxFileKB
	}
	if overrides.Runs > 0 {
		out.Runs = overrides.Runs
	}
	if overrides.Network != nil {
		out.Network = *overrides.Network
	}
	return out
}

func mergeLimits(defaults, overrides Limits) Limits {
	out := defaults
	if overrides.CPUTimeMS > 0 {
		out.CPUTimeMS = overrides.CPUTimeMS
	}
	if overrides.CPUExtraMS > 0 {
		out.CPUExtraMS = overrides.CPUExtraMS
	}
	if overrides.WallTimeMS > 0 {
		out.WallTimeMS = overrides.WallTimeMS
	}
	if overrides.MemoryKB > 0 {
		out.MemoryKB = overrides.MemoryKB
	}
	if overrides.StackKB > 0 {
		out.StackKB = overrides.StackKB
	}
	if overrides.MaxProcesses > 0 {
		out.MaxProcesses = overrides.MaxProcesses
	}
	if overrides.MaxOutputKB > 0 {
		out.MaxOutputKB = overrides.MaxOutputKB
	}
	if overrides.MaxFileKB > 0 {
		out.MaxFileKB = overrides.MaxFileKB
	}
	if overrides.Runs > 0 {
		out.Runs = overrides.Runs
	}
	out.Network = overrides.Network
	return out
}

func safeStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func validateCallbackURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return errors.New("must be a valid URL")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return errors.New("must use http or https")
	}
	if parsed.Host == "" {
		return errors.New("host is required")
	}
	return nil
}

func newToken() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes[:])
	return "sub_" + strings.ToLower(encoded), nil
}

// ValidationError describes an invalid field in an API request.
type ValidationError struct {
	Field   string
	Message string
}

// Error returns the field-qualified validation message.
func (e ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
