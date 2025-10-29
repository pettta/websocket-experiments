import json
import datetime
import socket 
from confluent_kafka import (
    Producer as ConfluentProducer
)


kafka_host = "localhost:9092"
# USERNAME = "test" # usually would be gotten from some keyvault or env variable 
# SECURITY_PROTOCOL = "SASL_PLAINTEXT"
# SASL_MECHANISM = "SCRAM-SHA-512"


config = {
    "bootstrap.servers": kafka_host,
    "acks": 1,
    "linger.ms": 0,
    "client.id": socket.gethostname(),
}

producer= ConfluentProducer(config)
producer.produce(topic="quickstart-events", key=f"test-key-{datetime.datetime.now().isoformat()}", value=json.dumps({"test": "data"}))