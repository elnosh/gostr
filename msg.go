package main

import (
	"context"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const (
	EventMsg  = "EVENT"
	EoseMsg   = "EOSE"
	NoticeMsg = "NOTICE"
	OKMsg     = "OK"
)

func WriteMessage(ctx context.Context, c *websocket.Conn, msgType, msgStr string) error {
	msg := []string{msgType, msgStr}
	return wsjson.Write(ctx, c, msg)
}
