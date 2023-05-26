package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/Masterminds/squirrel"
	_ "github.com/lib/pq"
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
	VALUES ($1, $2, $3, $4, $5, $6, $7)`

	jsonTags, err := json.Marshal(e.Tags)
	if err != nil {
		return fmt.Errorf("unable to save event: %w", err)
	}

	_, err = db.Exec(insertStatement, e.ID, e.PubKey, e.CreatedAt, e.Kind, jsonTags, e.Content, e.Sig)
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

func selectFilteredEvents(db *sql.DB, filter nostr.Filter) ([]nostr.Event, error) {
	filterQuery := squirrel.Select("*").From("events").PlaceholderFormat(squirrel.Dollar)

	if len(filter.IDs) > 0 {
		filterQuery = filterQuery.Where(squirrel.Eq{"id": filter.IDs})
	}

	if len(filter.Kinds) > 0 {
		filterQuery = filterQuery.Where(squirrel.Eq{"kind": filter.Kinds})
	}

	if len(filter.Authors) > 0 {
		filterQuery = filterQuery.Where(squirrel.Eq{"pubkey": filter.Authors})
	}

	if filter.Since != nil {
		filterQuery = filterQuery.Where(squirrel.Gt{"created_at": filter.Since})
	}

	if filter.Until != nil {
		filterQuery = filterQuery.Where(squirrel.Lt{"created_at": filter.Until})
	}

	if filter.Limit > 0 {
		filterQuery = filterQuery.OrderBy("created_at DESC").Limit(uint64(filter.Limit))
	}

	// TODO: filter by tags

	query, args, err := filterQuery.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var filteredEvents []nostr.Event

	for rows.Next() {
		var evt nostr.Event
		if err := rows.Scan(&evt.ID, &evt.PubKey, &evt.CreatedAt, &evt.Kind,
			&evt.Tags, &evt.Content, &evt.Sig); err != nil {
			return filteredEvents, err
		}
		filteredEvents = append(filteredEvents, evt)
	}

	if err = rows.Err(); err != nil {
		return filteredEvents, err
	}

	return filteredEvents, nil
}
