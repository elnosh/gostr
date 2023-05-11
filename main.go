package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

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
		//log.Println("new connection")

		// when creating method to handle the event, it should take the context
		for {
			var v interface{}
			_, message, err := c.Read(r.Context())
			// handle other closing cases
			if websocket.CloseStatus(err) == websocket.StatusNoStatusRcvd {
				break
			}

			if !json.Valid(message) {
				err = WriteMessage(r.Context(), c, NoticeMsg, "Invalid Message")
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
				err = WriteMessage(r.Context(), c, NoticeMsg, "Invalid Message")
				if err != nil {
					return
				}
				continue
			}

		messageLoop:
			for i, val := range msg {
				switch val {
				case "EVENT":
					if len(msg) > 1 {
						eventJson, err := json.Marshal(msg[i+1])
						if err != nil {
							// send NOTICE msg with err
							err = WriteMessage(r.Context(), c, NoticeMsg, "Invalid Message")
							if err != nil {
								return
							}
							break messageLoop
						}

						var e Event
						if err = json.Unmarshal(eventJson, &e); err != nil {
							// send NOTICE msg with err
							err = WriteMessage(r.Context(), c, NoticeMsg, "Invalid Message")
							if err != nil {
								return
							}
							break messageLoop
						}

						// check in db if event is duplicate
						// use context when saving event to db
						err = saveEvent(db, e)
						if err != nil {
							return
						}
						// send OK msg if successfully saved event to DB
						// look at nip-20 for how to handle command results
						err = WriteMessage(r.Context(), c, OKMsg, "Event received")
						if err != nil {
							return
						}
						fmt.Printf("saved event = %v to db\n", e)
						break messageLoop
					}
				case "REQ":
				case "CLOSE":
				default:
					err = WriteMessage(r.Context(), c, NoticeMsg, "Invalid Message")
					if err != nil {
						return
					}
					break messageLoop
				}
			}
		}
	}
}

func main() {
	config := GetConfig("config.json")
	db := InitDB(config)

	log.Println("relay is running")
	log.Fatal(http.ListenAndServe(":8080", ws(db)))
}
