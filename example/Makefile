chat-server:
	go run ./chat/server/main.go

chat-client:
	go run ./chat/client/main.go

echo-server:
	go run ./echo/server/main.go

echo-client:
	go run ./echo/client/main.go

certs:
	openssl req -newkey rsa:2048 -nodes -keyout server.key -x509 -days 365 -out server.crt
