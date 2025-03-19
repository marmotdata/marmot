package assetdocs

import (
	"context"
	"fmt"
	"time"

	validator "github.com/go-playground/validator/v10"
)

type CreateDocumentationInput struct {
	MRN     string `json:"mrn" validate:"required"`
	Content string `json:"content" validate:"required"`
	Source  string `json:"source" validate:"required"`
}

type GlobalDocumentation struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Documentation struct {
	ID         string    `json:"id"`
	MRN        string    `json:"mrn"`
	Content    string    `json:"content"`
	Source     string    `json:"source"`
	GlobalDocs []string  `json:"global_docs,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Service interface {
	Get(ctx context.Context, mrn string) ([]Documentation, error)
	Create(ctx context.Context, doc Documentation) error
	CreateGlobal(ctx context.Context, doc GlobalDocumentation) error
	CombineDocumentation(ctx context.Context, docs []Documentation) ([]Documentation, error)
}

type service struct {
	repo      Repository
	validator *validator.Validate
}

type ServiceOption func(*service)

func NewService(repo Repository, opts ...ServiceOption) Service {
	s := &service{
		repo:      repo,
		validator: validator.New(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *service) Get(ctx context.Context, mrn string) ([]Documentation, error) {
	docs, err := s.repo.GetDocumentation(ctx, mrn)
	if err != nil {
		return nil, err
	}

	return s.CombineDocumentation(ctx, docs)
}

func (s *service) Create(ctx context.Context, doc Documentation) error {
	return s.repo.CreateDocumentation(ctx, doc)
}

func (s *service) CreateGlobal(ctx context.Context, doc GlobalDocumentation) error {
	return s.repo.CreateGlobalDocumentation(ctx, doc)
}

func (s *service) CombineDocumentation(ctx context.Context, docs []Documentation) ([]Documentation, error) {
	if len(docs) == 0 {
		return docs, nil
	}

	// Get all referenced global docs
	globalDocs := make(map[string]string)
	for _, doc := range docs {
		for _, globalSource := range doc.GlobalDocs {
			if _, exists := globalDocs[globalSource]; !exists {
				content, err := s.repo.GetGlobalDocumentation(ctx, globalSource)
				if err != nil {
					return nil, fmt.Errorf("fetching global doc %s: %w", globalSource, err)
				}
				globalDocs[globalSource] = content
			}
		}
	}

	// Create combined docs
	var result []Documentation
	for _, doc := range docs {
		combined := doc
		if len(doc.GlobalDocs) > 0 {
			var fullContent string
			fullContent = doc.Content
			for _, globalSource := range doc.GlobalDocs {
				if content, exists := globalDocs[globalSource]; exists {
					fullContent += "\n\n" + content
				}
			}
			combined.Content = fullContent
		}
		result = append(result, combined)
	}

	return result, nil
}
