CREATE TYPE pr_status AS ENUM ('OPEN','MERGED');

CREATE TABLE pull_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    author_id UUID NOT NULL REFERENCES users(id),
    status pr_status NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    merged_at TIMESTAMP WITH TIME ZONE NULL
);