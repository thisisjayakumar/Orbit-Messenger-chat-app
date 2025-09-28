-- Migration: Add username field to users table
-- This script adds the username column and creates necessary indexes

-- Add username column to users table
ALTER TABLE users ADD COLUMN username TEXT;

-- Create unique index for username within organization (case-insensitive)
CREATE UNIQUE INDEX users_org_username_uidx ON users(organization_id, LOWER(username)) WHERE username IS NOT NULL;

-- Create index for username search (case-insensitive)
CREATE INDEX users_username_search_idx ON users(LOWER(username)) WHERE username IS NOT NULL;

-- Update existing users with temporary usernames based on their ID
-- This is a temporary solution - in production, you'd want to prompt users to set their usernames
UPDATE users 
SET username = 'user' || id::text 
WHERE username IS NULL;

-- Make username NOT NULL after setting values
ALTER TABLE users ALTER COLUMN username SET NOT NULL;

-- Add check constraint for username format (similar to Instagram rules)
ALTER TABLE users ADD CONSTRAINT users_username_format_check 
CHECK (
    LENGTH(username) >= 3 AND 
    LENGTH(username) <= 30 AND 
    username ~ '^[a-z0-9._]+$' AND 
    username NOT LIKE '.%' AND 
    username NOT LIKE '%.' AND 
    username NOT LIKE '%..%'
);

-- Add role column if it doesn't exist (for completeness)
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'role') THEN
        -- Create role enum if it doesn't exist
        DO $role_enum$
        BEGIN
            IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
                CREATE TYPE user_role AS ENUM ('admin', 'member');
            END IF;
        END $role_enum$;
        
        -- Add role column
        ALTER TABLE users ADD COLUMN role user_role DEFAULT 'member';
    END IF;
END $$;

-- Update statistics
ANALYZE users;

-- Display migration results
SELECT 
    'Migration completed successfully. Users table now has username column.' as status,
    COUNT(*) as total_users,
    COUNT(username) as users_with_username
FROM users;
