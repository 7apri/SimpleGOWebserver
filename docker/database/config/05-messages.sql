CREATE TYPE room_type AS ENUM ('dm', 'group');

CREATE TABLE rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type room_type NOT NULL DEFAULT 'dm',
    name TEXT DEFAULT NULL,
    created_by UUID REFERENCES users(id) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE room_participants (
    room_id UUID REFERENCES rooms(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_id, user_id)
);

CREATE TABLE room_keys (
    room_id UUID REFERENCES rooms(id) ON DELETE CASCADE,
    device_id UUID REFERENCES user_devices(device_id) ON DELETE CASCADE,
    key_version INT NOT NULL DEFAULT 1,
    encrypted_room_key TEXT NOT NULL,
    PRIMARY KEY (room_id, device_id, key_version)
);

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id),
    content_encrypted TEXT NOT NULL,  
    nonce TEXT NOT NULL,              
    key_version INT NOT NULL DEFAULT 1, 
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_devices_user_id ON user_devices(user_id);
CREATE INDEX idx_room_participants_user_id ON room_participants(user_id);
CREATE INDEX idx_messages_room_timeline ON messages(room_id, created_at DESC);