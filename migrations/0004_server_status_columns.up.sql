-- cache the latest status on the server row so a list query needs no join.
ALTER TABLE servers ADD COLUMN online      BOOLEAN   NOT NULL DEFAULT 0;
ALTER TABLE servers ADD COLUMN players     INTEGER   NOT NULL DEFAULT 0;
ALTER TABLE servers ADD COLUMN max_players INTEGER   NOT NULL DEFAULT 0;
ALTER TABLE servers ADD COLUMN version     TEXT      NOT NULL DEFAULT '';
ALTER TABLE servers ADD COLUMN motd        TEXT      NOT NULL DEFAULT '';
ALTER TABLE servers ADD COLUMN last_check  TIMESTAMP;
