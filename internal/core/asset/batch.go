package asset

import (
	"context"
	"fmt"
)

// MaxBatchIDs caps a batch read so one request cannot ask for the whole catalog.
const MaxBatchIDs = 200

// GetByIDs reads several assets in one query. Ids that match nothing are skipped, and the assets come back in the order they were asked for.
func (r *PostgresRepository) GetByIDs(ctx context.Context, ids []string) ([]*Asset, error) {
	if len(ids) == 0 {
		return []*Asset{}, nil
	}

	rows, err := r.db.Query(ctx, baseSelectAsset+` WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("querying assets by id: %w", err)
	}
	defer rows.Close()

	byID := make(map[string]*Asset, len(ids))
	for rows.Next() {
		a, err := r.scanAsset(ctx, rows)
		if err != nil {
			return nil, err
		}
		byID[a.ID] = a
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating assets: %w", err)
	}

	out := make([]*Asset, 0, len(byID))
	for _, id := range ids {
		if a, ok := byID[id]; ok {
			out = append(out, a)
			delete(byID, id)
		}
	}
	return out, nil
}

func (s *service) GetByIDs(ctx context.Context, ids []string) ([]*Asset, error) {
	if len(ids) > MaxBatchIDs {
		return nil, fmt.Errorf("%w: at most %d ids", ErrInvalidInput, MaxBatchIDs)
	}
	return s.repo.GetByIDs(ctx, ids)
}
