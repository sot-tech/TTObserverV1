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
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"
)

type Observer struct {
	Log struct {
		File  string `json:"file"`
		Level string `json:"level"`
	} `json:"log"`
	Crawler struct {
		URL            string `json:"url"`
		Threshold      uint   `json:"threshold"`
		ErrorThreshold uint   `json:"errorthreshold"`
		Delay          uint   `json:"delay"`
	} `json:"crawler"`
	SizeThreshold uint   `json:"sizethreshold"`
	TelegramToken string `json:"telegramtoken"`
	AdminOTPSeed  string `json:"adminotpseed"`
	DBFile        string `json:"dbfile"`
	Messages      struct {
		Announce string `json:"announce"`
		N1000    string `json:"n1000"`
		Added    string `json:"added"`
		Updated  string `json:"updated"`
		Error    string `json:"error"`
	} `json:"msg"`
	MessageReplacements map[string]string `json:"msgreplacements"`
}

func ReadConfig(path string) (*Observer, error) {
	var config Observer
	confData, err := ioutil.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(confData, &config)
	}
	return &config, err
}

func (cr *Observer) Engage() {
	database := &intl.Database{}
	telegram := &intl.Telegram{
		DB:    database,
	}
	err := database.Connect(cr.DBFile)
	if err == nil {
		defer database.Close()
		err = telegram.Connect(cr.TelegramToken, cr.AdminOTPSeed, -1)
		if err == nil {
			baseOffset, err := database.GetCrawlOffset()
			if err != nil {
				intl.Logger.Error(err)
			}
			var errCount uint
			for {
				var i, offset uint
				offset = baseOffset
				for i = 0; i < cr.Crawler.Threshold; i++ {
					currentOffset := offset + i
					intl.Logger.Debugf("Checking offset %d", currentOffset)
					fullUrl := fmt.Sprintf(cr.Crawler.URL, currentOffset)
					if resp, err := http.Get(fullUrl); err == nil && resp != nil && resp.StatusCode < 400 {
						errCount = 0
						if torrent, err := intl.ReadTorrent(resp.Body); err == nil {
							intl.Logger.Infof("New file: %s", torrent.Info.Name)
							torrent.URL = fullUrl
							newSize := torrent.FullSize()
							intl.Logger.Infof("New torrent size %d", newSize)
							if newSize > 0 {
								action := cr.Messages.Added
								announce := true
								existSize, err := database.GetTorrentSize(torrent.Info.Name)
								if err != nil {
									intl.Logger.Error(err)
								}
								intl.Logger.Debugf("Existing torrent size: %d", existSize)
								if existSize > 0 {
									size0, size1 := existSize, newSize
									if size0 > size1 {
										size1, size0 = size0, size1
									}
									perc := uint((size0*100)/size1)
									if perc > cr.SizeThreshold {
										action = cr.Messages.Updated
									} else {
										intl.Logger.Debugf("Size diff is less than threshold: %d (%d%% diff)", newSize, perc)
										announce = false
									}
								}
								if announce {
									go cr.Announce(telegram, action, torrent)
									if currentOffset%1000 == 0 {
										go cr.N1000Get(telegram, currentOffset)
									}
								}
								if err := database.UpdateTorrent(torrent.Info.Name, newSize); err != nil {
									intl.Logger.Error(err)
								}
								baseOffset = currentOffset
								if err := database.UpdateCrawlOffset(baseOffset); err != nil {
									intl.Logger.Error(err)
								}
							} else {
								intl.Logger.Errorf("Zero torrent size, baseOffset: ", baseOffset)
							}
						} else {
							intl.Logger.Debugf("%s not a torrent: %v", fullUrl, err)
						}
					} else {
						var errMsg string
						errCount++
						if err != nil {
							errMsg = err.Error()
						} else if resp == nil {
							errMsg = "Empty response"
						} else {
							errMsg = resp.Status
						}
						intl.Logger.Warningf("Crawling error: %s", errMsg)
						if errCount%cr.Crawler.ErrorThreshold == 0 {
							go cr.UnaiwailNotify(telegram, errCount, errMsg)
						}
					}
				}
				sleepTime := time.Duration(rand.Intn(int(cr.Crawler.Delay))+int(cr.Crawler.Delay))
				intl.Logger.Debugf("Sleeping %d sec", sleepTime)
				time.Sleep(sleepTime * time.Second)
			}
		}
	}
	if err != nil {
		intl.Logger.Fatal(err)
	}
}

func (cr *Observer) Announce(tlg *intl.Telegram, action string, torrent *intl.Torrent) {
	intl.Logger.Debugf("Announcing %s", torrent.Info.Name)
}

func (cr *Observer) N1000Get(tlg *intl.Telegram, offset uint) {
	intl.Logger.Debugf("Notifying %d GET", offset)
}

func (cr *Observer) UnaiwailNotify(tlg *intl.Telegram, count uint, msg string) {
	intl.Logger.Noticef("Notifying about error: try %d, message %s", count, msg)
}
