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
				WriteMessage(ctx, c, []string{NoticeMsg, "Invalid message"})
				continue
			}

			err = json.Unmarshal(message, &v)
			if err != nil {
				return
			}

			msg, ok := v.([]interface{})
			if !ok {
				WriteMessage(ctx, c, []string{NoticeMsg, "Invalid message"})
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
					return WriteMessage(ctx, c, []string{NoticeMsg, "Invalid message"})
				}

				var evt nostr.Event
				if err = evt.UnmarshalJSON(eventJson); err != nil {
					return WriteMessage(ctx, c, []string{NoticeMsg, "Invalid message"})
				}

				valid, err := validEvent(db, evt)
				if valid {
					err = saveEvent(db, evt)
					if err != nil {
						msg := buildOKMessage(evt.ID, "error: "+err.Error(), "false")
						return WriteMessage(ctx, c, msg)
					}

					msg := buildOKMessage(evt.ID, "Event received", "true")
					return WriteMessage(ctx, c, msg)
				} else {
					msg := buildOKMessage(evt.ID, "invalid: "+err.Error(), "false")
					return WriteMessage(ctx, c, msg)
				}
			} else {
				return WriteMessage(ctx, c, []string{NoticeMsg, "bad message length"})
			}
		case "REQ":
		case "CLOSE":
		default:
			return WriteMessage(ctx, c, []string{NoticeMsg, "Invalid message"})
		}
	}
	return nil
}
