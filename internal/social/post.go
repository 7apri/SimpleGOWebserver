package social

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type PostAuthor struct {
	ID          uuid.UUID
	Username    string
	DisplayName string
	AvatarURL   string
}

type Post struct {
	ID        uuid.UUID
	Author    PostAuthor
	Content   string
	CreatedAt time.Time
}

var ErrPostCreate = errors.New("failed_post_create")

type CreatePostParams struct {
	Author    PostAuthor
	Content   string
	MediaURLs []string
	ParentID  *uuid.UUID
	QuoteID   *uuid.UUID
}

func (s *SocialWrapper) CreateUnifiedPost(ctx context.Context, p CreatePostParams) (Post, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback(ctx)

	var postID uuid.UUID
	var createdAt time.Time

	// 1. Insert the actual post (works for standard, reply, and quote)
	err = tx.QueryRow(ctx, `
        INSERT INTO posts (user_id, content, media_urls, parent_id, quote_id) 
        VALUES ($1, $2, $3, $4, $5) 
        RETURNING id, created_at`,
		p.Author.ID, p.Content, p.MediaURLs, p.ParentID, p.QuoteID,
	).Scan(&postID, &createdAt)
	if err != nil {
		return Post{}, err
	}

	// 2. Increment the Author's total post count
	_, err = tx.Exec(ctx, `UPDATE users SET posts_count = posts_count + 1 WHERE id = $1`, p.Author.ID)
	if err != nil {
		return Post{}, err
	}

	// 3. If it's a Reply, increment the parent's reply count
	if p.ParentID != nil {
		_, err = tx.Exec(ctx, `UPDATE posts SET replies_count = replies_count + 1 WHERE id = $1`, *p.ParentID)
		if err != nil {
			return Post{}, err
		}
	}

	// 4. If it's a Quote, increment the quoted post's quote count
	if p.QuoteID != nil {
		_, err = tx.Exec(ctx, `UPDATE posts SET quotes_count = quotes_count + 1 WHERE id = $1`, *p.QuoteID)
		if err != nil {
			return Post{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Post{}, err
	}

	return Post{
		ID:        postID,
		Author:    p.Author,
		Content:   p.Content,
		CreatedAt: createdAt,
		// MediaURLs could be added to your Post struct if you want to render them immediately
	}, nil
}

func (s *SocialWrapper) ToggleLike(ctx context.Context, userID, postID uuid.UUID) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var liked bool
	tag, err := tx.Exec(ctx, `
		DELETE FROM likes 
		WHERE user_id = $1 AND post_id = $2`,
		userID, postID)
	if err != nil {
		return false, err
	}

	pipe := s.redis.Pipeline()

	if tag.RowsAffected() > 0 {
		liked = false
		pipe.HIncrBy(ctx, KeyPostLikes, postID.String(), -1)
	} else {
		_, err = tx.Exec(ctx, `
			INSERT INTO likes (user_id, post_id) 
			VALUES ($1, $2)`,
			userID, postID)
		if err != nil {
			return false, err
		}
		liked = true
		pipe.HIncrBy(ctx, KeyPostLikes, postID.String(), 1)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}

	_, err = pipe.Exec(ctx)
	return liked, err
}
