/*
 * BSD-3-Clause
 * Copyright 2020 sot (aka PR_713, C_rho_272)
 * Redistribution and use in source and binary forms, with or without modification,
 * are permitted provided that the following conditions are met:
 * 1. Redistributions of source code must retain the above copyright notice,
 * this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright notice,
 * this list of conditions and the following disclaimer in the documentation and/or
 * other materials provided with the distribution.
 * 3. Neither the name of the copyright holder nor the names of its contributors
 * may be used to endorse or promote products derived from this software without
 * specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
 * WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
 * IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT,
 * INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING,
 * BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA,
 * OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY,
 * WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
 * ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY
 * OF SUCH DAMAGE.
 */

package observer

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

type Observer struct {
	Log struct {
		File  string `json:"file"`
		Level string `json:"level"`
	}
	Crawler struct {
		URL       string `json:"url"`
		Threshold uint   `json:"threshold"`
		Delay     uint   `json:"delay"`
	} `json:"crawler"`
	SizeThreshold uint   `json:"sizethreshold"`
	TelegramToken string `json:"telegramtoken"`
	DBFile        string `json:"dbfile"`
	Messages      struct {
		Announce string `json:"announce"`
		N1000    string `json:"n1000"`
		Added    string `json:"added"`
		Updated  string `json:"updated"`
	} `json:"msg"`
	MessageReplacements map[string]string `json:"msgreplacements"`
}

func ReadConfig(path string) (*Observer, error) {
	var config Observer
	confData, err := ioutil.ReadFile(path)
	if err == nil {
		if json.Unmarshal(confData, &config) == nil {
			var outputWriter io.Writer
			if config.Log.File == "" {
				outputWriter = os.Stderr
			} else {
				outputWriter, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND, 0640)
			}
			if err != nil {
				backend := logging.AddModuleLevel(
					logging.NewBackendFormatter(
						logging.NewLogBackend(outputWriter, "", 0),
						logging.MustStringFormatter(`%{color}%{time:15:04:05.000}\t%{shortfunc}\t%{level}%{color:reset}:\t%{message}`)))
				var level logging.Level
				if level, err = logging.LogLevel(config.Log.Level); err != nil {
					println(err)
					level = logging.INFO
				}
				backend.SetLevel(level, "")
				logging.SetBackend(backend)
			}
		}
	}
	return &config, err
}

func (cr *Observer) Engage() {
	database := &intl.Database{
		Path: cr.DBFile,
	}
	telegram := &intl.Telegram{
		Token: cr.TelegramToken,
		DB:    database,
	}
	err := database.CheckConnection()
	if err == nil {
		err = telegram.Engage(-1)
		if err == nil {
			baseOffset, err := database.GetCrawlOffset()
			if err != nil {
				logger.Error(err)
			}
			for {
				var i, offset uint
				offset = baseOffset
				for i = 0; i < cr.Crawler.Threshold; i++ {
					currentOffset := offset + i
					fullUrl := fmt.Sprintf(cr.Crawler.URL, currentOffset)
					if resp, err := http.Get(fullUrl); err == nil && resp != nil {
						if torrent, err := intl.ReadTorrent(resp.Body); err != nil {
							newSize := torrent.FullSize()
							if newSize > 0 {
								action := cr.Messages.Added
								announce := true
								existSize, err := database.GetTorrentSize(torrent.Info.Name)
								if err != nil {
									logger.Error(err)
								}
								if existSize > 0 {
									size0, size1 := existSize, newSize
									if size0 > size1 {
										size1, size0 = size0, size1
									}
									if uint((size0*100)/size1) > cr.SizeThreshold {
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
								if err := database.UpdateTorrent(torrent.Info.Name, newSize); err != nil {
									logger.Error(err)
								}
								baseOffset = currentOffset
								if err := database.UpdateCrawlOffset(baseOffset); err != nil {
									logger.Error(err)
								}
							} else {
								logger.Errorf("Zero torrent size, baseOffset: ", baseOffset)
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
				time.Sleep(time.Duration(rand.Intn(int(cr.Crawler.Delay))+int(cr.Crawler.Delay)) * time.Second)
			}
		}
	}
	if err != nil {
		logger.Fatal(err)
	}
}

func (cr *Observer) Announce(telegram *intl.Telegram, action string, torrent *intl.Torrent) {

}

func (cr *Observer) N1000Get(telegram *intl.Telegram) {

}
