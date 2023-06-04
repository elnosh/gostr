package main

import (
	"database/sql"
	"fmt"
	"io"

	"github.com/gobwas/ws/wsutil"
	"github.com/nbd-wtf/go-nostr"
)

func WriteMessage(w io.Writer, msgEnvelope nostr.Envelope) error {
	jsonMsg, err := msgEnvelope.MarshalJSON()
	if err != nil {
		return err
	}
	return wsutil.WriteServerText(w, jsonMsg)
}

func buildOKMessage(id, message string, success bool) *nostr.OKEnvelope {
	return &nostr.OKEnvelope{EventID: id, OK: success, Reason: &message}
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

	return validSig, nil
}
