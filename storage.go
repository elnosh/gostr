package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
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

// pass context when saving e to db
func saveEvent(db *sql.DB, e Event) error {
	insertStatement := `
	INSERT INTO events (id, pubkey, created_at, kind, content, sig)
	VALUES ($1, $2, to_timestamp($3), $4, $5, $6)`

	_, err := db.Exec(insertStatement, e.Id, e.Pubkey, e.CreatedAt, e.Kind, e.Content, e.Sig)
	if err != nil {
		return fmt.Errorf("error saving event: %w", err)

	}
	return nil
}
