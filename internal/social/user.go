package social

import (
	"context"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
)

type UserProfile struct {
	ID             uuid.UUID `json:"id"`
	Username       string    `json:"username"`
	DisplayName    string    `json:"display_name"`
	AvatarURL      string    `json:"avatar_url"`
	BannerURL      string    `json:"banner_url"`
	Bio            string    `json:"bio"`
	FollowersCount int       `json:"followers_count"`
	FollowingCount int       `json:"following_count"`
	IsVerified     bool      `json:"is_verified"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (s *SocialWrapper) GetProfileByUsername(ctx context.Context, username string) (*UserProfile, error) {
	cacheKey := "user:profile:" + username

	var profile UserProfile
	err := s.redis.Get(ctx, cacheKey).Scan(&profile)
	if err == nil {
		return &profile, nil
	}

	err = s.pool.QueryRow(ctx, `
        SELECT id, username, display_name, avatar_url, banner_url, bio, 
               followers_count, following_count, created_at, updated_at
        FROM users 
        WHERE username = $1 AND deleted_at IS NULL`, username).Scan(
		&profile.ID, &profile.Username, &profile.DisplayName,
		&profile.AvatarURL, &profile.BannerURL, &profile.Bio,
		&profile.FollowersCount, &profile.FollowingCount, &profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	data, err := sonic.Marshal(profile)
	if err != nil {
		return nil, err
	}
	s.redis.Set(ctx, cacheKey, data, 1*time.Hour)

	return &profile, nil
}
func (s *SocialWrapper) IsFollowing(ctx context.Context, followerID, followedID uuid.UUID) (isFollowing bool, err error) {
	if followerID == uuid.Nil {
		return false, nil
	}
	err = s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followed_id = $2)", followerID, followedID).Scan(&isFollowing)
	return isFollowing, err
}
