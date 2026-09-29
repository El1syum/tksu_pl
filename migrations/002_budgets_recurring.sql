CREATE TABLE budgets (
 user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 category_id INTEGER NOT NULL REFERENCES categories(id),
 month TEXT NOT NULL CHECK(length(month)=7),
 amount INTEGER NOT NULL CHECK(amount > 0 AND amount <= 99999999999),
 PRIMARY KEY(user_id,category_id,month)
);
CREATE TABLE recurring_expenses (
 id INTEGER PRIMARY KEY,
 user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 category_id INTEGER NOT NULL REFERENCES categories(id),
 amount INTEGER NOT NULL CHECK(amount > 0 AND amount <= 99999999999),
 description TEXT NOT NULL CHECK(length(description) BETWEEN 1 AND 300),
 frequency TEXT NOT NULL CHECK(frequency IN ('daily','weekly','monthly','yearly')),
 anchor_date TEXT NOT NULL CHECK(length(anchor_date)=10),
 next_date TEXT NOT NULL CHECK(length(next_date)=10),
 end_date TEXT NOT NULL DEFAULT '',
 enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1))
);
CREATE INDEX recurring_due ON recurring_expenses(enabled,next_date,user_id);
-- Tombstones survive removal of an expense, so background retries never recreate it.
CREATE TABLE recurring_occurrences (
 recurring_id INTEGER NOT NULL REFERENCES recurring_expenses(id) ON DELETE CASCADE,
 scheduled_date TEXT NOT NULL,
 PRIMARY KEY(recurring_id,scheduled_date)
);
ALTER TABLE expenses ADD COLUMN recurring_id INTEGER REFERENCES recurring_expenses(id) ON DELETE SET NULL;
