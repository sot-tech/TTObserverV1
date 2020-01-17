default:	ttobserver

ttobserver:	*.go ../.. ../../intl/*.go
	GOPATH=$(realpath ../../)
	go get github.com/op/go-logging github.com/zeebo/bencode \
		github.com/go-telegram-bot-api/telegram-bot-api \
		github.com/xlzd/gotp \
		github.com/mattn/go-sqlite3
	go build