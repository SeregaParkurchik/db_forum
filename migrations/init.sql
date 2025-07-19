CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS ltree; 

CREATE TABLE IF NOT EXISTS users (
    nickname  CITEXT PRIMARY KEY NOT NULL,
    fullname  TEXT NOT NULL,
    email     CITEXT UNIQUE NOT NULL,
    about     TEXT
);

CREATE TABLE IF NOT EXISTS forums (
    slug          CITEXT PRIMARY KEY NOT NULL,
    title         VARCHAR(255) NOT NULL,
    user_nickname CITEXT NOT NULL REFERENCES users(nickname),
    posts         INT DEFAULT 0,
    threads       INT DEFAULT 0
);

CREATE TABLE IF NOT EXISTS threads (
    id              SERIAL PRIMARY KEY,
    title           VARCHAR(255) NOT NULL,
    author          CITEXT NOT NULL REFERENCES users(nickname),
    forum           CITEXT NOT NULL REFERENCES forums(slug),
    message         TEXT NOT NULL,
    votes           INT DEFAULT 0,
    slug            CITEXT UNIQUE,
    created         TIMESTAMP WITH TIME ZONE DEFAULT now()
);

CREATE TABLE IF NOT EXISTS posts (
    id            SERIAL PRIMARY KEY,
    parent        INT DEFAULT 0,
    author        CITEXT NOT NULL REFERENCES users(nickname),
    message       TEXT NOT NULL,
    is_edited     BOOLEAN DEFAULT FALSE,
    forum         CITEXT NOT NULL REFERENCES forums(slug),
    thread_id     INT NOT NULL REFERENCES threads(id),
    created       TIMESTAMP WITH TIME ZONE DEFAULT now(),
    path          BIGINT[], 
    root_parent_id INTEGER
);

CREATE TABLE IF NOT EXISTS votes (
    thread_id     INT NOT NULL REFERENCES threads(id),
    user_nickname CITEXT NOT NULL REFERENCES users(nickname),
    voice         INTEGER NOT NULL,
    PRIMARY KEY (thread_id, user_nickname)
);

CREATE TABLE IF NOT EXISTS forum_users (
    forum_slug    CITEXT NOT NULL REFERENCES forums(slug),
    user_nickname CITEXT NOT NULL REFERENCES users(nickname),
    PRIMARY KEY (forum_slug, user_nickname)
);

CREATE INDEX IF NOT EXISTS idx_posts_thread_id_root_parent_id_path_asc_id_asc ON posts (thread_id, root_parent_id ASC, path ASC, id ASC);
CREATE INDEX IF NOT EXISTS idx_posts_thread_id_root_parent_id_path_desc_id_desc ON posts (thread_id, root_parent_id DESC, path ASC, id DESC);


