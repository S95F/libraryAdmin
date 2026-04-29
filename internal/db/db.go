package db

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// LoadDotEnv reads a .env file and sets any variables that aren't already set.
func LoadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func dsn() string {
	host := envOr("PGHOST", "localhost")
	port := envOr("PGPORT", "5432")
	name := envOr("PGDATABASE", "library_db")
	user := envOr("PGUSER", "library_user")
	pass := os.Getenv("PGPASSWORD")
	ssl  := envOr("PGSSLMODE", "disable")
	return fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		host, port, name, user, pass, ssl,
	)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Open() *sql.DB {
	db, err := sql.Open("postgres", dsn())
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	if err := db.Ping(); err != nil {
		log.Fatalf("db ping: %v\nDSN: %s", err, dsn())
	}
	return db
}

// Migrate runs the schema and seeds demo data if the users table is empty.
func Migrate(db *sql.DB) error {
	log.Println("Running migrations…")
	if err := schema(db); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	log.Println("Schema applied.")
	if err := seed(db); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	return nil
}

func schema(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id            TEXT        PRIMARY KEY,
		username      TEXT        UNIQUE NOT NULL,
		email         TEXT        UNIQUE NOT NULL,
		password_hash TEXT        NOT NULL,
		role          TEXT        NOT NULL DEFAULT 'user',
		created_at    TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS books (
		id               TEXT        PRIMARY KEY,
		isbn             TEXT,
		title            TEXT        NOT NULL,
		author           TEXT        NOT NULL,
		genre            TEXT,
		description      TEXT,
		published_year   INTEGER,
		total_copies     INTEGER     NOT NULL DEFAULT 1,
		available_copies INTEGER     NOT NULL DEFAULT 1,
		created_at       TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS book_requests (
		id           TEXT        PRIMARY KEY,
		user_id      TEXT        NOT NULL REFERENCES users(id),
		book_id      TEXT        NOT NULL REFERENCES books(id),
		status       TEXT        NOT NULL DEFAULT 'pending',
		notes        TEXT,
		requested_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS checkouts (
		id             TEXT        PRIMARY KEY,
		user_id        TEXT        NOT NULL REFERENCES users(id),
		book_id        TEXT        NOT NULL REFERENCES books(id),
		clerk_id       TEXT        NOT NULL REFERENCES users(id),
		checked_out_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		due_date       TIMESTAMPTZ NOT NULL,
		returned_at    TIMESTAMPTZ
	);

	CREATE INDEX IF NOT EXISTS idx_book_requests_user   ON book_requests(user_id);
	CREATE INDEX IF NOT EXISTS idx_book_requests_status ON book_requests(status);
	CREATE INDEX IF NOT EXISTS idx_checkouts_user       ON checkouts(user_id);
	CREATE INDEX IF NOT EXISTS idx_checkouts_book       ON checkouts(book_id);
	CREATE INDEX IF NOT EXISTS idx_checkouts_returned   ON checkouts(returned_at) WHERE returned_at IS NULL;
	`)
	return err
}

func seed(db *sql.DB) error {
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if count > 0 {
		log.Println("Seed data already present — skipping.")
		return nil
	}

	hash := func(pw string) string {
		b, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
		return string(b)
	}

	users := []struct{ id, name, email, pw, role string }{
		{uuid.NewString(), "admin", "admin@library.com", "Admin123!", "admin"},
		{uuid.NewString(), "clerk", "clerk@library.com", "Clerk123!", "clerk"},
		{uuid.NewString(), "alice", "alice@example.com", "User123!", "user"},
	}
	for _, u := range users {
		if _, err := db.Exec(
			`INSERT INTO users (id,username,email,password_hash,role) VALUES ($1,$2,$3,$4,$5)`,
			u.id, u.name, u.email, hash(u.pw), u.role,
		); err != nil {
			return err
		}
	}

	books := []struct {
		title, author, genre, isbn, desc string
		year, copies                     int
	}{
		{"The Great Gatsby", "F. Scott Fitzgerald", "Fiction", "9780743273565", "A story of the fabulously wealthy Jay Gatsby.", 1925, 3},
		{"To Kill a Mockingbird", "Harper Lee", "Fiction", "9780061935466", "The story of racial injustice in the American South.", 1960, 4},
		{"1984", "George Orwell", "Dystopia", "9780451524935", "A dystopian novel set in a totalitarian society.", 1949, 5},
		{"Dune", "Frank Herbert", "Sci-Fi", "9780441013593", "Epic science fiction in a distant feudal future.", 1965, 4},
		{"The Hitchhiker's Guide to the Galaxy", "Douglas Adams", "Sci-Fi", "9780345391803", "Comic science fiction following Arthur Dent.", 1979, 3},
		{"Clean Code", "Robert C. Martin", "Technology", "9780132350884", "A handbook of agile software craftsmanship.", 2008, 2},
		{"The Pragmatic Programmer", "David Thomas", "Technology", "9780135957059", "Your journey to mastery in software development.", 1999, 2},
		{"Design Patterns", "Gang of Four", "Technology", "9780201633610", "Elements of reusable object-oriented software.", 1994, 2},
		{"Sapiens", "Yuval Noah Harari", "History", "9780062316097", "A brief history of humankind.", 2011, 3},
		{"The Alchemist", "Paulo Coelho", "Fiction", "9780062315007", "A philosophical novel about pursuing dreams.", 1988, 5},
		{"Brave New World", "Aldous Huxley", "Dystopia", "9780060850524", "A dystopian vision of a future totalitarian state.", 1932, 3},
		{"The Art of War", "Sun Tzu", "Philosophy", "9781590302255", "An ancient Chinese military treatise on strategy.", -500, 4},
		{"A Brief History of Time", "Stephen Hawking", "Science", "9780553380163", "Exploring the nature of time and the universe.", 1988, 3},
		{"Thinking, Fast and Slow", "Daniel Kahneman", "Psychology", "9780374533557", "The two systems that drive the way we think.", 2011, 3},
		{"The Catcher in the Rye", "J.D. Salinger", "Fiction", "9780316769488", "A coming-of-age story about teenage alienation.", 1951, 3},
		{"Harry Potter and the Sorcerer's Stone", "J.K. Rowling", "Fantasy", "9780590353427", "The first book in the beloved Harry Potter series.", 1997, 6},
		{"The Lord of the Rings", "J.R.R. Tolkien", "Fantasy", "9780618640157", "An epic high-fantasy adventure in Middle-earth.", 1954, 4},
		{"Crime and Punishment", "Fyodor Dostoevsky", "Fiction", "9780486415871", "A psychological novel exploring guilt and redemption.", 1866, 2},
		{"The Origin of Species", "Charles Darwin", "Science", "9780140432053", "On the origin of species by natural selection.", 1859, 2},
		{"Atomic Habits", "James Clear", "Self-Help", "9780735211292", "A framework for building good habits.", 2018, 4},
	}
	for _, b := range books {
		if _, err := db.Exec(
			`INSERT INTO books (id,isbn,title,author,genre,description,published_year,total_copies,available_copies)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			uuid.NewString(), b.isbn, b.title, b.author, b.genre, b.desc, b.year, b.copies, b.copies,
		); err != nil {
			return err
		}
	}

	log.Println("Seed complete. Admin: admin@library.com / Admin123! | Clerk: clerk@library.com / Clerk123!")
	return nil
}
