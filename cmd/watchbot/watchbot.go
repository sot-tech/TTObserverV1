package main

import (
	"../../bot"
	"flag"
	"github.com/op/go-logging"
)

var logger = logging.MustGetLogger("main")

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		logger.Fatal("Crawler file not set")
	}
	filename := args[0]
	crawler, err := bot.ReadConfig(filename)
	if err != nil {
		logger.Fatal(err)
	}
	crawler.Engage()
}
