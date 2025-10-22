# Frontends 
## producer-consumer
Idea: 
- We will have a table with a input above it for a key: It will be pulling from the websocket server backend, which on boot will seek in the kafka topic to the beginning, then send messages for every single kafka message it sees that was sent with a specfic key to the frontend. For each new message we see we add a row to that table 


Purpose: 
- if there is a world where we have to get tons of messages of a massive size, i want to see what needs to be done on the backend side to make this work, and what needs to be done on the frontend side to make this work.

- if there needs to be extra work done on the backend even with a go library like this, then i know its inevitable with large data that you need to do compression + deltas 

- if there needs to only be extra work done on the frontend, then that would be amazing. 

### To Run 
development: `npm run dev`
production: `npm run build` 

# Backend (WSS, Kafka)
## To run WSS 
development `go run main.go` 
production `go build -o websocket-server main.go && ./websocket-server` 

## To fill data into kafka 

