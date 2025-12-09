CREATE INDEX idx_servers_host ON servers (host);
CREATE INDEX idx_samples_server_time ON status_samples (server_id, sampled_at);
