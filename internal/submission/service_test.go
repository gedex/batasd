package submission

import (
	"context"
	"testing"
	"time"

	"github.com/gedex/batasd/internal/language"
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
	created *Submission
}

func (r *fakeRepository) Create(_ context.Context, sub *Submission) error {
	r.created = sub
	return nil
}

func (r *fakeRepository) FindByToken(_ context.Context, token string) (*Submission, error) {
	if r.created == nil || r.created.Token != token {
		return nil, ErrNotFound
	}
	return r.created, nil
}

func (r *fakeRepository) MarkProcessing(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (r *fakeRepository) StoreResult(_ context.Context, _ string, _ Result) error {
	return nil
}
