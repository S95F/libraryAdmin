package models

import (
	"database/sql"
	"fmt"
	"time"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

type Book struct {
	ID              string    `json:"id"`
	ISBN            string    `json:"isbn"`
	Title           string    `json:"title"`
	Author          string    `json:"author"`
	Genre           string    `json:"genre"`
	Description     string    `json:"description"`
	PublishedYear   int       `json:"published_year"`
	TotalCopies     int       `json:"total_copies"`
	AvailableCopies int       `json:"available_copies"`
	CreatedAt       time.Time `json:"created_at"`
}

type BookRequest struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	BookID      string    `json:"book_id"`
	Status      string    `json:"status"`
	Notes       string    `json:"notes"`
	RequestedAt time.Time `json:"requested_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Book        *Book     `json:"book,omitempty"`
	User        *User     `json:"user,omitempty"`
}

type Checkout struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	BookID       string     `json:"book_id"`
	ClerkID      string     `json:"clerk_id"`
	CheckedOutAt time.Time  `json:"checked_out_at"`
	DueDate      time.Time  `json:"due_date"`
	ReturnedAt   *time.Time `json:"returned_at"`
	IsOverdue    bool       `json:"is_overdue"`
	Book         *Book      `json:"book,omitempty"`
	User         *User      `json:"user,omitempty"`
	Clerk        *User      `json:"clerk,omitempty"`
}

type Analytics struct {
	TotalBooks      int           `json:"total_books"`
	TotalCopies     int           `json:"total_copies"`
	CheckedOut      int           `json:"checked_out"`
	Available       int           `json:"available"`
	Overdue         int           `json:"overdue"`
	TotalUsers      int           `json:"total_users"`
	PendingRequests int           `json:"pending_requests"`
	TopBooks        []BookStat    `json:"top_books"`
	GenreStats      []GenreStat   `json:"genre_stats"`
	MonthlyStats    []MonthlyStat `json:"monthly_stats"`
	RecentActivity  []Checkout    `json:"recent_activity"`
}

type BookStat    struct { Book Book `json:"book"`; CheckoutCount int `json:"checkout_count"` }
type GenreStat   struct { Genre string `json:"genre"`;  Count int    `json:"count"` }
type MonthlyStat struct { Month string `json:"month"`;  Count int    `json:"count"` }

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// ─── Users ────────────────────────────────────────────────────────────────────

func (s *Store) GetUserByEmail(email string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id,username,email,password_hash,role,created_at FROM users WHERE email=$1`, email,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	return u, err
}

func (s *Store) GetUserByID(id string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id,username,email,password_hash,role,created_at FROM users WHERE id=$1`, id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	return u, err
}

func (s *Store) CreateUser(id, username, email, hash, role string) error {
	_, err := s.db.Exec(
		`INSERT INTO users (id,username,email,password_hash,role) VALUES ($1,$2,$3,$4,$5)`,
		id, username, email, hash, role,
	)
	return err
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id,username,email,role,created_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.CreatedAt)
		users = append(users, u)
	}
	return users, nil
}

func (s *Store) UpdateUserRole(id, role string) error {
	_, err := s.db.Exec(`UPDATE users SET role=$1 WHERE id=$2`, role, id)
	return err
}

// ─── Books ────────────────────────────────────────────────────────────────────

func (s *Store) ListBooks(search, genre string) ([]Book, error) {
	q := `SELECT id,isbn,title,author,genre,description,published_year,total_copies,available_copies,created_at
	      FROM books WHERE 1=1`
	args := []interface{}{}
	n := 1
	if search != "" {
		q += fmt.Sprintf(` AND (title ILIKE $%d OR author ILIKE $%d OR isbn ILIKE $%d)`, n, n+1, n+2)
		like := "%" + search + "%"
		args = append(args, like, like, like)
		n += 3
	}
	if genre != "" {
		q += fmt.Sprintf(` AND genre=$%d`, n)
		args = append(args, genre)
		n++
	}
	q += ` ORDER BY title`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var books []Book
	for rows.Next() {
		var b Book
		rows.Scan(&b.ID, &b.ISBN, &b.Title, &b.Author, &b.Genre, &b.Description,
			&b.PublishedYear, &b.TotalCopies, &b.AvailableCopies, &b.CreatedAt)
		books = append(books, b)
	}
	return books, nil
}

func (s *Store) GetBookByID(id string) (*Book, error) {
	b := &Book{}
	err := s.db.QueryRow(
		`SELECT id,isbn,title,author,genre,description,published_year,total_copies,available_copies,created_at
		 FROM books WHERE id=$1`, id,
	).Scan(&b.ID, &b.ISBN, &b.Title, &b.Author, &b.Genre, &b.Description,
		&b.PublishedYear, &b.TotalCopies, &b.AvailableCopies, &b.CreatedAt)
	return b, err
}

func (s *Store) CreateBook(id, isbn, title, author, genre, desc string, year, copies int) error {
	_, err := s.db.Exec(
		`INSERT INTO books (id,isbn,title,author,genre,description,published_year,total_copies,available_copies)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		id, isbn, title, author, genre, desc, year, copies, copies,
	)
	return err
}

func (s *Store) UpdateBook(id, isbn, title, author, genre, desc string, year, total int) error {
	_, err := s.db.Exec(
		`UPDATE books
		 SET isbn=$1,title=$2,author=$3,genre=$4,description=$5,published_year=$6,
		     total_copies=$7,
		     available_copies=available_copies+($7-total_copies)
		 WHERE id=$8`,
		isbn, title, author, genre, desc, year, total, id,
	)
	return err
}

func (s *Store) DeleteBook(id string) error {
	_, err := s.db.Exec(`DELETE FROM books WHERE id=$1`, id)
	return err
}

func (s *Store) ListGenres() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT genre FROM books WHERE genre IS NOT NULL AND genre<>'' ORDER BY genre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var genres []string
	for rows.Next() {
		var g string
		rows.Scan(&g)
		genres = append(genres, g)
	}
	return genres, nil
}

// ─── Book Requests ────────────────────────────────────────────────────────────

func (s *Store) CreateRequest(id, userID, bookID, notes string) error {
	var exists int
	s.db.QueryRow(
		`SELECT COUNT(*) FROM book_requests WHERE user_id=$1 AND book_id=$2 AND status IN ('pending','approved')`,
		userID, bookID,
	).Scan(&exists)
	if exists > 0 {
		return fmt.Errorf("you already have an active request for this book")
	}
	_, err := s.db.Exec(
		`INSERT INTO book_requests (id,user_id,book_id,status,notes) VALUES ($1,$2,$3,'pending',$4)`,
		id, userID, bookID, notes,
	)
	return err
}

func (s *Store) GetUserRequests(userID string) ([]BookRequest, error) {
	rows, err := s.db.Query(`
		SELECT br.id,br.user_id,br.book_id,br.status,COALESCE(br.notes,''),br.requested_at,br.updated_at,
		       b.id,b.isbn,b.title,b.author,b.genre,b.available_copies
		FROM book_requests br
		JOIN books b ON b.id=br.book_id
		WHERE br.user_id=$1
		ORDER BY br.requested_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reqs []BookRequest
	for rows.Next() {
		var r BookRequest
		var b Book
		rows.Scan(&r.ID, &r.UserID, &r.BookID, &r.Status, &r.Notes, &r.RequestedAt, &r.UpdatedAt,
			&b.ID, &b.ISBN, &b.Title, &b.Author, &b.Genre, &b.AvailableCopies)
		r.Book = &b
		reqs = append(reqs, r)
	}
	return reqs, nil
}

func (s *Store) ListAllRequests(status string) ([]BookRequest, error) {
	q := `
		SELECT br.id,br.user_id,br.book_id,br.status,COALESCE(br.notes,''),br.requested_at,br.updated_at,
		       b.id,b.isbn,b.title,b.author,b.genre,
		       u.id,u.username,u.email
		FROM book_requests br
		JOIN books b ON b.id=br.book_id
		JOIN users u ON u.id=br.user_id
		WHERE 1=1`
	args := []interface{}{}
	if status != "" {
		q += ` AND br.status=$1`
		args = append(args, status)
	}
	q += ` ORDER BY br.requested_at DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reqs []BookRequest
	for rows.Next() {
		var r BookRequest
		var b Book
		var u User
		rows.Scan(&r.ID, &r.UserID, &r.BookID, &r.Status, &r.Notes, &r.RequestedAt, &r.UpdatedAt,
			&b.ID, &b.ISBN, &b.Title, &b.Author, &b.Genre,
			&u.ID, &u.Username, &u.Email)
		r.Book = &b
		r.User = &u
		reqs = append(reqs, r)
	}
	return reqs, nil
}

func (s *Store) UpdateRequestStatus(id, status string) error {
	_, err := s.db.Exec(
		`UPDATE book_requests SET status=$1,updated_at=CURRENT_TIMESTAMP WHERE id=$2`, status, id,
	)
	return err
}

func (s *Store) GetApprovedRequestsForUser(userID string) ([]BookRequest, error) {
	rows, err := s.db.Query(`
		SELECT br.id,br.user_id,br.book_id,br.status,COALESCE(br.notes,''),br.requested_at,br.updated_at,
		       b.id,b.isbn,b.title,b.author,b.genre,b.available_copies
		FROM book_requests br
		JOIN books b ON b.id=br.book_id
		WHERE br.user_id=$1 AND br.status='approved'
		ORDER BY br.requested_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reqs []BookRequest
	for rows.Next() {
		var r BookRequest
		var b Book
		rows.Scan(&r.ID, &r.UserID, &r.BookID, &r.Status, &r.Notes, &r.RequestedAt, &r.UpdatedAt,
			&b.ID, &b.ISBN, &b.Title, &b.Author, &b.Genre, &b.AvailableCopies)
		r.Book = &b
		reqs = append(reqs, r)
	}
	return reqs, nil
}

// ─── Checkouts ────────────────────────────────────────────────────────────────

func (s *Store) CreateCheckout(id, userID, bookID, clerkID string, due time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var avail int
	tx.QueryRow(`SELECT available_copies FROM books WHERE id=$1 FOR UPDATE`, bookID).Scan(&avail)
	if avail < 1 {
		return fmt.Errorf("no copies available")
	}
	if _, err := tx.Exec(
		`INSERT INTO checkouts (id,user_id,book_id,clerk_id,due_date) VALUES ($1,$2,$3,$4,$5)`,
		id, userID, bookID, clerkID, due,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE books SET available_copies=available_copies-1 WHERE id=$1`, bookID); err != nil {
		return err
	}
	tx.Exec(
		`UPDATE book_requests SET status='fulfilled',updated_at=CURRENT_TIMESTAMP
		 WHERE user_id=$1 AND book_id=$2 AND status='approved'`,
		userID, bookID,
	)
	return tx.Commit()
}

func (s *Store) ReturnBook(checkoutID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var bookID string
	var returnedAt sql.NullTime
	tx.QueryRow(`SELECT book_id,returned_at FROM checkouts WHERE id=$1`, checkoutID).
		Scan(&bookID, &returnedAt)
	if bookID == "" {
		return fmt.Errorf("checkout not found")
	}
	if returnedAt.Valid {
		return fmt.Errorf("book already returned")
	}
	if _, err := tx.Exec(`UPDATE checkouts SET returned_at=CURRENT_TIMESTAMP WHERE id=$1`, checkoutID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE books SET available_copies=available_copies+1 WHERE id=$1`, bookID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetActiveCheckoutsForUser(userID string) ([]Checkout, error) {
	rows, err := s.db.Query(`
		SELECT c.id,c.user_id,c.book_id,c.clerk_id,c.checked_out_at,c.due_date,c.returned_at,
		       b.id,b.isbn,b.title,b.author,b.genre
		FROM checkouts c
		JOIN books b ON b.id=c.book_id
		WHERE c.user_id=$1 AND c.returned_at IS NULL
		ORDER BY c.due_date ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCheckoutsWithBook(rows)
}

func (s *Store) ListAllCheckouts(activeOnly bool) ([]Checkout, error) {
	q := `
		SELECT c.id,c.user_id,c.book_id,c.clerk_id,c.checked_out_at,c.due_date,c.returned_at,
		       b.id,b.isbn,b.title,b.author,b.genre,
		       u.id,u.username,u.email
		FROM checkouts c
		JOIN books b ON b.id=c.book_id
		JOIN users u ON u.id=c.user_id
		WHERE 1=1`
	if activeOnly {
		q += ` AND c.returned_at IS NULL`
	}
	q += ` ORDER BY c.checked_out_at DESC LIMIT 200`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Checkout
	now := time.Now()
	for rows.Next() {
		var c Checkout
		var b Book
		var u User
		var returnedAt sql.NullTime
		rows.Scan(&c.ID, &c.UserID, &c.BookID, &c.ClerkID, &c.CheckedOutAt, &c.DueDate, &returnedAt,
			&b.ID, &b.ISBN, &b.Title, &b.Author, &b.Genre,
			&u.ID, &u.Username, &u.Email)
		if returnedAt.Valid {
			c.ReturnedAt = &returnedAt.Time
		}
		c.IsOverdue = c.ReturnedAt == nil && now.After(c.DueDate)
		c.Book = &b
		c.User = &u
		out = append(out, c)
	}
	return out, nil
}

func scanCheckoutsWithBook(rows *sql.Rows) ([]Checkout, error) {
	var out []Checkout
	now := time.Now()
	for rows.Next() {
		var c Checkout
		var b Book
		var returnedAt sql.NullTime
		rows.Scan(&c.ID, &c.UserID, &c.BookID, &c.ClerkID, &c.CheckedOutAt, &c.DueDate, &returnedAt,
			&b.ID, &b.ISBN, &b.Title, &b.Author, &b.Genre)
		if returnedAt.Valid {
			c.ReturnedAt = &returnedAt.Time
		}
		c.IsOverdue = c.ReturnedAt == nil && now.After(c.DueDate)
		c.Book = &b
		out = append(out, c)
	}
	return out, rows.Err()
}

// ─── Analytics ────────────────────────────────────────────────────────────────

func (s *Store) GetAnalytics() (*Analytics, error) {
	a := &Analytics{}

	s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(total_copies),0), COALESCE(SUM(available_copies),0) FROM books`,
	).Scan(&a.TotalBooks, &a.TotalCopies, &a.Available)
	a.CheckedOut = a.TotalCopies - a.Available

	s.db.QueryRow(
		`SELECT COUNT(*) FROM checkouts WHERE returned_at IS NULL AND due_date < CURRENT_TIMESTAMP`,
	).Scan(&a.Overdue)
	s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&a.TotalUsers)
	s.db.QueryRow(`SELECT COUNT(*) FROM book_requests WHERE status='pending'`).Scan(&a.PendingRequests)

	// Top books
	if rows, err := s.db.Query(`
		SELECT b.id,b.isbn,b.title,b.author,b.genre,b.total_copies,b.available_copies, COUNT(c.id) cnt
		FROM books b LEFT JOIN checkouts c ON c.book_id=b.id
		GROUP BY b.id ORDER BY cnt DESC LIMIT 10`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var bs BookStat
			rows.Scan(&bs.Book.ID, &bs.Book.ISBN, &bs.Book.Title, &bs.Book.Author,
				&bs.Book.Genre, &bs.Book.TotalCopies, &bs.Book.AvailableCopies, &bs.CheckoutCount)
			a.TopBooks = append(a.TopBooks, bs)
		}
	}

	// Genre distribution
	if rows, err := s.db.Query(
		`SELECT genre, COUNT(*) FROM books WHERE genre IS NOT NULL AND genre<>'' GROUP BY genre ORDER BY COUNT(*) DESC`,
	); err == nil {
		defer rows.Close()
		for rows.Next() {
			var gs GenreStat
			rows.Scan(&gs.Genre, &gs.Count)
			a.GenreStats = append(a.GenreStats, gs)
		}
	}

	// Monthly stats — PostgreSQL TO_CHAR
	if rows, err := s.db.Query(`
		SELECT TO_CHAR(checked_out_at,'YYYY-MM') AS month, COUNT(*)
		FROM checkouts
		WHERE checked_out_at >= NOW() - INTERVAL '6 months'
		GROUP BY month ORDER BY month`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var ms MonthlyStat
			rows.Scan(&ms.Month, &ms.Count)
			a.MonthlyStats = append(a.MonthlyStats, ms)
		}
	}

	// Recent activity
	if rows, err := s.db.Query(`
		SELECT c.id,c.user_id,c.book_id,c.clerk_id,c.checked_out_at,c.due_date,c.returned_at,
		       b.id,b.isbn,b.title,b.author,b.genre,
		       u.id,u.username,u.email
		FROM checkouts c
		JOIN books b ON b.id=c.book_id
		JOIN users u ON u.id=c.user_id
		ORDER BY c.checked_out_at DESC LIMIT 15`); err == nil {
		defer rows.Close()
		now := time.Now()
		for rows.Next() {
			var c Checkout
			var b Book
			var u User
			var returnedAt sql.NullTime
			rows.Scan(&c.ID, &c.UserID, &c.BookID, &c.ClerkID, &c.CheckedOutAt, &c.DueDate, &returnedAt,
				&b.ID, &b.ISBN, &b.Title, &b.Author, &b.Genre,
				&u.ID, &u.Username, &u.Email)
			if returnedAt.Valid {
				c.ReturnedAt = &returnedAt.Time
			}
			c.IsOverdue = c.ReturnedAt == nil && now.After(c.DueDate)
			c.Book = &b
			c.User = &u
			a.RecentActivity = append(a.RecentActivity, c)
		}
	}

	return a, nil
}
