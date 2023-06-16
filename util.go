package main

import (
	"database/sql"
	"encoding/hex"
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

func isValid32Hex(hexStr string) bool {
	hexBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return false
	}

	return len(hexBytes) == 32
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
		return false, fmt.Errorf("bad signature")
	}

	for _, tag := range evt.Tags {
		if len(tag) > 1 {
			if tag[0] == "e" || tag[0] == "p" {
				if !isValid32Hex(tag[1]) {
					return false, fmt.Errorf("bad %v tag", tag[0])
				}
			}
		}
	}

	if evt.Kind == 5 {
		if len(evt.Tags) < 1 {
			return false, fmt.Errorf("event kind '5' does not have any tags")
		} else {
			etagPresent := false
			for _, tag := range evt.Tags {
				if len(tag) > 1 && tag[0] == "e" {
					etagPresent = true
					break
				}
			}
			if !etagPresent {
				return false, fmt.Errorf("event kind '5' does not reference any events")
			}
		}
	}

	return validSig, nil
}
