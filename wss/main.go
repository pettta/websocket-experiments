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
	subscribers = newKafkaSubscriptionManager()
)

const topicQuickstart = "quickstart-events"

const (
	metadataTimeoutMs  = 5000
	kafkaPollTimeoutMs = 250
)

func main() {
	upgrader := snapws.NewUpgrader(nil)
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

type kafkaRecordPayload struct {
	Type    string        `json:"type"`
	Records []kafkaRecord `json:"records"`
}

type kafkaRecordStore struct {
	mu     sync.RWMutex
	buffer map[string][]kafkaRecord
}

const maxRecordsPerSubscribeID = 100

func newKafkaRecordStore() *kafkaRecordStore {
	return &kafkaRecordStore{buffer: make(map[string][]kafkaRecord)}
}

type kafkaSubscriptionManager struct {
	mu          sync.RWMutex
	subscribers map[string]map[*snapws.ManagedConn[string]]struct{}
}

func newKafkaSubscriptionManager() *kafkaSubscriptionManager {
	return &kafkaSubscriptionManager{subscribers: make(map[string]map[*snapws.ManagedConn[string]]struct{})}
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

func (m *kafkaSubscriptionManager) add(subscribeID string, conn *snapws.ManagedConn[string]) {
	if subscribeID == "" || conn == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	set, ok := m.subscribers[subscribeID]
	if !ok {
		set = make(map[*snapws.ManagedConn[string]]struct{})
		m.subscribers[subscribeID] = set
	}
	set[conn] = struct{}{}
}

func (m *kafkaSubscriptionManager) removeConnection(conn *snapws.ManagedConn[string]) {
	if conn == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, set := range m.subscribers {
		if _, ok := set[conn]; ok {
			delete(set, conn)
			if len(set) == 0 {
				delete(m.subscribers, id)
			}
		}
	}
}

func (m *kafkaSubscriptionManager) getSubscribers(subscribeID string) []*snapws.ManagedConn[string] {
	if subscribeID == "" {
		return nil
	}
	m.mu.RLock()
	set := m.subscribers[subscribeID]
	result := make([]*snapws.ManagedConn[string], 0, len(set))
	for conn := range set {
		result = append(result, conn)
	}
	m.mu.RUnlock()
	return result
}

func (m *kafkaSubscriptionManager) notify(record kafkaRecord) {
	conns := m.getSubscribers(record.SubscribeID)
	if len(conns) == 0 {
		return
	}

	payload := newKafkaRecordPayload(record.SubscribeID, []kafkaRecord{record})

	for _, conn := range conns {
		if err := conn.SendJSON(context.TODO(), payload); err != nil {
			log.Printf("error streaming Kafka record to %s: %v", conn.Key, err)
		}
	}
}

func newKafkaRecordPayload(subscribeID string, records []kafkaRecord) kafkaRecordPayload {
	if records == nil {
		records = []kafkaRecord{}
	}
	return kafkaRecordPayload{Type: subscribeID, Records: records}
}

func assignTopicFromBeginning(consumer *kafka.Consumer, topic string) ([]kafka.TopicPartition, error) {
	metadata, err := consumer.GetMetadata(&topic, false, metadataTimeoutMs)
	if err != nil {
		return nil, fmt.Errorf("fetch metadata for %s: %w", topic, err)
	}

	topicMeta, ok := metadata.Topics[topic]
	if !ok {
		return nil, fmt.Errorf("topic %s not found in metadata", topic)
	}

	assignments := make([]kafka.TopicPartition, 0, len(topicMeta.Partitions))
	topicCopy := topic
	for _, p := range topicMeta.Partitions {
		partition := kafka.TopicPartition{Topic: &topicCopy, Partition: p.ID, Offset: kafka.Offset(kafka.OffsetBeginning)}
		assignments = append(assignments, partition)
	}

	if err := consumer.Assign(assignments); err != nil {
		return nil, fmt.Errorf("assign partitions: %w", err)
	}

	return assignments, nil
}

func loadInitialRecords(consumer *kafka.Consumer, assignments []kafka.TopicPartition) {
	if len(assignments) == 0 {
		return
	}

	reached := make(map[int32]bool)
	remaining := len(assignments)

	for remaining > 0 {
		ev := consumer.Poll(kafkaPollTimeoutMs)
		if ev == nil {
			continue
		}

		switch e := ev.(type) {
		case *kafka.Message:
			if err := processKafkaMessage(e, false); err != nil {
				log.Printf("error processing Kafka message during bootstrap: %v", err)
			}
		case kafka.PartitionEOF:
			partition := e.Partition
			if !reached[partition] {
				reached[partition] = true
				remaining--
			}
		case kafka.Error:
			log.Printf("Kafka consumer error during bootstrap: %v", e)
		default:
		}
	}
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
			handleSubscription(conn, msg.Type)
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

func newKafkaConsumer() (*kafka.Consumer, error) {
	config := &kafka.ConfigMap{
		"bootstrap.servers":    "localhost:9092",
		"group.id":             "wss-gateway",
		"auto.offset.reset":    "earliest",
		"enable.auto.commit":   false,
		"enable.partition.eof": true,
	}

	consumer, err := kafka.NewConsumer(config)
	if err != nil {
		return nil, err
	}

	assignments, err := assignTopicFromBeginning(consumer, topicQuickstart)
	if err != nil {
		consumer.Close()
		return nil, err
	}

	loadInitialRecords(consumer, assignments)

	log.Println("Kafka consumer assigned to quickstart-events from beginning")
	return consumer, nil
}

func consumeKafka(ctx context.Context, consumer *kafka.Consumer) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ev := consumer.Poll(kafkaPollTimeoutMs)
		if ev == nil {
			continue
		}

		switch e := ev.(type) {
		case *kafka.Message:
			if err := processKafkaMessage(e, true); err != nil {
				log.Printf("error processing Kafka message: %v", err)
			}
		case kafka.Error:
			log.Printf("Kafka consumer error: %v", e)
		}
	}
}

func processKafkaMessage(msg *kafka.Message, broadcast bool) error {
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
	if broadcast {
		subscribers.notify(record)
	}
	return nil
}

func handleSubscription(conn *snapws.ManagedConn[string], subscribeID string) {
	subscribers.add(subscribeID, conn)
	records := kafkaEvents.get(subscribeID)
	if err := conn.SendJSON(context.TODO(), newKafkaRecordPayload(subscribeID, records)); err != nil {
		log.Printf("error sending Kafka records to %s: %v", conn.Key, err)
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
	subscribers.removeConnection(conn)
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("pong"))
}
