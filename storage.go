package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/lib/pq"
	"github.com/nbd-wtf/go-nostr"
)

func InitDB(config Config) *sql.DB {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		config.Host, 5432, config.User, config.Password, config.DBname)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("connected to db")
	return db
}

// use context when saving e to db
func saveEvent(db *sql.DB, e nostr.Event) error {
	insertStatement := `
	INSERT INTO events (id, pubkey, created_at, kind, tags, content, sig)
	VALUES ($1, $2, to_timestamp($3), $4, $5, $6, $7)`

	_, err := db.Exec(insertStatement, e.ID, e.PubKey, e.CreatedAt, e.Kind, pq.Array(e.Tags), e.Content, e.Sig)
	if err != nil {
		return fmt.Errorf("unable to save event: %w", err)

	}
	return nil
}

func eventExists(db *sql.DB, id string) (bool, error) {
	sqlStatement := `SELECT EXISTS(SELECT 1 FROM events WHERE id=$1)`
	var exists bool

	row := db.QueryRow(sqlStatement, id)
	err := row.Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
