-- Reset Database and Create Super Admin User
-- WARNING: This will delete ALL data in the database!

-- Disable foreign key checks temporarily
SET session_replication_role = replica;

-- Truncate all tables in the correct order (respecting foreign key constraints)
TRUNCATE TABLE audit_events RESTART IDENTITY CASCADE;
TRUNCATE TABLE device_sessions RESTART IDENTITY CASCADE;
TRUNCATE TABLE attachments RESTART IDENTITY CASCADE;
TRUNCATE TABLE media_files RESTART IDENTITY CASCADE;
TRUNCATE TABLE message_receipts RESTART IDENTITY CASCADE;
TRUNCATE TABLE messages RESTART IDENTITY CASCADE;
TRUNCATE TABLE conversation_participants RESTART IDENTITY CASCADE;
TRUNCATE TABLE conversations RESTART IDENTITY CASCADE;
TRUNCATE TABLE user_presence RESTART IDENTITY CASCADE;
TRUNCATE TABLE users RESTART IDENTITY CASCADE;
TRUNCATE TABLE organizations RESTART IDENTITY CASCADE;

-- Re-enable foreign key checks
SET session_replication_role = DEFAULT;

-- Create default organization
INSERT INTO organizations (id, name, settings) VALUES 
    ('00000000-0000-0000-0000-000000000000', 'Default Organization', '{"max_users": 1000, "features": ["chat", "media", "presence"]}');

-- Create super admin user
-- Password: 'password' (using the same hash from init-with-int-user-ids.sql)
-- This hash is for the password 'password': $2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi
INSERT INTO users (organization_id, email, display_name, role, password_hash, created_at) VALUES 
    ('00000000-0000-0000-0000-000000000000', 
     'admin@orbit.com', 
     'Super Admin', 
     'admin', 
     '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 
     NOW());

-- Initialize user presence for the admin
INSERT INTO user_presence (user_id, status, last_seen, updated_at) 
SELECT id, 'offline', NOW(), NOW() FROM users WHERE email = 'admin@orbit.com';

-- Display the created admin user details
SELECT 
    u.id,
    u.email,
    u.display_name,
    u.role,
    u.created_at,
    o.name as organization_name,
    'Password: password' as login_info
FROM users u
JOIN organizations o ON u.organization_id = o.id
WHERE u.email = 'admin@orbit.com';

COMMIT;
