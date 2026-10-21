-- IAM identities are bound by issuer + immutable subject, never by username.
ALTER TABLE users ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE auth_sessions ADD COLUMN auth_source TEXT NOT NULL DEFAULT 'local';
CREATE TABLE federated_identities (
 issuer TEXT NOT NULL,
 subject TEXT NOT NULL,
 user_id TEXT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
 PRIMARY KEY (issuer, subject)
);
