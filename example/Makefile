chat-server:
	go run ./chat/post.go ./chat/server.go

chat-client:
	go run ./chat/post.go ./chat/client.go

echo-server:
	go run ./echo/server.go

echo-client:
	go run ./echo/client.go

certs:
	openssl req -newkey rsa:2048 -nodes -keyout server.key -x509 -days 365 -out server.crt
