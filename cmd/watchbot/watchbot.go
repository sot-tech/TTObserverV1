package main

import (
	"flag"
	"github.com/op/go-logging"
	"../../watchbot"
)

var logger = logging.MustGetLogger("main")

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		logger.Fatal("Config file not set")
	}
	filename := args[0]
	_, err := watchbot.ReadConfig(filename)
	if err != nil {
		logger.Fatal(err)
	}
}

