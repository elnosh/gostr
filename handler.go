package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/nbd-wtf/go-nostr"
	"nhooyr.io/websocket"
)

func ws(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		defer c.Close(websocket.StatusInternalError, "error with connection")
		log.Println("new connection")

		ctx := r.Context()
		for {
			_, message, err := c.Read(ctx)

			// handle other closing cases
			if websocket.CloseStatus(err) == websocket.StatusNoStatusRcvd {
				break
			}

			if !json.Valid(message) {
				var notice nostr.NoticeEnvelope = "Invalid json"
				WriteMessage(ctx, c, &notice)
				continue
			}

			var rawjson []json.RawMessage
			err = json.Unmarshal(message, &rawjson)
			if err != nil {
				str := fmt.Sprintf("ERROR: %s", err.Error())
				var notice nostr.NoticeEnvelope = nostr.NoticeEnvelope(str)
				WriteMessage(ctx, c, &notice)
				continue
			}

			err = HandleMessage(ctx, c, db, rawjson)
			if err != nil {
				return
			}
		}
		log.Println("closing connection")
	}
}

func HandleMessage(ctx context.Context, c *websocket.Conn, db *sql.DB, msg []json.RawMessage) error {
	if len(msg) < 2 {
		var notice nostr.NoticeEnvelope = "ERROR: message too short"
		return WriteMessage(ctx, c, &notice)
	}

	var msgType string
	json.Unmarshal(msg[0], &msgType)

	switch msgType {
	case "EVENT":
		var evt nostr.Event
		if err := evt.UnmarshalJSON(msg[1]); err != nil {
			var notice nostr.NoticeEnvelope = "Invalid message"
			return WriteMessage(ctx, c, &notice)
		}

		valid, err := validEvent(db, evt)
		if err != nil {
			msg := buildOKMessage(evt.ID, "invalid: "+err.Error(), false)
			return WriteMessage(ctx, c, msg)
		}

		if valid {
			err = saveEvent(db, evt)
			if err != nil {
				msg := buildOKMessage(evt.ID, "error: "+err.Error(), false)
				return WriteMessage(ctx, c, msg)
			}

			msg := buildOKMessage(evt.ID, "Event received", true)
			return WriteMessage(ctx, c, msg)
		} else {
			msg := buildOKMessage(evt.ID, "invalid: bad signature", false)
			return WriteMessage(ctx, c, msg)
		}
	case "REQ":
		if len(msg) < 3 {
			var notice nostr.NoticeEnvelope = "ERROR: message too short"
			return WriteMessage(ctx, c, &notice)
		}

		var subId string
		json.Unmarshal(msg[1], &subId)

		// if REQ has more than one filter
		for i := 2; i < len(msg); i++ {
			var filter nostr.Filter
			json.Unmarshal(msg[i], &filter)

			filteredEvents, err := selectFilteredEvents(db, filter)
			if err != nil {
				str := fmt.Sprintf("ERROR: %s", err.Error())
				var notice nostr.NoticeEnvelope = nostr.NoticeEnvelope(str)
				return WriteMessage(ctx, c, &notice)
			}

			for _, event := range filteredEvents {
				evtEnvelope := nostr.EventEnvelope{
					SubscriptionID: &subId,
					Event:          event,
				}
				WriteMessage(ctx, c, &evtEnvelope)
			}

			var eose nostr.EOSEEnvelope = nostr.EOSEEnvelope(subId)
			return WriteMessage(ctx, c, &eose)
		}
	case "CLOSE":
	default:
		var notice nostr.NoticeEnvelope = "Invalid message"
		return WriteMessage(ctx, c, &notice)
	}
	return nil
}
