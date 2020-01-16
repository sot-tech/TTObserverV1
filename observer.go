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

package TTObserver

import (
	"./intl"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
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
	SizeThreshold uint          `json:"sizethreshold"`
	TelegramToken string        `json:"telegramtoken"`
	AdminOTPSeed  string        `json:"adminotpseed"`
	DBFile        string        `json:"dbfile"`
	Messages      intl.Messages `json:"msg"`
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
		Messages: cr.Messages,
		DB: database,
	}
	err := database.Connect(cr.DBFile)
	if err == nil {
		defer database.Close()
		err = telegram.Connect(cr.TelegramToken, cr.AdminOTPSeed, -1)
		if err == nil {
			go telegram.HandleUpdates()
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
					if torrent, err := intl.GetTorrent(fullUrl); err == nil {
						errCount = 0
						if torrent != nil {
							intl.Logger.Infof("New file: %s", torrent.Info.Name)
							torrent.URL = fullUrl
							newSize := torrent.FullSize()
							intl.Logger.Infof("New torrent size %d", newSize)
							if newSize > 0 {
								isNew := true
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
									diff := uint((size0 * 100) / size1)
									if diff > cr.SizeThreshold {
										isNew = false
									} else {
										intl.Logger.Debugf("Size diff is less than threshold: %d (%d%% diff)", newSize, diff)
										announce = false
									}
								}
								if announce {
									if isNew{
										go telegram.AnnounceNew(torrent)
									} else{
										go telegram.AnnounceUpdate(torrent)
									}
									if currentOffset%1000 == 0 {
										go telegram.N1000Get(currentOffset)
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
							intl.Logger.Debugf("%s not a torrent", fullUrl)
						}
					} else {
						errCount++
						intl.Logger.Warningf("Crawling error: %v", err)
						if errCount%cr.Crawler.ErrorThreshold == 0 {
							go telegram.UnaiwailNotify(errCount, err.Error())
						}
					}
				}
				sleepTime := time.Duration(rand.Intn(int(cr.Crawler.Delay)) + int(cr.Crawler.Delay))
				intl.Logger.Debugf("Sleeping %d sec", sleepTime)
				time.Sleep(sleepTime * time.Second)
			}
		}
	}
	if err != nil {
		intl.Logger.Fatal(err)
	}
}

