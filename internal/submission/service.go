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

type LanguageRegistry interface {
	Get(slug string) (language.Language, bool)
}

type ServiceConfig struct {
	Repository Repository
	Queue      Queue
	Languages  LanguageRegistry
	Limits     Limits
}

type Service struct {
	repo      Repository
	queue     Queue
	languages LanguageRegistry
	limits    Limits
}

func NewService(cfg ServiceConfig) *Service {
	return &Service{
		repo:      cfg.Repository,
		queue:     cfg.Queue,
		languages: cfg.Languages,
		limits:    cfg.Limits,
	}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*Submission, error) {
	if strings.TrimSpace(req.Source) == "" {
		return nil, ValidationError{Field: "source", Message: "source is required"}
	}
	if _, ok := s.languages.Get(req.Language); !ok {
		return nil, ValidationError{Field: "language", Message: "unsupported language"}
	}
	if req.Callback != nil && strings.TrimSpace(req.Callback.URL) != "" {
		if err := validateCallbackURL(req.Callback.URL); err != nil {
			return nil, ValidationError{Field: "callback.url", Message: err.Error()}
		}
	}

	now := time.Now().UTC()
	limits := s.limits
	if req.Limits != nil {
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
		Language:        req.Language,
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

func (s *Service) Get(ctx context.Context, token string) (*Submission, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ValidationError{Field: "token", Message: "token is required"}
	}
	return s.repo.FindByToken(ctx, token)
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

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
