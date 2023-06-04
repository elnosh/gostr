package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/nbd-wtf/go-nostr"
)

// map subscription ids to list of filters
type Subcriptions map[string]nostr.Filters

// map open connections to all subscriptions for that connection
var OpenConns = map[*net.Conn]Subcriptions{}

func handleWS(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			log.Printf("unable to establish connection: %v", err)
			return
		}

		OpenConns[&conn] = make(Subcriptions)
		log.Println("new connection")

		go func() {
			defer func() {
				log.Println("closing connection")
				conn.Close()
			}()

			ctx := r.Context()
			for {
				//log.Println("running")
				message, err := wsutil.ReadClientText(conn)
				if err != nil {
					log.Println(err)
					return
				}

				// handle other closing cases
				// if websocket.CloseStatus(err) == websocket.StatusNoStatusRcvd {
				// 	break
				// }

				if !json.Valid(message) {
					var notice nostr.NoticeEnvelope = "Invalid json"
					WriteMessage(conn, &notice)
					continue
				}

				var rawjson []json.RawMessage
				err = json.Unmarshal(message, &rawjson)
				if err != nil {
					str := fmt.Sprintf("ERROR: %s", err.Error())
					var notice nostr.NoticeEnvelope = nostr.NoticeEnvelope(str)
					WriteMessage(conn, &notice)
					continue
				}

				err = handleMessage(ctx, &conn, db, rawjson)
				if err != nil {
					return
				}
			}
		}()
	}
}

func handleMessage(ctx context.Context, conn *net.Conn, db *sql.DB, msg []json.RawMessage) error {
	if len(msg) < 2 {
		var notice nostr.NoticeEnvelope = "ERROR: message too short"
		return WriteMessage(*conn, &notice)
	}

	var msgType string
	json.Unmarshal(msg[0], &msgType)

	switch msgType {
	case "EVENT":
		var evt nostr.Event
		if err := evt.UnmarshalJSON(msg[1]); err != nil {
			var notice nostr.NoticeEnvelope = "Invalid message"
			return WriteMessage(*conn, &notice)
		}

		valid, err := validEvent(db, evt)
		if err != nil {
			msg := buildOKMessage(evt.ID, "invalid: "+err.Error(), false)
			return WriteMessage(*conn, msg)
		}

		if valid {
			err = saveEvent(db, evt)
			if err != nil {
				msg := buildOKMessage(evt.ID, "error: "+err.Error(), false)
				//msg := buildOKMessage(evt.ID, "error: "+errors.Unwrap(err).Error(), false)
				return WriteMessage(*conn, msg)
			}

			go publishEvent(evt)

			msg := buildOKMessage(evt.ID, "Event received", true)
			return WriteMessage(*conn, msg)
		} else {
			msg := buildOKMessage(evt.ID, "invalid: bad signature", false)
			return WriteMessage(*conn, msg)
		}
	case "REQ":
		if len(msg) < 3 {
			var notice nostr.NoticeEnvelope = "ERROR: message too short"
			return WriteMessage(*conn, &notice)
		}

		var subId string
		json.Unmarshal(msg[1], &subId)

		for i := 2; i < len(msg); i++ {
			var filter nostr.Filter
			json.Unmarshal(msg[i], &filter)

			if filters, ok := OpenConns[conn][subId]; ok {
				filters = append(filters, filter)
			} else {
				OpenConns[conn][subId] = nostr.Filters{filter}
			}

			filteredEvents, err := selectFilteredEvents(db, filter)
			if err != nil {
				str := fmt.Sprintf("ERROR: %s", err.Error())
				var notice nostr.NoticeEnvelope = nostr.NoticeEnvelope(str)
				return WriteMessage(*conn, &notice)
			}

			for _, event := range filteredEvents {
				evtEnvelope := nostr.EventEnvelope{
					SubscriptionID: &subId,
					Event:          event,
				}
				WriteMessage(*conn, &evtEnvelope)
			}
		}
		var eose nostr.EOSEEnvelope = nostr.EOSEEnvelope(subId)
		return WriteMessage(*conn, &eose)
	case "CLOSE":
		var subId string
		json.Unmarshal(msg[1], &subId)
		if subs, ok := OpenConns[conn]; ok {
			if _, ok := subs[subId]; ok {
				delete(subs, subId)
				noticeStr := fmt.Sprintf("subscription '%v' closed", subId)
				var notice nostr.NoticeEnvelope = nostr.NoticeEnvelope(noticeStr)
				return WriteMessage(*conn, &notice)
			}
		}
	default:
		var notice nostr.NoticeEnvelope = "Invalid message"
		return WriteMessage(*conn, &notice)
	}
	return nil
}

// publish event to all open connections - connections will check if newly received
// event matches any of their filters and send it to client
func publishEvent(evt nostr.Event) {
	for conn, subs := range OpenConns {
		go func(conn *net.Conn, subs Subcriptions) {
			// each connection can have many subscriptions
			for subId, filters := range subs {
				// each subscription can have multiple filters
				for _, filter := range filters {
					if filter.Matches(&evt) {
						evtEnvelope := nostr.EventEnvelope{
							SubscriptionID: &subId,
							Event:          evt,
						}
						WriteMessage(*conn, &evtEnvelope)
					}
				}
			}
		}(conn, subs)
	}
}
