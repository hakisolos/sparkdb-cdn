dev:
	go run ./cmd/main.go

start: 
	go build -o ./bin/app ./cmd/main.go && ./bin/app

tidy:
	go mod tidy

clean:
	rm ./bin/app
