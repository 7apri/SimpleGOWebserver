package social

import (
	"context"
	"strconv"
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

func (p *UserProfile) GetUpdatedAtString() string {
	return strconv.FormatInt(p.UpdatedAt.Unix(), 16)
}

func (s *SocialWrapper) GetProfileByUsername(ctx context.Context, username string) (*UserProfile, error) {
	var userID uuid.UUID
	uuidKey := "user:uuid:" + username

	idBytes, err := s.redis.Get(ctx, uuidKey).Bytes()
	if err == nil {
		userID, _ = uuid.ParseBytes(idBytes)
	} else {
		err = s.pool.QueryRow(ctx, "SELECT id FROM users WHERE username = $1 AND deleted_at IS NULL", username).Scan(&userID)
		if err != nil {
			return nil, err
		}
		s.redis.Set(ctx, uuidKey, userID.String(), 24*time.Hour)
	}

	return s.GetProfileByID(ctx, userID)
}
func (s *SocialWrapper) GetProfileByID(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	profileKey := "user:profile:" + userID.String()
	var profile UserProfile

	profileBytes, err := s.redis.Get(ctx, profileKey).Bytes()
	if err != nil {
		err = s.pool.QueryRow(ctx, `
			SELECT id, username, display_name, avatar_url, banner_url, bio, 
				   followers_count, following_count, created_at, updated_at
			FROM users 
			WHERE id = $1 AND deleted_at IS NULL`, userID).Scan(
			&profile.ID, &profile.Username, &profile.DisplayName,
			&profile.AvatarURL, &profile.BannerURL, &profile.Bio,
			&profile.FollowersCount, &profile.FollowingCount, &profile.CreatedAt, &profile.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		data, _ := sonic.Marshal(profile)
		s.redis.Set(ctx, profileKey, data, 1*time.Hour)
	} else {
		_ = sonic.Unmarshal(profileBytes, &profile)
	}

	pipe := s.redis.Pipeline()
	followersCurCmd := pipe.HGet(ctx, KeyFollowers, userID.String())
	followersProcCmd := pipe.HGet(ctx, KeyFollowers+":proc", userID.String())
	followingCurCmd := pipe.HGet(ctx, KeyFollowing, userID.String())
	followingProcCmd := pipe.HGet(ctx, KeyFollowing+":proc", userID.String())
	_, _ = pipe.Exec(ctx)

	if v, err := followersCurCmd.Int(); err == nil {
		profile.FollowersCount += v
	}
	if v, err := followersProcCmd.Int(); err == nil {
		profile.FollowersCount += v
	}
	if v, err := followingCurCmd.Int(); err == nil {
		profile.FollowingCount += v
	}
	if v, err := followingProcCmd.Int(); err == nil {
		profile.FollowingCount += v
	}

	return &profile, nil
}
func (s *SocialWrapper) IsFollowing(ctx context.Context, followerID, followedID uuid.UUID) (isFollowing bool, err error) {
	if followerID == uuid.Nil {
		return false, nil
	}
	err = s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followed_id = $2)", followerID, followedID).Scan(&isFollowing)
	return isFollowing, err
}
func (s *SocialWrapper) GetRealTimeFollowerCount(ctx context.Context, followedID uuid.UUID) (int, error) {
	profile, err := s.GetProfileByID(ctx, followedID)
	if err != nil {
		return 0, err
	}
	return profile.FollowersCount, nil
}
