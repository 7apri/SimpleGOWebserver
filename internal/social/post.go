package social

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type PostAuthor struct {
	ID          uuid.UUID
	Username    string
	DisplayName string
	AvatarURL   string
}

func (p *UserProfile) MapToAuthor() PostAuthor {
	return PostAuthor{
		ID:          p.ID,
		Username:    p.Username,
		DisplayName: p.DisplayName,
		AvatarURL:   p.AvatarURL,
	}
}

type Post struct {
	ID           uuid.UUID
	Content      string
	MediaURLs    []string
	LikesCount   int
	RepostsCount int
	RepliesCount int
	CreatedAt    time.Time
	Author       PostAuthor
	IsLiked      bool
	IsReposted   bool
	IsOwn        bool
}

var ErrPostCreate = errors.New("failed_post_create")

type CreatePostParams struct {
	Author    PostAuthor
	Content   string
	MediaURLs []string
	ParentID  *uuid.UUID
	QuoteID   *uuid.UUID
}

func (s *SocialWrapper) CreatePost(ctx context.Context, p CreatePostParams) (Post, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback(ctx)

	postID, err := uuid.NewV7()
	if err != nil {
		return Post{}, err
	}
	var (
		createdAt time.Time
	)

	err = tx.QueryRow(ctx, `
        INSERT INTO posts (id, user_id, content, media_urls, parent_id, quote_id) 
        VALUES ($1, $2, $3, $4, $5, $6) 
        RETURNING created_at`,
		postID, p.Author.ID, p.Content, p.MediaURLs, p.ParentID, p.QuoteID,
	).Scan(&createdAt)
	if err != nil {
		return Post{}, err
	}

	if p.ParentID != nil {
		_, err = tx.Exec(ctx, `UPDATE posts SET replies_count = replies_count + 1 WHERE id = $1`, *p.ParentID)
		if err != nil {
			return Post{}, err
		}
	}

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
		Content:   p.Content,
		MediaURLs: p.MediaURLs,
		Author:    p.Author,
		CreatedAt: createdAt,
		IsOwn:     true,
	}, nil
}

func (s *SocialWrapper) DeletePost(ctx context.Context, postID, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM posts WHERE id=$1 AND user_id=$2", postID, userID)
	return err
}

func (s *SocialWrapper) GetGlobalFeed(ctx context.Context, currentUserID uuid.UUID, cursor uuid.UUID, limit int) ([]Post, error) {
	var rows pgx.Rows
	var err error

	if cursor == uuid.Nil {
		rows, err = s.pool.Query(ctx, `
		SELECT 
		p.id, p.content, p.media_urls, p.created_at,
		p.likes_count, p.reposts_count, p.replies_count,
		u.id, u.username, u.display_name, u.avatar_url,
		EXISTS(SELECT 1 FROM likes WHERE user_id = $1 AND post_id = p.id) as is_liked,
		EXISTS(SELECT 1 FROM reposts WHERE user_id = $1 AND post_id = p.id) as is_reposted
		FROM posts p
		JOIN users u ON p.user_id = u.id
		WHERE p.parent_id IS NULL 
		AND p.deleted_at IS NULL
		ORDER BY p.id DESC
		LIMIT $2`, currentUserID, limit)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT 
			p.id, p.content, p.media_urls, p.created_at,
			p.likes_count, p.reposts_count, p.replies_count,
			u.id, u.username, u.display_name, u.avatar_url,
			EXISTS(SELECT 1 FROM likes WHERE user_id = $1 AND post_id = p.id) as is_liked,
			EXISTS(SELECT 1 FROM reposts WHERE user_id = $1 AND post_id = p.id) as is_reposted
			FROM posts p
			JOIN users u ON p.user_id = u.id
			WHERE p.parent_id IS NULL 
			AND p.deleted_at IS NULL
			AND p.id < $2
			ORDER BY p.id DESC
			LIMIT $3`, currentUserID, cursor, limit)
	}
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	var posts []Post
	pipe := s.redis.Pipeline()

	type deltaCmds struct {
		likes       *redis.StringCmd
		likesProc   *redis.StringCmd
		reposts     *redis.StringCmd
		repostsProc *redis.StringCmd
	}
	cmds := make(map[uuid.UUID]deltaCmds)

	for rows.Next() {
		var p Post
		err := rows.Scan(
			&p.ID, &p.Content, &p.MediaURLs, &p.CreatedAt,
			&p.LikesCount, &p.RepostsCount, &p.RepliesCount,
			&p.Author.ID, &p.Author.Username, &p.Author.DisplayName, &p.Author.AvatarURL,
			&p.IsLiked, &p.IsReposted,
		)
		if err != nil {
			return nil, err
		}
		if p.Author.ID == currentUserID {
			p.IsOwn = true
		}
		posts = append(posts, p)

		idStr := p.ID.String()
		cmds[p.ID] = deltaCmds{
			likes:       pipe.HGet(ctx, KeyPostLikes, idStr),
			likesProc:   pipe.HGet(ctx, KeyPostLikes+":proc", idStr),
			reposts:     pipe.HGet(ctx, KeyPostReposts, idStr),
			repostsProc: pipe.HGet(ctx, KeyPostReposts+":proc", idStr),
		}
	}

	if len(posts) > 0 {
		_, _ = pipe.Exec(ctx)
	}

	for i := range posts {
		id := posts[i].ID
		if c, exists := cmds[id]; exists {
			if val, err := c.likes.Int(); err == nil {
				posts[i].LikesCount += val
			}
			if val, err := c.likesProc.Int(); err == nil {
				posts[i].LikesCount += val
			}
			if val, err := c.reposts.Int(); err == nil {
				posts[i].RepostsCount += val
			}
			if val, err := c.repostsProc.Int(); err == nil {
				posts[i].RepostsCount += val
			}
		}
	}

	return posts, nil
}

func (s *SocialWrapper) GetPostLikes(ctx context.Context, postID uuid.UUID) (int64, error) {
	var base int64
	err := s.pool.QueryRow(ctx, "SELECT likes_count FROM posts WHERE id = $1", postID).Scan(&base)
	if err != nil {
		return 0, err
	}

	delta, err := s.redis.HGet(ctx, KeyPostLikes, postID.String()).Int64()
	if err != nil && err != redis.Nil {
		return 0, err
	}

	return base + delta, nil
}

func (s *SocialWrapper) GetPostReposts(ctx context.Context, postID uuid.UUID) (int64, error) {
	var base int64
	err := s.pool.QueryRow(ctx, "SELECT reposts_count FROM posts WHERE id = $1", postID).Scan(&base)
	if err != nil {
		return 0, err
	}

	delta, err := s.redis.HGet(ctx, KeyPostReposts, postID.String()).Int64()
	if err != nil && err != redis.Nil {
		return 0, err
	}

	return base + delta, nil
}
