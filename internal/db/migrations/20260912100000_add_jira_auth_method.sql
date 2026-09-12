-- +goose Up
ALTER TABLE thunderdome.jira_instance
    ADD COLUMN auth_method varchar(10) NOT NULL DEFAULT ''
    CHECK (auth_method IN ('', 'basic', 'pat'));

-- +goose Down
ALTER TABLE thunderdome.jira_instance DROP COLUMN auth_method;
