# Frontends 
## producer-consumer
Idea: 
- On the left hand side have a textbox. This textbox is entirely local. Have a button in said textbox that says "send xxx messages". When you click that it produces that many kafka messages and sends them along 

- On the right hand side, it will be synced to every single user that has this app open. It will be pulling from the websocket server backend, which on boot will seek in the kafka topic to the beginning, then send messages for every single kafka message it sees that was sent with a specfic key to the frontend. 


Purpose: 
- if there is a world where we have to get tons of messages of a massive size, i want to see what needs to be done on the backend side to make this work, and what needs to be done on the frontend side to make this work.

- if there needs to be extra work done on the backend even with a go library like this, then i know its inevitable with large data that you need to do compression + deltas 

- if there needs to only be extra work done on the frontend, then that would be amazing. 

### To Run 
development: `npm run dev`
production: `npm run build` 

# Backend 
## To run 
development `go run main.go` 
production `go build -o websocket-server main.go && ./websocket-server` 
