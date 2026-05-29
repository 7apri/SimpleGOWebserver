package social

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	KeyFollowers   = "social:follower_counts"
	KeyFollowing   = "social:following_counts"
	KeyPostLikes   = "social:post_likes"
	KeyPostReposts = "social:repost_counts"
	KeyPostReplies = "social:reply_counts"
	KeyPostQuotes  = "social:quote_counts"

	batchSize = 500
)

var syncQueries = map[string]string{
	KeyFollowers: "UPDATE users SET followers_count = followers_count + $1 WHERE id = $2",
	KeyFollowing: "UPDATE users SET following_count = following_count + $1 WHERE id = $2",
	KeyPostLikes: "UPDATE posts SET like_count = like_count + $1 WHERE id = $2",

	KeyPostReposts: "UPDATE posts SET reposts_count = reposts_count + $1 WHERE id = $2",
	KeyPostReplies: "UPDATE posts SET replies_count = replies_count + $1 WHERE id = $2",
	KeyPostQuotes:  "UPDATE posts SET quotes_count = quotes_count + $1 WHERE id = $2",
}

func (s *SocialWrapper) runFlush() {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	for mainKey, query := range syncQueries {
		s.processMetric(ctx, mainKey, query)
	}
}

func (s *SocialWrapper) processMetric(ctx context.Context, mainKey, query string) {
	procKey := mainKey + ":proc"

	exists, _ := s.redis.Exists(ctx, procKey).Result()
	if exists == 0 {
		if err := s.redis.Rename(ctx, mainKey, procKey).Err(); err != nil {
			return
		}
	}

	counts, err := s.redis.HGetAll(ctx, procKey).Result()
	if err != nil || len(counts) == 0 {
		return
	}

	b := &pgx.Batch{}
	count := 0

	send := func() {
		if b.Len() == 0 {
			return
		}
		res := s.pool.SendBatch(ctx, b)
		if err := res.Close(); err != nil {
			slog.Error("[Social] Batch flush failed", "key", mainKey, "error", err)
		}
		b = &pgx.Batch{}
	}

	for idStr, deltaStr := range counts {
		delta, err := strconv.Atoi(deltaStr)
		if err != nil {
			continue
		}

		b.Queue(query, delta, idStr)
		count++

		if count >= batchSize {
			send()
			count = 0
		}
	}
	send()

	if mainKey == KeyFollowers || mainKey == KeyFollowing {
		pipe := s.redis.Pipeline()
		for idStr := range counts {
			pipe.Del(ctx, "user:profile:"+idStr)
		}
		_, _ = pipe.Exec(ctx)
	}

	s.redis.Del(ctx, procKey)
}

func (s *SocialWrapper) ToggleFollow(ctx context.Context, followerID, followedID uuid.UUID) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var followed bool
	tag, err := tx.Exec(ctx, `
		DELETE FROM follows 
		WHERE follower_id = $1 AND followed_id = $2`,
		followerID, followedID)
	if err != nil {
		return false, err
	}

	pipe := s.redis.Pipeline()

	if tag.RowsAffected() > 0 {
		followed = false
		pipe.HIncrBy(ctx, KeyFollowers, followedID.String(), -1)
		pipe.HIncrBy(ctx, KeyFollowing, followerID.String(), -1)
	} else {
		_, err = tx.Exec(ctx, `
			INSERT INTO follows (follower_id, followed_id) 
			VALUES ($1, $2)`,
			followerID, followedID)
		if err != nil {
			return false, err
		}
		followed = true
		pipe.HIncrBy(ctx, KeyFollowers, followedID.String(), 1)
		pipe.HIncrBy(ctx, KeyFollowing, followerID.String(), 1)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}

	_, err = pipe.Exec(ctx)
	return followed, err
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
