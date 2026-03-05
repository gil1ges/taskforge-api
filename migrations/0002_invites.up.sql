CREATE TABLE team_invites (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  team_id BIGINT UNSIGNED NOT NULL,
  email VARCHAR(255) NOT NULL,
  role ENUM('admin','member') NOT NULL,
  invited_by BIGINT UNSIGNED NOT NULL,
  code_hash BINARY(32) NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_inv_team_email (team_id, email),
  KEY ix_inv_expires (expires_at),
  CONSTRAINT fk_inv_team FOREIGN KEY (team_id) REFERENCES teams(id),
  CONSTRAINT fk_inv_invited_by FOREIGN KEY (invited_by) REFERENCES users(id)
) ENGINE=InnoDB;