package main

import (
	"database/sql"
	"errors"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/mattn/go-sqlite3"
)

type OVPNClient struct {
	Id         string
	CommonName string
	IpAddress  string
	ClassName  string
}

type SQLiteRepository struct {
	db       *sql.DB
	lockFile *os.File
	mutex    sync.Mutex
}

const databaseFile string = "/etc/openvpn/plugin/db/ovpn-radius.db"
const lockFile string = "/etc/openvpn/plugin/db/ovpn-radius.db.lock"

var (
	ErrDuplicate    = errors.New("record already exists")
	ErrNotExists    = errors.New("row not exists")
	ErrUpdateFailed = errors.New("update failed")
	ErrDeleteFailed = errors.New("delete failed")
)

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{
		db: db,
	}
}

// acquireLock acquires a file-based lock for database operations
func (r *SQLiteRepository) acquireLock() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.lockFile != nil {
		return nil // Already locked
	}

	// Create lock file
	file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	// Try to acquire exclusive lock with timeout
	done := make(chan error, 1)
	go func() {
		done <- syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
	}()

	select {
	case err := <-done:
		if err != nil {
			file.Close()
			return err
		}
		r.lockFile = file
		return nil
	case <-time.After(10 * time.Second): // 10 second timeout
		file.Close()
		return errors.New("timeout acquiring database lock")
	}
}

// releaseLock releases the file-based lock
func (r *SQLiteRepository) releaseLock() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.lockFile == nil {
		return nil // Not locked
	}

	err := syscall.Flock(int(r.lockFile.Fd()), syscall.LOCK_UN)
	r.lockFile.Close()
	r.lockFile = nil
	return err
}

// Close closes the database connection and releases any locks
func (r *SQLiteRepository) Close() error {
	if err := r.releaseLock(); err != nil {
		// Log the error but continue with closing the database
	}
	return r.db.Close()
}

func (r *SQLiteRepository) Migrate() error {
	query := `
    CREATE TABLE IF NOT EXISTS OVPNClients(
        id TEXT NOT NULL UNIQUE,
        common_name TEXT NOT NULL,
        ip_address TEXT NULL,
        class_name TEXT NULL
    );
    `

	_, err := r.db.Exec(query)
	return err
}

func (r *SQLiteRepository) Create(client OVPNClient) (*OVPNClient, error) {
	if err := r.acquireLock(); err != nil {
		return nil, err
	}
	defer r.releaseLock()

	_, err := r.db.Exec("INSERT INTO OVPNClients(id, common_name, ip_address, class_name) values(?,?,?,?)", client.Id, client.CommonName, client.IpAddress, client.ClassName)

	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) {
			if errors.Is(sqliteErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
				return nil, ErrDuplicate
			}
		}
		return nil, err
	}

	return &client, nil
}

func (r *SQLiteRepository) All() ([]OVPNClient, error) {
	rows, err := r.db.Query("SELECT * FROM OVPNClients")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []OVPNClient
	for rows.Next() {
		var client OVPNClient
		if err := rows.Scan(&client.Id, &client.CommonName, &client.IpAddress, &client.ClassName); err != nil {
			return nil, err
		}
		all = append(all, client)
	}
	return all, nil
}

func (r *SQLiteRepository) GetById(id string) (*OVPNClient, error) {
	row := r.db.QueryRow("SELECT * FROM OVPNClients WHERE id = ?", id)

	var client OVPNClient
	if err := row.Scan(&client.Id, &client.CommonName, &client.IpAddress, &client.ClassName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotExists
		}
		return nil, err
	}
	return &client, nil
}

func (r *SQLiteRepository) Update(client OVPNClient) (*OVPNClient, error) {
	if len(client.Id) <= 0 {
		return nil, errors.New("invalid updated ID")
	}

	if err := r.acquireLock(); err != nil {
		return nil, err
	}
	defer r.releaseLock()

	res, err := r.db.Exec("UPDATE OVPNClients SET common_name = ?, ip_address = ?, class_name = ? WHERE id = ?", client.CommonName, client.IpAddress, client.ClassName, client.Id)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrUpdateFailed
	}

	return &client, nil
}

func (r *SQLiteRepository) Delete(id string) error {
	if err := r.acquireLock(); err != nil {
		return err
	}
	defer r.releaseLock()

	res, err := r.db.Exec("DELETE FROM OVPNClients WHERE id = ?", id)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrDeleteFailed
	}

	return err
}

func InitializeDatabase(isNewDatabase bool) (*SQLiteRepository, error) {
	if isNewDatabase {
		os.Remove(databaseFile)
		os.Remove(lockFile)
	}

	// Ensure the directory exists
	if err := os.MkdirAll("/etc/openvpn/plugin/db", 0755); err != nil && !os.IsExist(err) {
		return nil, err
	}

	// Configure SQLite3 connection string for better concurrent access
	// WAL mode allows concurrent readers and writers
	// _busy_timeout sets timeout for locked database
	// _journal_mode=WAL enables Write-Ahead Logging
	// _synchronous=NORMAL provides good performance while maintaining data integrity
	connectionString := databaseFile + "?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000"

	db, err := sql.Open("sqlite3", connectionString)
	if err != nil {
		return nil, err
	}

	// Configure connection pool for better concurrency
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	repository := NewSQLiteRepository(db)

	if err := repository.Migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return repository, nil
}
