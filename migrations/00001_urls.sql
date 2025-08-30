-- +goose Up
-- +goose StatementBegin
CREATE TABLE urls (
	id SERIAL PRIMARY KEY,
	original_url TEXT NOT NULL UNIQUE,
	alias TEXT NOT NULL UNIQUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE urls;
-- +goose StatementEnd
