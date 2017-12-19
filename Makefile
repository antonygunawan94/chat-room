run: build start

build:
	@echo " >> building binaries"
	@go build -o bin/chat-room

start:
	@echo " >> starting binaries"
	@./bin/chat-room
