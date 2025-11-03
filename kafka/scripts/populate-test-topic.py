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

key = datetime.datetime.now().isoformat()
s= json.dumps({'a': 'b'*900_000,
               'subscribe-id': 'abc123'})

producer= ConfluentProducer(config)
producer.produce(topic="quickstart-events", key=key, value=s)
producer.flush()