package social

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ChatListItem struct {
	ID          uuid.UUID
	Name        string
	LastMessage string
	UnreadCount int
	UpdatedAt   time.Time
}

func (s *SocialWrapper) GetChats(ctx context.Context, userID uuid.UUID) ([]ChatListItem, error) {
	const q = `
        SELECT 
            r.id, 
            r.name, 
            COALESCE(m.content_encrypted, '') as last_message,
            (SELECT COUNT(*) FROM messages msg
             WHERE msg.room_id = r.id 
             AND msg.created_at > rp.last_read_at) as unread_count,
            COALESCE(m.created_at, r.created_at) as updated_at
        FROM rooms r
        JOIN room_participants rp ON r.id = rp.room_id
        LEFT JOIN LATERAL (
            SELECT content_encrypted, created_at 
            FROM messages 
            WHERE room_id = r.id 
            ORDER BY created_at DESC LIMIT 1
        ) m ON true
        WHERE rp.user_id = $1
        ORDER BY updated_at DESC`

	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, ErrDatabase
	}
	defer rows.Close()

	var rooms []ChatListItem
	for rows.Next() {
		var room ChatListItem
		err := rows.Scan(&room.ID, &room.Name, &room.LastMessage, &room.UnreadCount, &room.UpdatedAt)
		if err == nil {
			rooms = append(rooms, room)
		}
	}

	return rooms, nil
}
