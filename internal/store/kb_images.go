package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// KBInsertImage stores an image and returns its id. articleID may
// be nil for "uploaded during a new-article draft before the article
// row exists" — those get adopted on the article's first save (the
// handler walks the body_html for /api/kb/images/{id} references and
// links them).
func (s *Store) KBInsertImage(ctx context.Context, img KBImage) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO kb_images (article_id, filename, mime, bytes, content, sha256)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		img.ArticleID, img.Filename, img.MIME, img.Bytes, img.Content, img.SHA256).
		Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert kb_image: %w", err)
	}
	return id, nil
}

// KBGetImage returns the bytes + mime for the serve handler.
func (s *Store) KBGetImage(ctx context.Context, id int64) (KBImage, error) {
	var img KBImage
	err := s.pool.QueryRow(ctx, `
		SELECT id, article_id, filename, mime, bytes, content, sha256, created_at
		FROM kb_images WHERE id = $1`, id).
		Scan(&img.ID, &img.ArticleID, &img.Filename, &img.MIME, &img.Bytes,
			&img.Content, &img.SHA256, &img.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBImage{}, ErrNotFound
	}
	if err != nil {
		return KBImage{}, fmt.Errorf("get kb_image: %w", err)
	}
	return img, nil
}

// KBAdoptOrphanImages walks the body_html for `/api/kb/images/{id}`
// references and updates each matching kb_images row's article_id
// to point at the saved article. No-op for images already linked to
// this article. Called from the article-save handler after the row
// exists.
func (s *Store) KBAdoptOrphanImages(ctx context.Context, articleID int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE kb_images SET article_id = $1
		WHERE id = ANY($2) AND (article_id IS NULL OR article_id = $1)`,
		articleID, ids)
	if err != nil {
		return fmt.Errorf("adopt kb_images: %w", err)
	}
	return nil
}
