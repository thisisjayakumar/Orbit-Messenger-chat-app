-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "citext";

-- Organizations
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    settings JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email CITEXT NOT NULL,
    display_name TEXT NOT NULL,
    avatar_url TEXT,
    profile JSONB DEFAULT '{}'::jsonb,
    password_hash TEXT,
    keycloak_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX users_org_email_uidx ON users(organization_id, email);

-- Conversation type
CREATE TYPE conversation_type AS ENUM ('DM','GROUP');

CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    type conversation_type NOT NULL,
    title TEXT,
    created_by UUID NOT NULL REFERENCES users(id),
    is_encrypted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX conv_org_type_idx ON conversations(organization_id, type);

-- Participants
CREATE TYPE participant_role AS ENUM ('admin','member');

CREATE TABLE conversation_participants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role participant_role NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_read_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX conv_part_unique ON conversation_participants(conversation_id, user_id);
CREATE INDEX conv_part_user_idx ON conversation_participants(user_id, conversation_id);

-- Messages
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    content_type TEXT NOT NULL,
    content TEXT NOT NULL,
    meta JSONB DEFAULT '{}'::jsonb,
    dedupe_key TEXT,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    edited_at TIMESTAMPTZ,
    deleted BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX msg_conv_time_idx ON messages(conversation_id, sent_at DESC);
CREATE UNIQUE INDEX msg_dedupe_uidx ON messages(conversation_id, dedupe_key) 
WHERE dedupe_key IS NOT NULL;

-- Receipts
CREATE TYPE receipt_status AS ENUM ('delivered','read');

CREATE TABLE message_receipts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status receipt_status NOT NULL,
    at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX msg_receipt_unique ON message_receipts(message_id, user_id, status);
CREATE INDEX msg_receipt_user_idx ON message_receipts(user_id, status, at DESC);

-- Attachments
CREATE TABLE attachments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id UUID REFERENCES messages(id) ON DELETE CASCADE,
    object_key TEXT NOT NULL,
    file_name TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    size BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'uploading',
    meta JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX attachments_message_id_idx ON attachments(message_id);
CREATE INDEX attachments_status_idx ON attachments(status);

-- Device sessions
CREATE TABLE device_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id TEXT NOT NULL,
    device_info TEXT,
    ip INET,
    connected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    disconnected_at TIMESTAMPTZ
);

CREATE INDEX device_sessions_user_time_idx ON device_sessions(user_id, connected_at DESC);

-- Audit events
CREATE TABLE audit_events (
    id BIGSERIAL PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id),
    action TEXT NOT NULL,
    target_type TEXT,
    target_id TEXT,
    details JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);