CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id UUID NOT NULL REFERENCES users(id),
    room_id UUID DEFAULT NULL, 
    receiver_id UUID DEFAULT NULL REFERENCES users(id), 
    content_encrypted TEXT NOT NULL,
    nonce TEXT NOT NULL,           
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- For Group Chats
CREATE TABLE rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE room_keys (
    room_id UUID REFERENCES rooms(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    encrypted_room_key TEXT NOT NULL,
    PRIMARY KEY (room_id, user_id)
);