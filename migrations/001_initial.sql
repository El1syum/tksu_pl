CREATE TABLE users (
 id INTEGER PRIMARY KEY,
 email TEXT NOT NULL UNIQUE COLLATE NOCASE,
 password_hash TEXT NOT NULL,
 created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE categories (
 id INTEGER PRIMARY KEY,
 name TEXT NOT NULL UNIQUE,
 color TEXT NOT NULL,
 icon TEXT NOT NULL
);
INSERT INTO categories (id,name,color,icon) VALUES
 (1,'Еда','#e6a35b','Е'),(2,'Транспорт','#649cce','Т'),
 (3,'Жильё','#a48acc','Ж'),(4,'Развлечения','#db869b','Р'),
 (5,'Здоровье','#69a696','З'),(6,'Покупки','#8e9dd0','П'),
 (7,'Образование','#c5a457','О'),(8,'Другое','#98a1a7','Д');
CREATE TABLE expenses (
 id INTEGER PRIMARY KEY,
 user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 category_id INTEGER NOT NULL REFERENCES categories(id),
 amount INTEGER NOT NULL CHECK (amount > 0 AND amount <= 99999999999),
 description TEXT NOT NULL CHECK (length(description) BETWEEN 1 AND 300),
 date TEXT NOT NULL CHECK (length(date) = 10),
 created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX expenses_user_date ON expenses(user_id,date DESC,id DESC);
CREATE INDEX expenses_user_category_date ON expenses(user_id,category_id,date);
CREATE TABLE sessions (
 session_id TEXT PRIMARY KEY,
 user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 expires_at INTEGER NOT NULL
);
CREATE INDEX sessions_expiry ON sessions(expires_at);
