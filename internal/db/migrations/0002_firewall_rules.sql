CREATE TABLE IF NOT EXISTS firewall_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    port TEXT NOT NULL,
    protocol TEXT DEFAULT 'tcp',
    action TEXT DEFAULT 'ALLOW',
    source TEXT DEFAULT 'Anywhere',
    comment TEXT
);

-- Seed initial rules to match setup
INSERT OR IGNORE INTO firewall_rules (port, protocol, action, source, comment) VALUES 
('22', 'tcp', 'ALLOW', 'Anywhere', 'SSH'),
('53', 'tcp', 'ALLOW', 'Anywhere', 'DNS'),
('53', 'udp', 'ALLOW', 'Anywhere', 'DNS'),
('80', 'tcp', 'ALLOW', 'Anywhere', 'HTTP Web Traffic'),
('443', 'tcp', 'ALLOW', 'Anywhere', 'HTTPS Web Traffic'),
('21', 'tcp', 'ALLOW', 'Anywhere', 'FTP'),
('25', 'tcp', 'ALLOW', 'Anywhere', 'SMTP'),
('143', 'tcp', 'ALLOW', 'Anywhere', 'IMAP'),
('465', 'tcp', 'ALLOW', 'Anywhere', 'SMTPS'),
('587', 'tcp', 'ALLOW', 'Anywhere', 'SMTP Submission'),
('993', 'tcp', 'ALLOW', 'Anywhere', 'IMAPS'),
('995', 'tcp', 'ALLOW', 'Anywhere', 'POP3S'),
('8080', 'tcp', 'ALLOW', 'Anywhere', 'Dashboard HTTP'),
('8443', 'tcp', 'ALLOW', 'Anywhere', 'Dashboard HTTPS');
