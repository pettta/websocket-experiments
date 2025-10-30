# Frontends 
## producer-consumer

Idea: 
- We will have a table with a input above it for a key: It will be pulling from the websocket server backend, which on boot will seek in the kafka topic to the beginning, then send messages for every single kafka message it sees that was sent with a specfic key to the frontend. For each new message we see we add a row to that table 


Purpose: 
- if there is a world where we have to get tons of messages of a massive size, i want to see what needs to be done on the backend side to make this work, and what needs to be done on the frontend side to make this work.

- if there needs to be extra work done on the backend even with a go library like this, then i know its inevitable with large data that you need to do compression + deltas 

- if there needs to only be extra work done on the frontend, then that would be amazing. 

### To Run 
Requirements:
- nodejs / npm 

setup: `npm install` 

development: `npm run dev`

production: `npm run build` 


# Backend (WSS, Kafka)
## To run WSS 
Requirements: 
- golang

Golang Requirements: 
- snapws 
- confluentkafka

development `go run main.go` 

production `go build -o websocket-server main.go && ./websocket-server` 


## To fill data into kafka 
Requirements: 
- WSL + docker desktop + python 

Python Requirements: 
- confluentkafka 

### Setup 
Setup to get the docker image and start the container over TCP: 
- `docker pull apache/kafka:4.1.0`
- `docker run -p 9092:9092 apache/kafka:4.1.0`

Now Open a new terminal and create a topic:
- `docker ps`
- `docker exec -it <container id from previous command> sh`
- `cd /opt/kafka/`
- `bin/kafka-topics.sh --create --topic quickstart-events --bootstrap-server localhost:9092`

Verify the topic now exists and see some key metrics about it:
- `bin/kafka-topics.sh --describe --topic quickstart-events --bootstrap-server localhost:9092`
- technically from here you can read/write from a topic, but we want to automate writing to this topic so we'll make a script for that 
- to manually read from the beginning of the topic: `bin/kafka-console-consumer.sh --topic quickstart-events --from-beginning --bootstrap-server localhost:9092`

### Bulk loading data into a topic 

