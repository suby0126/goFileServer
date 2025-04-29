
---

## 📄 3. `db/schema.sql`
-- ```sql

CREATE TABLE users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role ENUM('admin', 'user') NOT NULL DEFAULT 'user',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE files (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    original_name VARCHAR(255),
    saved_name VARCHAR(255),
    use_type VARCHAR(50),
    content_type VARCHAR(100),
    size BIGINT,
    created_at DATETIME
);

CREATE TABLE refresh_tokens (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    token TEXT NOT NULL,
    expires_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE file_history (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    file_id BIGINT,
    username VARCHAR(100),
    action ENUM('upload', 'download', 'delete'),
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);
