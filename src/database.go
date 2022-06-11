package main

import (
	"database/sql"
	"errors"
	"os"

	"github.com/mattn/go-sqlite3"
)

type OVPNClient struct {
	Id         string
	CommonName string
	IpAddress  string
	ClassName  string
}

type SQLiteRepository struct {
	db *sql.DB
}

const databaseFile string = "/etc/openvpn/plugin/db/ovpn-radius.db"

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
	}

	db, err := sql.Open("sqlite3", databaseFile)
	if err != nil {
		return nil, err
	}

	repository := NewSQLiteRepository(db)

	if err := repository.Migrate(); err != nil {
		return nil, err
	}

	return repository, err
}
