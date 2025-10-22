package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	snapws "github.com/Atheer-Ganayem/SnapWS"
)

var manager *snapws.Manager[string]

func main() {
	// Initializing the upgrader that handles upgrading requests to Websocket.
	upgrader := snapws.NewUpgrader(nil)

	// Initializing Manager to keep track of connection and broadcast messages.
	manager = snapws.NewManager[string](upgrader)
	defer manager.Shutdown()

	// Hooks
	manager.OnRegister = onRegister
	manager.OnUnregister = onUnregister

	http.HandleFunc("/ws", handler)
	fmt.Println("Server listening on port 8080")
	http.ListenAndServe(":8080", nil)
}

type sentMsg struct {
	Type string `json:"type"`
	Text string `json:"text"`
	To   string `json:"to"` // the user the message is meant to be sent to
}

type receivedMsg struct {
	Type string `json:"type"`
	Text string `json:"text"`
	From string `json:"from"` // the user who sent the message
}

func handler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	conn, err := manager.Connect(name, w, r)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		var msg sentMsg
		err := conn.ReadJSON(&msg)
		if snapws.IsFatalErr(err) {
			return // Connection closed
		} else if err != nil {
			fmt.Println("Non-fatal error:", err)
			continue
		}

		if msg.Type == "ping" {
			if err := conn.SendJSON(context.TODO(), map[string]string{"type": "pong"}); err != nil {
				fmt.Printf("error sending pong to %s: %v\n", name, err)
			}
			continue
		}

		if targetConn := manager.Get(msg.To); targetConn != nil {
			rm := receivedMsg{Type: msg.Type, Text: fmt.Sprintf("%s: %s", name, msg.Text), From: name}
			if err := targetConn.SendJSON(context.TODO(), rm); err != nil {
				fmt.Printf("error sending message from %s to %s: %v\n", name, msg.To, err)
			}
		}
	}
}

// This is some dummy hooks.
// In real world you might send a message to update the user's status for the other connected users.
func onRegister(conn *snapws.ManagedConn[string]) {
	id := conn.Key
	manager.BroadcastString(context.TODO(), []byte(id+" is online!"), id)
}
func onUnregister(conn *snapws.ManagedConn[string]) {
	id := conn.Key
	conn.Manager.BroadcastString(context.TODO(), []byte(id+" is offline"), id)
}
