CREATE TABLE pr_reviewers (
    id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pull_request_id UUID NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
    assigned_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    PRIMARY KEY(pull_request_id, id)
);