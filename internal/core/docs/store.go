package docs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrPageNotFound         = errors.New("page not found")
	ErrImageNotFound        = errors.New("image not found")
	ErrInvalidInput         = errors.New("invalid input")
	ErrStorageLimitExceeded = errors.New("storage limit exceeded")
	ErrImageTooLarge        = errors.New("image exceeds maximum size")
	ErrInvalidImageType     = errors.New("invalid image type")
	ErrMaxPagesExceeded     = errors.New("maximum pages exceeded")
)

const (
	MaxImageSizeBytes    = 5 * 1024 * 1024   // 5MB per image
	MaxTotalStorageBytes = 100 * 1024 * 1024 // 100MB per entity
	MaxImagesPerPage     = 50
	MaxPagesPerEntity    = 100
)

var ValidImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

type EntityType string

const (
	EntityTypeAsset       EntityType = "asset"
	EntityTypeDataProduct EntityType = "data_product"
)

type Page struct {
	ID         string     `json:"id"`
	EntityType EntityType `json:"entity_type"`
	EntityID   string     `json:"entity_id"`
	ParentID   *string    `json:"parent_id,omitempty"`
	Position   int        `json:"position"`
	Title      string     `json:"title"`
	Emoji      *string    `json:"emoji,omitempty"`
	Content    *string    `json:"content,omitempty"`
	CreatedBy  *string    `json:"created_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`

	Children   []*Page `json:"children,omitempty"`
	ImageCount int     `json:"image_count,omitempty"`
}

type Image struct {
	ID          string    `json:"id"`
	PageID      string    `json:"page_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int       `json:"size_bytes"`
	Data        []byte    `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}

type ImageMeta struct {
	ID          string    `json:"id"`
	PageID      string    `json:"page_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int       `json:"size_bytes"`
	URL         string    `json:"url"`
	CreatedAt   time.Time `json:"created_at"`
}

type StorageStats struct {
	EntityType  EntityType `json:"entity_type"`
	EntityID    string     `json:"entity_id"`
	UsedBytes   int64      `json:"used_bytes"`
	MaxBytes    int64      `json:"max_bytes"`
	ImageCount  int        `json:"image_count"`
	PageCount   int        `json:"page_count"`
	UsedPercent float64    `json:"used_percent"`
}

type PageTree struct {
	Pages      []*Page      `json:"pages"`
	TotalPages int          `json:"total_pages"`
	Stats      StorageStats `json:"stats"`
}

type CreatePageInput struct {
	ParentID *string `json:"parent_id,omitempty"`
	Title    string  `json:"title"`
	Emoji    *string `json:"emoji,omitempty"`
	Content  *string `json:"content,omitempty"`
}

type UpdatePageInput struct {
	Title   *string `json:"title,omitempty"`
	Emoji   *string `json:"emoji,omitempty"`
	Content *string `json:"content,omitempty"`

	UpdatedByID   string `json:"-"`
	UpdatedByName string `json:"-"`
}

type MovePageInput struct {
	ParentID *string `json:"parent_id,omitempty"`
	Position int     `json:"position"`
}

type UploadImageInput struct {
	Filename    string
	ContentType string
	Data        []byte
}

type Repository interface {
	CreatePage(ctx context.Context, entityType EntityType, entityID string, input CreatePageInput, createdBy *string) (*Page, error)
	GetPage(ctx context.Context, pageID string) (*Page, error)
	UpdatePage(ctx context.Context, pageID string, input UpdatePageInput) (*Page, error)
	DeletePage(ctx context.Context, pageID string) error
	MovePage(ctx context.Context, pageID string, input MovePageInput) (*Page, error)

	GetPageTree(ctx context.Context, entityType EntityType, entityID string) (*PageTree, error)
	GetRootPages(ctx context.Context, entityType EntityType, entityID string) ([]*Page, error)
	GetChildPages(ctx context.Context, parentID string) ([]*Page, error)
	SearchPages(ctx context.Context, entityType EntityType, entityID string, query string, limit, offset int) ([]*Page, int, error)
	CountPages(ctx context.Context, entityType EntityType, entityID string) (int, error)

	CreateImage(ctx context.Context, pageID string, input UploadImageInput) (*Image, error)
	GetImage(ctx context.Context, imageID string) (*Image, error)
	GetImageMeta(ctx context.Context, imageID string) (*ImageMeta, error)
	DeleteImage(ctx context.Context, imageID string) error
	ListPageImages(ctx context.Context, pageID string) ([]*ImageMeta, error)
	CountPageImages(ctx context.Context, pageID string) (int, error)

	GetStorageStats(ctx context.Context, entityType EntityType, entityID string) (*StorageStats, error)
	GetTotalStorageBytes(ctx context.Context, entityType EntityType, entityID string) (int64, error)

	GetPageEntityInfo(ctx context.Context, pageID string) (EntityType, string, error)
	GetImagePageID(ctx context.Context, imageID string) (string, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreatePage(ctx context.Context, entityType EntityType, entityID string, input CreatePageInput, createdBy *string) (*Page, error) {
	var position int
	posQuery := `
		SELECT COALESCE(MAX(position), -1) + 1
		FROM doc_pages
		WHERE entity_type = $1 AND entity_id = $2 AND parent_id IS NOT DISTINCT FROM $3`

	err := r.db.QueryRow(ctx, posQuery, entityType, entityID, input.ParentID).Scan(&position)
	if err != nil {
		return nil, fmt.Errorf("getting next position: %w", err)
	}

	query := `
		INSERT INTO doc_pages (entity_type, entity_id, parent_id, position, title, emoji, content, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, entity_type, entity_id, parent_id, position, title, emoji, content, created_by, created_at, updated_at`

	var page Page
	err = r.db.QueryRow(ctx, query,
		entityType, entityID, input.ParentID, position, input.Title, input.Emoji, input.Content, createdBy,
	).Scan(
		&page.ID, &page.EntityType, &page.EntityID, &page.ParentID,
		&page.Position, &page.Title, &page.Emoji, &page.Content, &page.CreatedBy,
		&page.CreatedAt, &page.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating page: %w", err)
	}

	return &page, nil
}

func (r *PostgresRepository) GetPage(ctx context.Context, pageID string) (*Page, error) {
	query := `
		SELECT id, entity_type, entity_id, parent_id, position, title, emoji, content, created_by, created_at, updated_at
		FROM doc_pages
		WHERE id = $1`

	var page Page
	err := r.db.QueryRow(ctx, query, pageID).Scan(
		&page.ID, &page.EntityType, &page.EntityID, &page.ParentID,
		&page.Position, &page.Title, &page.Emoji, &page.Content, &page.CreatedBy,
		&page.CreatedAt, &page.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting page: %w", err)
	}

	var imageCount int
	err = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM doc_images WHERE page_id = $1", pageID).Scan(&imageCount)
	if err != nil {
		return nil, fmt.Errorf("counting images: %w", err)
	}
	page.ImageCount = imageCount

	return &page, nil
}

func (r *PostgresRepository) UpdatePage(ctx context.Context, pageID string, input UpdatePageInput) (*Page, error) {
	query := `
		UPDATE doc_pages
		SET title = COALESCE($2, title),
		    emoji = COALESCE($3, emoji),
		    content = COALESCE($4, content),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, entity_type, entity_id, parent_id, position, title, emoji, content, created_by, created_at, updated_at`

	var page Page
	err := r.db.QueryRow(ctx, query, pageID, input.Title, input.Emoji, input.Content).Scan(
		&page.ID, &page.EntityType, &page.EntityID, &page.ParentID,
		&page.Position, &page.Title, &page.Emoji, &page.Content, &page.CreatedBy,
		&page.CreatedAt, &page.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("updating page: %w", err)
	}

	return &page, nil
}

func (r *PostgresRepository) DeletePage(ctx context.Context, pageID string) error {
	result, err := r.db.Exec(ctx, "DELETE FROM doc_pages WHERE id = $1", pageID)
	if err != nil {
		return fmt.Errorf("deleting page: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrPageNotFound
	}
	return nil
}

func (r *PostgresRepository) MovePage(ctx context.Context, pageID string, input MovePageInput) (*Page, error) {
	page, err := r.GetPage(ctx, pageID)
	if err != nil {
		return nil, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if input.ParentID != page.ParentID || input.Position != page.Position {
		shiftQuery := `
			UPDATE doc_pages
			SET position = position + 1
			WHERE entity_type = $1 AND entity_id = $2
				AND parent_id IS NOT DISTINCT FROM $3
				AND position >= $4
				AND id != $5`

		_, err = tx.Exec(ctx, shiftQuery, page.EntityType, page.EntityID, input.ParentID, input.Position, pageID)
		if err != nil {
			return nil, fmt.Errorf("shifting positions: %w", err)
		}
	}

	updateQuery := `
		UPDATE doc_pages
		SET parent_id = $2, position = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING id, entity_type, entity_id, parent_id, position, title, emoji, content, created_by, created_at, updated_at`

	err = tx.QueryRow(ctx, updateQuery, pageID, input.ParentID, input.Position).Scan(
		&page.ID, &page.EntityType, &page.EntityID, &page.ParentID,
		&page.Position, &page.Title, &page.Emoji, &page.Content, &page.CreatedBy,
		&page.CreatedAt, &page.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("updating page position: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return page, nil
}

func (r *PostgresRepository) GetPageTree(ctx context.Context, entityType EntityType, entityID string) (*PageTree, error) {
	query := `
		SELECT p.id, p.entity_type, p.entity_id, p.parent_id, p.position, p.title, p.emoji, p.content,
		       p.created_by, p.created_at, p.updated_at, COALESCE(ic.image_count, 0) as image_count
		FROM doc_pages p
		LEFT JOIN (
			SELECT page_id, COUNT(*) as image_count FROM doc_images GROUP BY page_id
		) ic ON ic.page_id = p.id
		WHERE p.entity_type = $1 AND p.entity_id = $2
		ORDER BY p.position`

	rows, err := r.db.Query(ctx, query, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("querying pages: %w", err)
	}
	defer rows.Close()

	pageMap := make(map[string]*Page)
	var allPages []*Page

	for rows.Next() {
		var page Page
		err := rows.Scan(
			&page.ID, &page.EntityType, &page.EntityID, &page.ParentID,
			&page.Position, &page.Title, &page.Emoji, &page.Content, &page.CreatedBy,
			&page.CreatedAt, &page.UpdatedAt, &page.ImageCount,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning page: %w", err)
		}
		pageMap[page.ID] = &page
		allPages = append(allPages, &page)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pages: %w", err)
	}

	rootPages := make([]*Page, 0)
	for _, page := range allPages {
		if page.ParentID == nil {
			rootPages = append(rootPages, page)
		} else if parent, exists := pageMap[*page.ParentID]; exists {
			parent.Children = append(parent.Children, page)
		}
	}

	stats, err := r.GetStorageStats(ctx, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("getting storage stats: %w", err)
	}

	return &PageTree{
		Pages:      rootPages,
		TotalPages: len(allPages),
		Stats:      *stats,
	}, nil
}

func (r *PostgresRepository) GetRootPages(ctx context.Context, entityType EntityType, entityID string) ([]*Page, error) {
	query := `
		SELECT id, entity_type, entity_id, parent_id, position, title, emoji, content, created_by, created_at, updated_at
		FROM doc_pages
		WHERE entity_type = $1 AND entity_id = $2 AND parent_id IS NULL
		ORDER BY position`

	rows, err := r.db.Query(ctx, query, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("querying root pages: %w", err)
	}
	defer rows.Close()

	var pages []*Page
	for rows.Next() {
		var page Page
		err := rows.Scan(
			&page.ID, &page.EntityType, &page.EntityID, &page.ParentID,
			&page.Position, &page.Title, &page.Emoji, &page.Content, &page.CreatedBy,
			&page.CreatedAt, &page.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning page: %w", err)
		}
		pages = append(pages, &page)
	}

	return pages, rows.Err()
}

func (r *PostgresRepository) GetChildPages(ctx context.Context, parentID string) ([]*Page, error) {
	query := `
		SELECT id, entity_type, entity_id, parent_id, position, title, emoji, content, created_by, created_at, updated_at
		FROM doc_pages
		WHERE parent_id = $1
		ORDER BY position`

	rows, err := r.db.Query(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("querying child pages: %w", err)
	}
	defer rows.Close()

	var pages []*Page
	for rows.Next() {
		var page Page
		err := rows.Scan(
			&page.ID, &page.EntityType, &page.EntityID, &page.ParentID,
			&page.Position, &page.Title, &page.Emoji, &page.Content, &page.CreatedBy,
			&page.CreatedAt, &page.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning page: %w", err)
		}
		pages = append(pages, &page)
	}

	return pages, rows.Err()
}

func (r *PostgresRepository) SearchPages(ctx context.Context, entityType EntityType, entityID string, queryStr string, limit, offset int) ([]*Page, int, error) {
	countQuery := `
		SELECT COUNT(*)
		FROM doc_pages
		WHERE entity_type = $1 AND entity_id = $2 AND search_text @@ plainto_tsquery('english', $3)`

	var total int
	err := r.db.QueryRow(ctx, countQuery, entityType, entityID, queryStr).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting search results: %w", err)
	}

	searchQuery := `
		SELECT id, entity_type, entity_id, parent_id, position, title, emoji, content, created_by, created_at, updated_at,
		       ts_rank(search_text, plainto_tsquery('english', $3)) as rank
		FROM doc_pages
		WHERE entity_type = $1 AND entity_id = $2 AND search_text @@ plainto_tsquery('english', $3)
		ORDER BY rank DESC, updated_at DESC
		LIMIT $4 OFFSET $5`

	rows, err := r.db.Query(ctx, searchQuery, entityType, entityID, queryStr, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("searching pages: %w", err)
	}
	defer rows.Close()

	var pages []*Page
	for rows.Next() {
		var page Page
		var rank float64
		err := rows.Scan(
			&page.ID, &page.EntityType, &page.EntityID, &page.ParentID,
			&page.Position, &page.Title, &page.Emoji, &page.Content, &page.CreatedBy,
			&page.CreatedAt, &page.UpdatedAt, &rank,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning page: %w", err)
		}
		pages = append(pages, &page)
	}

	return pages, total, rows.Err()
}

func (r *PostgresRepository) CountPages(ctx context.Context, entityType EntityType, entityID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM doc_pages WHERE entity_type = $1 AND entity_id = $2",
		entityType, entityID,
	).Scan(&count)
	return count, err
}

func (r *PostgresRepository) CreateImage(ctx context.Context, pageID string, input UploadImageInput) (*Image, error) {
	query := `
		INSERT INTO doc_images (page_id, filename, content_type, size_bytes, data)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, page_id, filename, content_type, size_bytes, created_at`

	var image Image
	err := r.db.QueryRow(ctx, query, pageID, input.Filename, input.ContentType, len(input.Data), input.Data).Scan(
		&image.ID, &image.PageID, &image.Filename, &image.ContentType, &image.SizeBytes, &image.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating image: %w", err)
	}

	image.Data = input.Data
	return &image, nil
}

func (r *PostgresRepository) GetImage(ctx context.Context, imageID string) (*Image, error) {
	query := `
		SELECT id, page_id, filename, content_type, size_bytes, data, created_at
		FROM doc_images
		WHERE id = $1`

	var image Image
	err := r.db.QueryRow(ctx, query, imageID).Scan(
		&image.ID, &image.PageID, &image.Filename, &image.ContentType,
		&image.SizeBytes, &image.Data, &image.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrImageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting image: %w", err)
	}

	return &image, nil
}

func (r *PostgresRepository) GetImageMeta(ctx context.Context, imageID string) (*ImageMeta, error) {
	query := `
		SELECT id, page_id, filename, content_type, size_bytes, created_at
		FROM doc_images
		WHERE id = $1`

	var meta ImageMeta
	err := r.db.QueryRow(ctx, query, imageID).Scan(
		&meta.ID, &meta.PageID, &meta.Filename, &meta.ContentType, &meta.SizeBytes, &meta.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrImageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting image metadata: %w", err)
	}

	meta.URL = fmt.Sprintf("/api/v1/docs/images/%s", meta.ID)
	return &meta, nil
}

func (r *PostgresRepository) DeleteImage(ctx context.Context, imageID string) error {
	result, err := r.db.Exec(ctx, "DELETE FROM doc_images WHERE id = $1", imageID)
	if err != nil {
		return fmt.Errorf("deleting image: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrImageNotFound
	}
	return nil
}

func (r *PostgresRepository) ListPageImages(ctx context.Context, pageID string) ([]*ImageMeta, error) {
	query := `
		SELECT id, page_id, filename, content_type, size_bytes, created_at
		FROM doc_images
		WHERE page_id = $1
		ORDER BY created_at`

	rows, err := r.db.Query(ctx, query, pageID)
	if err != nil {
		return nil, fmt.Errorf("listing page images: %w", err)
	}
	defer rows.Close()

	var images []*ImageMeta
	for rows.Next() {
		var meta ImageMeta
		err := rows.Scan(&meta.ID, &meta.PageID, &meta.Filename, &meta.ContentType, &meta.SizeBytes, &meta.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning image: %w", err)
		}
		meta.URL = fmt.Sprintf("/api/v1/docs/images/%s", meta.ID)
		images = append(images, &meta)
	}

	return images, rows.Err()
}

func (r *PostgresRepository) CountPageImages(ctx context.Context, pageID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM doc_images WHERE page_id = $1", pageID).Scan(&count)
	return count, err
}

func (r *PostgresRepository) GetStorageStats(ctx context.Context, entityType EntityType, entityID string) (*StorageStats, error) {
	query := `
		SELECT COALESCE(SUM(di.size_bytes), 0), COUNT(DISTINCT di.id), COUNT(DISTINCT dp.id)
		FROM doc_pages dp
		LEFT JOIN doc_images di ON di.page_id = dp.id
		WHERE dp.entity_type = $1 AND dp.entity_id = $2`

	stats := StorageStats{
		EntityType: entityType,
		EntityID:   entityID,
		MaxBytes:   MaxTotalStorageBytes,
	}

	err := r.db.QueryRow(ctx, query, entityType, entityID).Scan(
		&stats.UsedBytes, &stats.ImageCount, &stats.PageCount,
	)
	if err != nil {
		return nil, fmt.Errorf("getting storage stats: %w", err)
	}

	if stats.MaxBytes > 0 {
		stats.UsedPercent = float64(stats.UsedBytes) / float64(stats.MaxBytes) * 100
	}

	return &stats, nil
}

func (r *PostgresRepository) GetTotalStorageBytes(ctx context.Context, entityType EntityType, entityID string) (int64, error) {
	query := `
		SELECT COALESCE(SUM(di.size_bytes), 0)
		FROM doc_images di
		INNER JOIN doc_pages dp ON di.page_id = dp.id
		WHERE dp.entity_type = $1 AND dp.entity_id = $2`

	var total int64
	err := r.db.QueryRow(ctx, query, entityType, entityID).Scan(&total)
	return total, err
}

func (r *PostgresRepository) GetPageEntityInfo(ctx context.Context, pageID string) (EntityType, string, error) {
	var entityType EntityType
	var entityID string

	err := r.db.QueryRow(ctx,
		"SELECT entity_type, entity_id FROM doc_pages WHERE id = $1",
		pageID,
	).Scan(&entityType, &entityID)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrPageNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("getting page entity info: %w", err)
	}

	return entityType, entityID, nil
}

func (r *PostgresRepository) GetImagePageID(ctx context.Context, imageID string) (string, error) {
	var pageID string
	err := r.db.QueryRow(ctx, "SELECT page_id FROM doc_images WHERE id = $1", imageID).Scan(&pageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrImageNotFound
	}
	return pageID, err
}
