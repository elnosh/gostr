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

// map subscription ids to list of filters
type Subcriptions map[string]nostr.Filters

// map open connections to all subscriptions for that connection
var OpenConns = map[*websocket.Conn]Subcriptions{}

func ws(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		defer c.Close(websocket.StatusInternalError, "error with connection")
		log.Println("new connection")

		OpenConns[c] = make(Subcriptions)

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

			err = handleMessage(ctx, c, db, rawjson)
			if err != nil {
				return
			}
		}
		log.Println("closing connection")
	}
}

func handleMessage(ctx context.Context, c *websocket.Conn, db *sql.DB, msg []json.RawMessage) error {
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
				//msg := buildOKMessage(evt.ID, "error: "+errors.Unwrap(err).Error(), false)
				return WriteMessage(ctx, c, msg)
			}

			go publishEvent(ctx, evt)

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

		for i := 2; i < len(msg); i++ {
			var filter nostr.Filter
			json.Unmarshal(msg[i], &filter)

			if filters, ok := OpenConns[c][subId]; ok {
				filters = append(filters, filter)
			} else {
				OpenConns[c][subId] = nostr.Filters{filter}
			}

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
		var subId string
		json.Unmarshal(msg[1], &subId)
		if subs, ok := OpenConns[c]; ok {
			if _, ok := subs[subId]; ok {
				delete(subs, subId)
				noticeStr := fmt.Sprintf("subscription '%v' closed", subId)
				var notice nostr.NoticeEnvelope = nostr.NoticeEnvelope(noticeStr)
				return WriteMessage(ctx, c, &notice)
			}
		}
	default:
		var notice nostr.NoticeEnvelope = "Invalid message"
		return WriteMessage(ctx, c, &notice)
	}
	return nil
}

// publish event to all open connections - connections will check if newly received
// event matches any of their filters and send it to client
func publishEvent(ctx context.Context, evt nostr.Event) {
	for conn, subs := range OpenConns {
		// one goroutine for each connection
		go func(conn *websocket.Conn, subs Subcriptions) {
			// each connection can have many subscriptions
			for subId, filters := range subs {
				// each subscription can have multiple filters
				for _, filter := range filters {
					if filter.Matches(&evt) {
						evtEnvelope := nostr.EventEnvelope{
							SubscriptionID: &subId,
							Event:          evt,
						}
						WriteMessage(ctx, conn, &evtEnvelope)
					}
				}
			}
		}(conn, subs)
	}
}
