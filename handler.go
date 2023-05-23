package main

import (
	"context"
	"database/sql"
	"encoding/json"
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
			var v interface{}
			_, message, err := c.Read(ctx)

			// handle other closing cases
			if websocket.CloseStatus(err) == websocket.StatusNoStatusRcvd {
				break
			}

			if !json.Valid(message) {
				var notice nostr.NoticeEnvelope = "Invalid message"
				WriteMessage(ctx, c, &notice)
				continue
			}

			err = json.Unmarshal(message, &v)
			if err != nil {
				return
			}

			msg, ok := v.([]interface{})
			if !ok {
				var notice nostr.NoticeEnvelope = "Invalid message"
				WriteMessage(ctx, c, &notice)
				continue
			}

			err = HandleMessage(ctx, c, db, msg)
			if err != nil {
				return
			}
		}
		log.Println("closing connection")
	}
}

func HandleMessage(ctx context.Context, c *websocket.Conn, db *sql.DB, msg []interface{}) error {
	for i, val := range msg {
		switch val {
		case "EVENT":
			if len(msg) > 1 {
				eventJson, err := json.Marshal(msg[i+1])
				if err != nil {
					var notice nostr.NoticeEnvelope = "Invalid message"
					return WriteMessage(ctx, c, &notice)
				}

				var evt nostr.Event
				if err = evt.UnmarshalJSON(eventJson); err != nil {
					var notice nostr.NoticeEnvelope = "Invalid message"
					return WriteMessage(ctx, c, &notice)
				}

				valid, err := validEvent(db, evt)
				if valid {
					err = saveEvent(db, evt)
					if err != nil {
						msg := buildOKMessage(evt.ID, "error: "+err.Error(), false)
						return WriteMessage(ctx, c, msg)
					}

					msg := buildOKMessage(evt.ID, "Event received", true)
					return WriteMessage(ctx, c, msg)
				} else {
					msg := buildOKMessage(evt.ID, "invalid: "+err.Error(), false)
					return WriteMessage(ctx, c, msg)
				}
			} else {
				var notice nostr.NoticeEnvelope = "bad message length"
				return WriteMessage(ctx, c, &notice)
			}
		case "REQ":
		case "CLOSE":
		default:
			var notice nostr.NoticeEnvelope = "Invalid message"
			return WriteMessage(ctx, c, &notice)
		}
	}
	return nil
}
