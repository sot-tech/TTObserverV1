package bot

import (
	"../intl"
	"encoding/json"
	"fmt"
	"github.com/op/go-logging"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"time"
)

var logger = logging.MustGetLogger("bot")

type Crawler struct {
	LogFile                string `json:"LogFile"`
	LogLevel               string `json:"LogLevel"`
	CrawlURL               string `json:"CrawlURL"`
	CrawlThreshold         uint   `json:"CrawlThreshold"`
	CrawlDelay             uint   `json:"CrawlDelay"`
	NotifyPercentThreshold uint   `json:"NotifyPercentThreshold"`
	TelegramToken          string `json:"TelegramToken"`
	DatabaseFile           string `json:"DatabaseFile"`
	Messages               struct {
		Announce string `json:"Announce"`
		N1000    string `json:"N1000"`
		Added    string `json:"Added"`
		Updated  string `json:"Updated"`
	} `json:"Messages"`
}

func ReadConfig(path string) (*Crawler, error) {
	var config Crawler
	confData, err := ioutil.ReadFile(path)
	if err == nil {
		if json.Unmarshal(confData, &config) == nil {
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
	return &config, err
}

func (cr *Crawler) Engage() {
	database := &intl.Database{}
	telegram := &intl.Telegram{
		Token: cr.TelegramToken,
		DB:    database,
	}
	if err := telegram.Engage(-1); err == nil {
		baseOffset := database.GetCrawlOffset()
		for {
			var i, offset uint
			offset = baseOffset
			for i = 0; i < cr.CrawlThreshold; i++ {
				currentOffset := offset + i
				fullUrl := fmt.Sprintf(cr.CrawlURL, currentOffset)
				if resp, err := http.Get(fullUrl); err == nil && resp != nil {
					if torrent, err := intl.ReadTorrent(resp.Body); err != nil {
						newSize := torrent.FullLength()
						if newSize > 0 {
							action := cr.Messages.Added
							announce := true
							existSize := database.GetTorrentLength(torrent.Info.Name)
							if existSize > 0 {
								size0, size1 := existSize, newSize
								if size0 > size1 {
									size1, size0 = size0, size1
								}
								if uint((size0*100)/size1) > cr.NotifyPercentThreshold {
									action = cr.Messages.Updated
								} else {
									announce = false
								}
							}
							if announce {
								go cr.Announce(telegram, action, torrent)
								if currentOffset%1000 == 0 {
									go cr.N1000Get(telegram)
								}
							}
							database.UpsertTorrent(torrent.Info.Name, newSize)
							baseOffset = currentOffset
							database.UpsertCrawlOffset(baseOffset)
						} else {
							logger.Errorf("Zero torrent length, baseOffset: ", baseOffset)
						}
					}
				} else {
					errMsg := "Empty response"
					if err != nil {
						errMsg = err.Error()
					}
					logger.Warning(errMsg)
				}
			}
			time.Sleep(time.Duration(rand.Intn(int(cr.CrawlDelay))+int(cr.CrawlDelay)) * time.Second)
		}
	} else {
		logger.Fatal(err)
	}
}

func (cr *Crawler) Announce(telegram *intl.Telegram, action string, torrent *intl.Torrent) {

}

func (cr *Crawler) N1000Get(telegram *intl.Telegram) {

}
