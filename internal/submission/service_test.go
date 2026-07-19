package submission

import (
	"context"
	"testing"
	"time"

	"github.com/gedex/batasd/internal/language"
	"github.com/gedex/batasd/internal/status"
)

func TestCreateAppliesLanguageDefaultLimits(t *testing.T) {
	repo := &fakeRepository{}
	queue := &fakeQueue{}
	service := NewService(ServiceConfig{
		Repository: repo,
		Queue:      queue,
		Languages: fakeLanguages{
			"node-22": {
				Slug: "node-22",
				DefaultLimits: &language.LimitOverrides{
					MemoryKB:     2097152,
					MaxProcesses: 256,
				},
				Enabled: true,
			},
		},
		Limits: defaultTestLimits(),
		MaxLimits: Limits{
			MemoryKB:     3145728,
			MaxProcesses: 512,
		},
	})

	sub, err := service.Create(context.Background(), CreateRequest{
		Language: "node-22",
		Source:   "console.log('ok')",
	})
	if err != nil {
		t.Fatal(err)
	}

	if sub.Limits.MemoryKB != 2097152 {
		t.Fatalf("MemoryKB = %d, want 2097152", sub.Limits.MemoryKB)
	}
	if sub.Limits.MaxProcesses != 256 {
		t.Fatalf("MaxProcesses = %d, want 256", sub.Limits.MaxProcesses)
	}
	if sub.Limits.CPUTimeMS != 5000 {
		t.Fatalf("CPUTimeMS = %d, want 5000", sub.Limits.CPUTimeMS)
	}
	if repo.created == nil || repo.created.Token != sub.Token {
		t.Fatal("submission was not persisted")
	}
	if queue.enqueued != sub.Token {
		t.Fatalf("enqueued token = %q, want %q", queue.enqueued, sub.Token)
	}
}

func TestCreateRequestLimitsOverrideLanguageDefaultLimits(t *testing.T) {
	service := NewService(ServiceConfig{
		Repository: &fakeRepository{},
		Queue:      &fakeQueue{},
		Languages: fakeLanguages{
			"node-22": {
				Slug: "node-22",
				DefaultLimits: &language.LimitOverrides{
					MemoryKB:     2097152,
					MaxProcesses: 256,
				},
				Enabled: true,
			},
		},
		Limits: defaultTestLimits(),
	})

	sub, err := service.Create(context.Background(), CreateRequest{
		Language: "node-22",
		Source:   "console.log('ok')",
		Limits: &Limits{
			MemoryKB:     3145728,
			MaxProcesses: 512,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if sub.Limits.MemoryKB != 3145728 {
		t.Fatalf("MemoryKB = %d, want 3145728", sub.Limits.MemoryKB)
	}
	if sub.Limits.MaxProcesses != 512 {
		t.Fatalf("MaxProcesses = %d, want 512", sub.Limits.MaxProcesses)
	}
}

func TestCreateRejectsRequestLimitsAboveMaxLimits(t *testing.T) {
	service := NewService(ServiceConfig{
		Repository: &fakeRepository{},
		Queue:      &fakeQueue{},
		Languages: fakeLanguages{
			"python-3.12": {
				Slug:    "python-3.12",
				Enabled: true,
			},
		},
		Limits:    defaultTestLimits(),
		MaxLimits: defaultTestMaxLimits(),
	})

	_, err := service.Create(context.Background(), CreateRequest{
		Language: "python-3.12",
		Source:   "print('ok')",
		Limits: &Limits{
			MemoryKB: defaultTestMaxLimits().MemoryKB + 1,
		},
	})
	if err == nil {
		t.Fatal("Create returned nil error, want validation error")
	}

	validation, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("err = %T, want ValidationError", err)
	}
	if validation.Field != "limits.memory_kb" {
		t.Fatalf("field = %q, want limits.memory_kb", validation.Field)
	}
}

func TestCreateUsesGlobalLimitsWhenLanguageHasNoDefaultLimits(t *testing.T) {
	service := NewService(ServiceConfig{
		Repository: &fakeRepository{},
		Queue:      &fakeQueue{},
		Languages: fakeLanguages{
			"python-3.12": {
				Slug:    "python-3.12",
				Enabled: true,
			},
		},
		Limits: defaultTestLimits(),
	})

	sub, err := service.Create(context.Background(), CreateRequest{
		Language: "python-3.12",
		Source:   "print('ok')",
	})
	if err != nil {
		t.Fatal(err)
	}

	if sub.Limits.MemoryKB != 128000 {
		t.Fatalf("MemoryKB = %d, want 128000", sub.Limits.MemoryKB)
	}
	if sub.Limits.MaxProcesses != 60 {
		t.Fatalf("MaxProcesses = %d, want 60", sub.Limits.MaxProcesses)
	}
}

func TestCreateRejectsUnsupportedAdditionalFilesEncoding(t *testing.T) {
	service := NewService(ServiceConfig{
		Repository: &fakeRepository{},
		Queue:      &fakeQueue{},
		Languages: fakeLanguages{
			"python-3.12": {
				Slug:    "python-3.12",
				Enabled: true,
			},
		},
		Limits: defaultTestLimits(),
	})

	_, err := service.Create(context.Background(), CreateRequest{
		Language: "python-3.12",
		Source:   "print('ok')",
		AdditionalFiles: &AdditionalFiles{
			Encoding: "plain",
			Content:  "hello",
		},
	})
	if err == nil {
		t.Fatal("Create returned nil error, want validation error")
	}

	validation, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("err = %T, want ValidationError", err)
	}
	if validation.Field != "additional_files.encoding" {
		t.Fatalf("field = %q, want additional_files.encoding", validation.Field)
	}
}

func TestListUsesDefaults(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(ServiceConfig{
		Repository: repo,
		Queue:      &fakeQueue{},
		Languages:  fakeLanguages{},
		Limits:     defaultTestLimits(),
	})

	result, err := service.List(context.Background(), ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if repo.listQuery.Limit != defaultListLimit {
		t.Fatalf("repo limit = %d, want %d", repo.listQuery.Limit, defaultListLimit)
	}
	if result.Limit != defaultListLimit {
		t.Fatalf("result limit = %d, want %d", result.Limit, defaultListLimit)
	}
}

func TestWaitForCompletionReturnsTerminalSubmission(t *testing.T) {
	repo := &fakeRepository{
		findResults: []*Submission{
			{Token: "sub_test", StatusCode: status.Queued},
			{Token: "sub_test", StatusCode: status.Accepted},
		},
	}
	service := NewService(ServiceConfig{
		Repository: repo,
		Queue:      &fakeQueue{},
		Languages:  fakeLanguages{},
		Limits:     defaultTestLimits(),
	})

	sub, err := service.WaitForCompletion(context.Background(), "sub_test", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if sub.StatusCode != status.Accepted {
		t.Fatalf("status = %q, want %q", sub.StatusCode, status.Accepted)
	}
	if repo.findCalls != 2 {
		t.Fatalf("find calls = %d, want 2", repo.findCalls)
	}
}

func TestWaitForCompletionReturnsLastSubmissionOnContextDone(t *testing.T) {
	repo := &fakeRepository{
		findResults: []*Submission{
			{Token: "sub_test", StatusCode: status.Processing},
		},
	}
	service := NewService(ServiceConfig{
		Repository: repo,
		Queue:      &fakeQueue{},
		Languages:  fakeLanguages{},
		Limits:     defaultTestLimits(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sub, err := service.WaitForCompletion(ctx, "sub_test", time.Second)
	if err == nil {
		t.Fatal("WaitForCompletion returned nil error, want context error")
	}
	if sub == nil {
		t.Fatal("submission = nil, want last observed submission")
	}
	if sub.StatusCode != status.Processing {
		t.Fatalf("status = %q, want %q", sub.StatusCode, status.Processing)
	}
}

func TestListValidatesAndPassesFilters(t *testing.T) {
	repo := &fakeRepository{
		created: &Submission{Token: "sub_cursor"},
	}
	service := NewService(ServiceConfig{
		Repository: repo,
		Queue:      &fakeQueue{},
		Languages: fakeLanguages{
			"python-3.12": {
				Slug:    "python-3.12",
				Enabled: true,
			},
		},
		Limits: defaultTestLimits(),
	})

	_, err := service.List(context.Background(), ListQuery{
		Limit:       2,
		BeforeToken: " sub_cursor ",
		StatusCode:  status.Accepted,
		Language:    "python-3.12",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.listQuery.Limit != 2 {
		t.Fatalf("limit = %d, want 2", repo.listQuery.Limit)
	}
	if repo.listQuery.BeforeToken != "sub_cursor" {
		t.Fatalf("before token = %q, want sub_cursor", repo.listQuery.BeforeToken)
	}
	if repo.listQuery.StatusCode != status.Accepted {
		t.Fatalf("status = %q, want %q", repo.listQuery.StatusCode, status.Accepted)
	}
	if repo.listQuery.Language != "python-3.12" {
		t.Fatalf("language = %q, want python-3.12", repo.listQuery.Language)
	}
}

func TestListClampsLimit(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(ServiceConfig{
		Repository: repo,
		Queue:      &fakeQueue{},
		Languages:  fakeLanguages{},
		Limits:     defaultTestLimits(),
	})

	result, err := service.List(context.Background(), ListQuery{Limit: 10000000})
	if err != nil {
		t.Fatal(err)
	}
	if repo.listQuery.Limit != maxListLimit {
		t.Fatalf("repo limit = %d, want %d", repo.listQuery.Limit, maxListLimit)
	}
	if result.Limit != maxListLimit {
		t.Fatalf("result limit = %d, want %d", result.Limit, maxListLimit)
	}
}

func TestListRejectsInvalidQuery(t *testing.T) {
	service := NewService(ServiceConfig{
		Repository: &fakeRepository{},
		Queue:      &fakeQueue{},
		Languages:  fakeLanguages{},
		Limits:     defaultTestLimits(),
	})

	tests := []struct {
		name  string
		query ListQuery
		field string
	}{
		{
			name:  "zero limit",
			query: ListQuery{Limit: -1},
			field: "limit",
		},
		{
			name:  "unknown status",
			query: ListQuery{StatusCode: "not_real"},
			field: "status",
		},
		{
			name:  "unknown language",
			query: ListQuery{Language: "ruby-3.3"},
			field: "language",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.List(context.Background(), tt.query)
			if err == nil {
				t.Fatal("List returned nil error, want validation error")
			}
			validation, ok := err.(ValidationError)
			if !ok {
				t.Fatalf("err = %T, want ValidationError", err)
			}
			if validation.Field != tt.field {
				t.Fatalf("field = %q, want %q", validation.Field, tt.field)
			}
		})
	}
}

func TestListRequiresExistingBeforeToken(t *testing.T) {
	service := NewService(ServiceConfig{
		Repository: &fakeRepository{},
		Queue:      &fakeQueue{},
		Languages:  fakeLanguages{},
		Limits:     defaultTestLimits(),
	})

	_, err := service.List(context.Background(), ListQuery{BeforeToken: "sub_missing"})
	if err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestListCallbackAttemptsRequiresExistingSubmission(t *testing.T) {
	repo := &fakeRepository{
		attempts: []CallbackAttempt{
			{SubmissionToken: "sub_test", Attempt: 1},
		},
	}
	service := NewService(ServiceConfig{
		Repository: repo,
		Queue:      &fakeQueue{},
		Languages:  fakeLanguages{},
		Limits:     defaultTestLimits(),
	})

	_, err := service.ListCallbackAttempts(context.Background(), "sub_missing")
	if err == nil {
		t.Fatal("ListCallbackAttempts returned nil error, want not found")
	}
	if err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}

	_ = repo.Create(context.Background(), &Submission{Token: "sub_test"})
	attempts, err := service.ListCallbackAttempts(context.Background(), "sub_test")
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 {
		t.Fatalf("len(attempts) = %d, want 1", len(attempts))
	}
}

func defaultTestLimits() Limits {
	return Limits{
		CPUTimeMS:    5000,
		CPUExtraMS:   1000,
		WallTimeMS:   10000,
		MemoryKB:     128000,
		StackKB:      64000,
		MaxProcesses: 60,
		MaxOutputKB:  1024,
		MaxFileKB:    1024,
		Runs:         1,
	}
}

func defaultTestMaxLimits() Limits {
	return Limits{
		CPUTimeMS:    15000,
		CPUExtraMS:   3000,
		WallTimeMS:   30000,
		MemoryKB:     2097152,
		StackKB:      64000,
		MaxProcesses: 256,
		MaxOutputKB:  4096,
		MaxFileKB:    10240,
		Runs:         1,
	}
}

type fakeLanguages map[string]language.Language

func (f fakeLanguages) Get(slug string) (language.Language, bool) {
	lang, ok := f[slug]
	if !ok || !lang.Enabled {
		return language.Language{}, false
	}
	return lang, true
}

type fakeQueue struct {
	enqueued string
}

func (q *fakeQueue) Enqueue(_ context.Context, token string) error {
	q.enqueued = token
	return nil
}

type fakeRepository struct {
	created     *Submission
	findResults []*Submission
	findCalls   int
	listQuery   ListQuery
	listResult  ListResult
	attempts    []CallbackAttempt
}

func (r *fakeRepository) Create(_ context.Context, sub *Submission) error {
	r.created = sub
	return nil
}

func (r *fakeRepository) FindByToken(_ context.Context, token string) (*Submission, error) {
	if len(r.findResults) > 0 {
		index := r.findCalls
		if index >= len(r.findResults) {
			index = len(r.findResults) - 1
		}
		r.findCalls++
		sub := r.findResults[index]
		if sub.Token != token {
			return nil, ErrNotFound
		}
		return sub, nil
	}
	if r.created == nil || r.created.Token != token {
		return nil, ErrNotFound
	}
	return r.created, nil
}

func (r *fakeRepository) List(_ context.Context, query ListQuery) (ListResult, error) {
	r.listQuery = query
	result := r.listResult
	result.Limit = query.Limit
	return result, nil
}

func (r *fakeRepository) MarkProcessing(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (r *fakeRepository) StoreResult(_ context.Context, _ string, _ Result) error {
	return nil
}

func (r *fakeRepository) RecoverUnfinished(_ context.Context, _ time.Time) ([]string, error) {
	return nil, nil
}

func (r *fakeRepository) CreateCallbackAttempt(_ context.Context, attempt CallbackAttempt) error {
	r.attempts = append(r.attempts, attempt)
	return nil
}

func (r *fakeRepository) ListCallbackAttempts(_ context.Context, token string) ([]CallbackAttempt, error) {
	var attempts []CallbackAttempt
	for _, attempt := range r.attempts {
		if attempt.SubmissionToken == token {
			attempts = append(attempts, attempt)
		}
	}
	return attempts, nil
}
