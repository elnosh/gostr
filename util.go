package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const (
	EventMsg  = "EVENT"
	EoseMsg   = "EOSE"
	NoticeMsg = "NOTICE"
	OKMsg     = "OK"
)

func WriteMessage(ctx context.Context, c *websocket.Conn, msg []string) error {
	return wsjson.Write(ctx, c, msg)
}

func buildOKMessage(id, message, success string) []string {
	return []string{"OK", id, success, message}
}

func validEvent(db *sql.DB, evt nostr.Event) (bool, error) {
	eventId := evt.GetID()
	if eventId != evt.ID {
		return false, fmt.Errorf("event id")
	}

	exists, err := eventExists(db, evt.ID)
	if err != nil {
		return false, err
	}
	if exists {
		return false, fmt.Errorf("duplicate event")
	}

	validSig, err := evt.CheckSignature()
	if err != nil {
		return false, err
	}
	if !validSig {
		return false, err
	}

	return true, nil
}
