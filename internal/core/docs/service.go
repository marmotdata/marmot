package docs

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreatePage(ctx context.Context, entityType EntityType, entityID string, input CreatePageInput, createdBy *string) (*Page, error) {
	if entityType != EntityTypeAsset && entityType != EntityTypeDataProduct {
		return nil, fmt.Errorf("%w: invalid entity type", ErrInvalidInput)
	}

	count, err := s.repo.CountPages(ctx, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("counting pages: %w", err)
	}
	if count >= MaxPagesPerEntity {
		return nil, ErrMaxPagesExceeded
	}

	if strings.TrimSpace(input.Title) == "" {
		input.Title = "Untitled"
	}

	return s.repo.CreatePage(ctx, entityType, entityID, input, createdBy)
}

func (s *Service) GetPage(ctx context.Context, pageID string) (*Page, error) {
	return s.repo.GetPage(ctx, pageID)
}

func (s *Service) UpdatePage(ctx context.Context, pageID string, input UpdatePageInput) (*Page, error) {
	if input.Title != nil && strings.TrimSpace(*input.Title) == "" {
		return nil, fmt.Errorf("%w: title cannot be empty", ErrInvalidInput)
	}
	return s.repo.UpdatePage(ctx, pageID, input)
}

func (s *Service) DeletePage(ctx context.Context, pageID string) error {
	return s.repo.DeletePage(ctx, pageID)
}

func (s *Service) MovePage(ctx context.Context, pageID string, input MovePageInput) (*Page, error) {
	return s.repo.MovePage(ctx, pageID, input)
}

func (s *Service) GetPageTree(ctx context.Context, entityType EntityType, entityID string) (*PageTree, error) {
	return s.repo.GetPageTree(ctx, entityType, entityID)
}

func (s *Service) SearchPages(ctx context.Context, entityType EntityType, entityID string, query string, limit, offset int) ([]*Page, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.SearchPages(ctx, entityType, entityID, query, limit, offset)
}

func (s *Service) UploadImage(ctx context.Context, pageID string, input UploadImageInput) (*ImageMeta, error) {
	if !ValidImageTypes[input.ContentType] {
		return nil, ErrInvalidImageType
	}

	if len(input.Data) > MaxImageSizeBytes {
		return nil, ErrImageTooLarge
	}

	detectedType := http.DetectContentType(input.Data)
	if !ValidImageTypes[detectedType] {
		return nil, fmt.Errorf("%w: detected type %s", ErrInvalidImageType, detectedType)
	}

	page, err := s.repo.GetPage(ctx, pageID)
	if err != nil {
		return nil, err
	}

	imageCount, err := s.repo.CountPageImages(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("counting images: %w", err)
	}
	if imageCount >= MaxImagesPerPage {
		return nil, fmt.Errorf("%w: maximum images per page exceeded", ErrInvalidInput)
	}

	currentStorage, err := s.repo.GetTotalStorageBytes(ctx, page.EntityType, page.EntityID)
	if err != nil {
		return nil, fmt.Errorf("getting storage stats: %w", err)
	}
	if currentStorage+int64(len(input.Data)) > MaxTotalStorageBytes {
		return nil, ErrStorageLimitExceeded
	}

	image, err := s.repo.CreateImage(ctx, pageID, input)
	if err != nil {
		return nil, err
	}

	return &ImageMeta{
		ID:          image.ID,
		PageID:      image.PageID,
		Filename:    image.Filename,
		ContentType: image.ContentType,
		SizeBytes:   image.SizeBytes,
		URL:         fmt.Sprintf("/api/v1/docs/images/%s", image.ID),
		CreatedAt:   image.CreatedAt,
	}, nil
}

func (s *Service) GetImage(ctx context.Context, imageID string) (*Image, error) {
	return s.repo.GetImage(ctx, imageID)
}

func (s *Service) DeleteImage(ctx context.Context, imageID string) error {
	return s.repo.DeleteImage(ctx, imageID)
}

func (s *Service) ListPageImages(ctx context.Context, pageID string) ([]*ImageMeta, error) {
	return s.repo.ListPageImages(ctx, pageID)
}

func (s *Service) GetStorageStats(ctx context.Context, entityType EntityType, entityID string) (*StorageStats, error) {
	stats, err := s.repo.GetStorageStats(ctx, entityType, entityID)
	if err != nil {
		return nil, err
	}

	stats.MaxBytes = MaxTotalStorageBytes
	if stats.MaxBytes > 0 {
		stats.UsedPercent = float64(stats.UsedBytes) / float64(stats.MaxBytes) * 100
	}

	return stats, nil
}

func (s *Service) GetPageEntityInfo(ctx context.Context, pageID string) (EntityType, string, error) {
	return s.repo.GetPageEntityInfo(ctx, pageID)
}

func (s *Service) GetImageEntityInfo(ctx context.Context, imageID string) (EntityType, string, error) {
	pageID, err := s.repo.GetImagePageID(ctx, imageID)
	if err != nil {
		return "", "", err
	}
	return s.repo.GetPageEntityInfo(ctx, pageID)
}

// ExtractImageIDs extracts image IDs referenced in markdown/HTML content.
func ExtractImageIDs(content string) []string {
	re := regexp.MustCompile(`/api/v1/docs/images/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)
	matches := re.FindAllStringSubmatch(content, -1)

	ids := make([]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			ids = append(ids, match[1])
			seen[match[1]] = true
		}
	}

	return ids
}
