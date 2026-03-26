-- Create examples table
CREATE TABLE IF NOT EXISTS examples (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create index for status filtering
CREATE INDEX IF NOT EXISTS idx_examples_status ON examples(status);

-- Create index for soft delete queries
CREATE INDEX IF NOT EXISTS idx_examples_deleted_at ON examples(deleted_at);

-- Add comment to table
COMMENT ON TABLE examples IS 'Example table for reference implementation';
