
CREATE TABLE IF NOT EXISTS tasks (
	task TEXT NOT NULL,
	level VARCHAR(32) NOT NULL,
	answer TEXT NOT NULL,
	position INT DEFAULT 0,

	PRIMARY KEY (level, position)
);

CREATE TABLE IF NOT EXISTS words (
	word VARCHAR(255) NOT NULL,
	explain TEXT NOT NULL,
	level VARCHAR(32) NOT NULL,
	first_letter VARCHAR(1) NOT NULL,
	serial INT NOT NULL,

	PRIMARY KEY (serial)
);
