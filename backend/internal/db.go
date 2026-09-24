package internal

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Reading struct {
	ID          int64     `json:"id"`
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
	CreatedAt   time.Time `json:"created_at"`
}

func InitDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrasi: %w", err)
	}
	if err := seedSuperAdmin(db); err != nil {
		return nil, fmt.Errorf("seeder: %w", err)
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			email VARCHAR(190) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			role ENUM('user','super_admin') NOT NULL DEFAULT 'user',
			status ENUM('pending','approved','rejected') NOT NULL DEFAULT 'pending',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS readings (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			temperature DOUBLE NOT NULL,
			humidity DOUBLE NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_readings_created (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

// seedSuperAdmin membuat akun super admin admin@gmail.com / bagas_1911
// hanya jika belum ada super admin di database.
func seedSuperAdmin(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE role='super_admin'`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("bagas_1911"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`INSERT INTO users (name, email, password_hash, role, status) VALUES (?,?,?,?,?)`,
		"Super Admin", "admin@gmail.com", string(hash), "super_admin", "approved",
	)
	if err != nil {
		return err
	}
	log.Println("[seeder] super admin dibuat: admin@gmail.com")
	return nil
}

func FindUserByEmail(db *sql.DB, email string) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		`SELECT id, name, email, password_hash, role, status, created_at FROM users WHERE email=?`, email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func FindUserByID(db *sql.DB, id int64) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		`SELECT id, name, email, password_hash, role, status, created_at FROM users WHERE id=?`, id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func ListUsers(db *sql.DB, status string) ([]User, error) {
	query := `SELECT id, name, email, role, status, created_at FROM users`
	args := []interface{}{}
	if status != "" {
		query += ` WHERE status=?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Status, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func SetUserStatus(db *sql.DB, id int64, status string) error {
	res, err := db.Exec(`UPDATE users SET status=? WHERE id=?`, status, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func DeleteUser(db *sql.DB, id int64) error {
	res, err := db.Exec(`DELETE FROM users WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func InsertReading(db *sql.DB, temperature, humidity float64) error {
	_, err := db.Exec(`INSERT INTO readings (temperature, humidity) VALUES (?,?)`, temperature, humidity)
	return err
}

func LatestReading(db *sql.DB) (*Reading, error) {
	r := &Reading{}
	err := db.QueryRow(
		`SELECT id, temperature, humidity, created_at FROM readings ORDER BY id DESC LIMIT 1`,
	).Scan(&r.ID, &r.Temperature, &r.Humidity, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func ReadingsSince(db *sql.DB, since time.Time, limit int) ([]Reading, error) {
	rows, err := db.Query(
		`SELECT id, temperature, humidity, created_at FROM readings WHERE created_at>=? ORDER BY id DESC LIMIT ?`,
		since, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Reading{}
	for rows.Next() {
		var r Reading
		if err := rows.Scan(&r.ID, &r.Temperature, &r.Humidity, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// balikkan jadi urut naik (terlama -> terbaru)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}