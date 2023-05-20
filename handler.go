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
				err = WriteMessage(ctx, c, NoticeMsg, "Invalid Message")
				if err != nil {
					return
				}
				continue
			}

			err = json.Unmarshal(message, &v)
			if err != nil {
				return
			}

			msg, ok := v.([]interface{})
			if !ok {
				err = WriteMessage(ctx, c, NoticeMsg, "Invalid Message")
				if err != nil {
					return
				}
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
					return WriteMessage(ctx, c, NoticeMsg, "Invalid Message")
				}

				var evt nostr.Event
				if err = evt.UnmarshalJSON(eventJson); err != nil {
					log.Println(err)
					return WriteMessage(ctx, c, NoticeMsg, "Invalid Message")
				}

				validSig, err := evt.CheckSignature()
				if err != nil {
					log.Println(err)
					return WriteMessage(ctx, c, NoticeMsg, "Invalid Message: "+err.Error())
				}
				if !validSig {
					return WriteMessage(ctx, c, NoticeMsg, "Invalid signature: "+err.Error())
				} else {
					// use context when saving event to db
					err = saveEvent(db, evt)
					if err != nil {
						log.Println(err)
						return WriteMessage(ctx, c, NoticeMsg, "Invalid Message")
					}

					// send OK msg if successfully saved event to DB
					// look at nip-20 for how to handle command results
					return WriteMessage(ctx, c, OKMsg, "Event received")
				}
			} else {
				return WriteMessage(ctx, c, NoticeMsg, "Invalid Message")
			}
		case "REQ":
		case "CLOSE":
		default:
			return WriteMessage(ctx, c, NoticeMsg, "Invalid Message")
		}
	}
	return nil
}
