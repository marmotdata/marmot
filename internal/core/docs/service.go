package docs

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// MentionNotifier sends notifications when users or teams are mentioned in docs.
type MentionNotifier interface {
	OnMention(ctx context.Context, mention Mention, pageID, pageTitle, entityType, entityID, mentionerID, mentionerName string)
}

type Service struct {
	repo            Repository
	mentionNotifier MentionNotifier
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// SetMentionNotifier sets the notifier for user mentions.
func (s *Service) SetMentionNotifier(notifier MentionNotifier) {
	s.mentionNotifier = notifier
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

	// Get old page content to detect NEW mentions only
	// Key format: "label:type" to track both user and team mentions separately
	var oldMentionKeys map[string]bool
	if s.mentionNotifier != nil && input.Content != nil && input.UpdatedByID != "" {
		if oldPage, err := s.repo.GetPage(ctx, pageID); err == nil && oldPage.Content != nil && *oldPage.Content != "" {
			oldMentionsList := ExtractMentions(*oldPage.Content)
			oldMentionKeys = make(map[string]bool, len(oldMentionsList))
			for _, m := range oldMentionsList {
				oldMentionKeys[m.Label+":"+m.Type] = true
			}
		} else {
			oldMentionKeys = make(map[string]bool)
		}
	}

	page, err := s.repo.UpdatePage(ctx, pageID, input)
	if err != nil {
		return nil, err
	}

	// Only notify for NEW mentions (not existing ones)
	if s.mentionNotifier != nil && input.Content != nil && input.UpdatedByID != "" {
		newMentions := ExtractMentions(*input.Content)
		log.Debug().
			Int("old_mentions_count", len(oldMentionKeys)).
			Int("new_mentions_count", len(newMentions)).
			Str("page_id", pageID).
			Msg("Processing mentions for page update")

		for _, mention := range newMentions {
			// Skip if this mention already existed in the old content
			key := mention.Label + ":" + mention.Type
			if oldMentionKeys[key] {
				log.Debug().Str("mention", mention.Label).Str("type", mention.Type).Msg("Skipping existing mention")
				continue
			}
			log.Debug().Str("mention", mention.Label).Str("type", mention.Type).Msg("Notifying for new mention")
			s.mentionNotifier.OnMention(
				ctx,
				mention,
				page.ID,
				page.Title,
				string(page.EntityType),
				page.EntityID,
				input.UpdatedByID,
				input.UpdatedByName,
			)
		}
	}

	return page, nil
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

// Mention represents a parsed @mention from content.
type Mention struct {
	Label string // Display name
	ID    string // UUID if available
	Type  string // "user" or "team"
}

// mentionRegex matches markdown link format: [@Label](mention:type:id)
var mentionRegex = regexp.MustCompile(`\[@([^\]]+)\]\(mention:(user|team):([^)]+)\)`)

// ExtractMentions extracts unique mentions from content.
func ExtractMentions(content string) []Mention {
	matches := mentionRegex.FindAllStringSubmatch(content, -1)

	mentions := make([]Mention, 0, len(matches))
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		label := match[1]       // Label from [@Label]
		mentionType := match[2] // "user" or "team" from (mention:type:id)
		id := match[3]          // ID from (mention:type:id)

		// Use label+type as key to allow same name as user and team
		key := label + ":" + mentionType
		if label != "" && !seen[key] {
			mentions = append(mentions, Mention{
				Label: label,
				ID:    id,
				Type:  mentionType,
			})
			seen[key] = true
		}
	}

	return mentions
}
