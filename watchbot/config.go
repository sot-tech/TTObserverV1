package watchbot

import (
	"encoding/json"
	"github.com/op/go-logging"
	"io"
	"io/ioutil"
	"os"
)

type Config struct {
	LogFile        string `json:"LogFile"`
	LogLevel       string `json:"LogLevel"`
	CrawlURL       string `json:"CrawlURL"`
	CrawlThreshold uint   `json:"CrawlThreshold"`
	TelegramToken  string `json:"TelegramToken"`
	Database       string `json:"Database"`
	Messages       struct {
		Message  string `json:"Message"`
		Announce string `json:"Announce"`
	} `json:"Messages"`
}

func ReadConfig(path string) (Config, error) {
	var config Config
	confData, err := ioutil.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(confData, &config)
		if err == nil {
			var outputWriter io.Writer
			if config.LogFile == "" {
				outputWriter = os.Stderr
			} else {
				outputWriter, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND, 0640)
			}
			if err != nil {
				backend := logging.AddModuleLevel(
					logging.NewBackendFormatter(
						logging.NewLogBackend(outputWriter, "", 0),
						logging.MustStringFormatter(`%{color}%{time:15:04:05.000}\t%{shortfunc}\t%{level}%{color:reset}:\t%{message}`)))
				backend.SetLevel(logging.ERROR, "")
				logging.SetBackend(backend)
			}
		}
	}
	return config, err
}
