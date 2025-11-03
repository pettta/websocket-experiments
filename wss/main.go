package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	snapws "github.com/Atheer-Ganayem/SnapWS"
	kafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

var (
	manager     *snapws.Manager[string]
	kafkaEvents = newKafkaRecordStore()
)

func main() {
	// Initializing the upgrader that handles upgrading requests to Websocket.
	upgrader := snapws.NewUpgrader(nil)

	// Initializing Manager to keep track of connection and broadcast messages.
	manager = snapws.NewManager[string](upgrader)
	defer manager.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumer, err := newKafkaConsumer()
	if err != nil {
		log.Fatalf("creating Kafka consumer: %v", err)
	}
	defer consumer.Close()

	go consumeKafka(ctx, consumer)

	// Hooks
	manager.OnRegister = onRegister
	manager.OnUnregister = onUnregister

	http.HandleFunc("/ws", handler)
	http.HandleFunc("/ping", pingHandler)
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

type kafkaRecord struct {
	SubscribeID string          `json:"subscribe-id"`
	Key         string          `json:"key,omitempty"`
	Value       json.RawMessage `json:"value"`
	Partition   int32           `json:"partition"`
	Offset      int64           `json:"offset"`
}

type kafkaRecordStore struct {
	mu     sync.RWMutex
	buffer map[string][]kafkaRecord
}

const maxRecordsPerSubscribeID = 100

func newKafkaRecordStore() *kafkaRecordStore {
	return &kafkaRecordStore{buffer: make(map[string][]kafkaRecord)}
}

func (s *kafkaRecordStore) add(record kafkaRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := append(s.buffer[record.SubscribeID], record)
	if len(bucket) > maxRecordsPerSubscribeID {
		bucket = bucket[len(bucket)-maxRecordsPerSubscribeID:]
	}
	s.buffer[record.SubscribeID] = bucket
}

func (s *kafkaRecordStore) get(subscribeID string) []kafkaRecord {
	if subscribeID == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	records, ok := s.buffer[subscribeID]
	if !ok {
		return nil
	}
	copyBuf := make([]kafkaRecord, len(records))
	copy(copyBuf, records)
	return copyBuf
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

		if msg.Type != "" {
			if sendKafkaRecords(conn, msg.Type) {
				continue
			}
		}

		if targetConn := manager.Get(msg.To); targetConn != nil {
			rm := receivedMsg{Type: msg.Type, Text: fmt.Sprintf("%s: %s", name, msg.Text), From: name}
			if err := targetConn.SendJSON(context.TODO(), rm); err != nil {
				fmt.Printf("error sending message from %s to %s: %v\n", name, msg.To, err)
			}
		}
	}
}

func newKafkaConsumer() (*kafka.Consumer, error) {
	config := &kafka.ConfigMap{
		"bootstrap.servers": "localhost:9092",
		"group.id":          "wss-gateway",
		"auto.offset.reset": "earliest",
	}

	consumer, err := kafka.NewConsumer(config)
	if err != nil {
		return nil, err
	}

	if err := consumer.Subscribe("quickstart-events", nil); err != nil {
		consumer.Close()
		return nil, err
	}

	log.Println("Kafka consumer subscribed to quickstart-events")
	return consumer, nil
}

func consumeKafka(ctx context.Context, consumer *kafka.Consumer) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ev := consumer.Poll(250)
		if ev == nil {
			continue
		}

		switch e := ev.(type) {
		case *kafka.Message:
			if err := processKafkaMessage(e); err != nil {
				log.Printf("error processing Kafka message: %v", err)
			}
		case kafka.Error:
			log.Printf("Kafka consumer error: %v", e)
		}
	}
}

func processKafkaMessage(msg *kafka.Message) error {
	if msg == nil || len(msg.Value) == 0 {
		return nil
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(msg.Value, &envelope); err != nil {
		return fmt.Errorf("decode Kafka payload: %w", err)
	}

	subscribeRaw, ok := envelope["subscribe-id"]
	if !ok {
		return nil
	}

	var subscribeID string
	if err := json.Unmarshal(subscribeRaw, &subscribeID); err != nil {
		return fmt.Errorf("decode subscribe-id: %w", err)
	}
	if subscribeID == "" {
		return nil
	}

	record := kafkaRecord{
		SubscribeID: subscribeID,
		Partition:   msg.TopicPartition.Partition,
		Offset:      int64(msg.TopicPartition.Offset),
		Value:       json.RawMessage(append([]byte(nil), msg.Value...)),
	}
	if msg.Key != nil {
		record.Key = string(msg.Key)
	}

	kafkaEvents.add(record)
	return nil
}

func sendKafkaRecords(conn *snapws.ManagedConn[string], subscribeID string) bool {
	records := kafkaEvents.get(subscribeID)
	if len(records) == 0 {
		return false
	}

	payload := struct {
		Type    string        `json:"type"`
		Records []kafkaRecord `json:"records"`
	}{
		Type:    subscribeID,
		Records: records,
	}

	if err := conn.SendJSON(context.TODO(), payload); err != nil {
		log.Printf("error sending Kafka records to %s: %v", conn.Key, err)
	}
	return true
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

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("pong"))
}
