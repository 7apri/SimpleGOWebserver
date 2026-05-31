package social

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
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
		Author:    p.Author,
		Content:   p.Content,
		CreatedAt: createdAt,
	}, nil
}

type FeedPost struct {
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
}

func (s *SocialWrapper) GetGlobalFeed(ctx context.Context, currentUserID uuid.UUID, cursor uuid.UUID, limit int) ([]FeedPost, error) {
	rows, err := s.pool.Query(ctx, `
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
		  AND ($2::uuid IS NULL OR p.id < $2)
		ORDER BY p.id DESC
		LIMIT $3`,
		currentUserID, cursor, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []FeedPost
	pipe := s.redis.Pipeline()

	type deltaCmds struct {
		likes       *redis.StringCmd
		likesProc   *redis.StringCmd
		reposts     *redis.StringCmd
		repostsProc *redis.StringCmd
	}
	cmds := make(map[uuid.UUID]deltaCmds)

	for rows.Next() {
		var fp FeedPost
		err := rows.Scan(
			&fp.ID, &fp.Content, &fp.MediaURLs, &fp.CreatedAt,
			&fp.LikesCount, &fp.RepostsCount, &fp.RepliesCount,
			&fp.Author.ID, &fp.Author.Username, &fp.Author.DisplayName, &fp.Author.AvatarURL,
			&fp.IsLiked, &fp.IsReposted,
		)
		if err != nil {
			return nil, err
		}
		posts = append(posts, fp)

		idStr := fp.ID.String()
		cmds[fp.ID] = deltaCmds{
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
