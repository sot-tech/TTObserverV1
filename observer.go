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
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	actGo      = "go"
	actExtract = "extract"
	actCheck   = "check"
	actReturn  = "return"

	paramTorrent = "${torrent}"
	paramArg     = "${arg}"
)

type Observer struct {
	Log struct {
		File  string `json:"file"`
		Level string `json:"level"`
	} `json:"log"`
	Crawler struct {
		URL struct {
			Base               string `json:"base"`
			Torrent            string `json:"torrent"`
			ExtractNameActions []struct {
				Action string `json:"action"`
				Param  string `json:"param"`
			} `json:"extractnameactions"`
		} `json:"url"`
		Threshold      uint `json:"threshold"`
		ErrorThreshold uint `json:"errorthreshold"`
		Delay          uint `json:"delay"`
	} `json:"crawler"`
	TryExtractPrettyName bool          `json:"tryextractprettyname"`
	SizeThreshold        float64       `json:"sizethreshold"`
	TelegramToken        string        `json:"telegramtoken"`
	AdminOTPSeed         string        `json:"adminotpseed"`
	DBFile               string        `json:"dbfile"`
	Messages             intl.Messages `json:"msg"`
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
		DB:       database,
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
				wasError := false
				for i = 0; i < cr.Crawler.Threshold; i++ {
					currentOffset := offset + i
					intl.Logger.Debugf("Checking offset %d", currentOffset)
					torrentContext := fmt.Sprintf(cr.Crawler.URL.Torrent, currentOffset)
					fullUrl := cr.Crawler.URL.Base + torrentContext
					if torrent, err := intl.GetTorrent(fullUrl); err == nil {
						wasError = false
						errCount = 0
						if torrent != nil {
							intl.Logger.Infof("New file %s", torrent.Info.Name)
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
									size0, size1 := float64(existSize), float64(newSize)
									diff := 100 - math.Abs(size0*100.0/size1)
									if diff > cr.SizeThreshold {
										isNew = false
									} else {
										intl.Logger.Infof("Size diff is less than threshold: %d (%.1f%% diff)", newSize, diff)
										announce = false
									}
								}
								if announce {
									if cr.TryExtractPrettyName {
										if prettyName, nameUrl := cr.ExtractPrettyName(torrentContext); prettyName != "" {
											torrent.Info.Name = fmt.Sprintf("%s (%s)", prettyName, torrent.Info.Name)
											torrent.PublisherUrl = nameUrl
										}
									}
									if isNew {
										go telegram.AnnounceNew(torrent)
									} else {
										go telegram.AnnounceUpdate(torrent)
									}
									if currentOffset%1000 == 0 {
										go telegram.N1000Get(currentOffset)
									}
								}
								if err := database.UpdateTorrent(torrent.Info.Name, newSize); err != nil {
									intl.Logger.Error(err)
								}
								baseOffset = currentOffset + 1
								if err := database.UpdateCrawlOffset(baseOffset); err != nil {
									intl.Logger.Error(err)
								}
							} else {
								intl.Logger.Errorf("Zero torrent size, offset %d", currentOffset)
							}
						} else {
							intl.Logger.Noticef("%s not a torrent", fullUrl)
						}
					} else {
						wasError = true
						intl.Logger.Errorf("Crawling error: %v", err)
						if errCount > 0 && errCount%cr.Crawler.ErrorThreshold == 0 {
							go telegram.UnaiwailNotify(errCount, err.Error())
						}
						break
					}
				}
				if wasError {
					errCount++
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

func (cr *Observer) ExtractPrettyName(context string) (string, string) {
	var res, lastUrl string
	funcs := list.New()
	var f *list.Element
	stop := false
	for actIndex := len(cr.Crawler.URL.ExtractNameActions) - 1; actIndex >= 0; actIndex-- {
		action := cr.Crawler.URL.ExtractNameActions[actIndex]
		var currentFunc func(string)
		switch action.Action {
		case actGo:
			currentFunc = func(param string) {
				var nextFunc func(string)
				if f != nil {
					f = f.Next()
					if f != nil {
						nextFunc = f.Value.(func(string))
					}
				}
				url := strings.Replace(action.Param, paramArg, param, -1)
				url = strings.Replace(url, paramTorrent, context, -1)
				if strings.Index(param, cr.Crawler.URL.Base) < 0 {
					url = cr.Crawler.URL.Base + url
				}
				if resp, err := http.Get(url); err == nil && resp != nil && resp.StatusCode < 400 {
					lastUrl = url
					if bytes, err := ioutil.ReadAll(resp.Body); err == nil {
						if nextFunc != nil {
							nextFunc(string(bytes))
						}
					} else {
						intl.Logger.Warningf("Read body error: %v", err)
					}
				} else {
					var errMsg string
					if err != nil {
						errMsg = err.Error()
					} else if resp == nil {
						errMsg = "empty response"
					} else {
						errMsg = resp.Status
					}
					err = errors.New(errMsg)
					intl.Logger.Warningf("HTTP error: %s", errMsg)
				}
			}
		case actExtract:
			currentFunc = func(param string) {
				var nextFunc func(string)
				if f != nil {
					f = f.Next()
					if f != nil {
						nextFunc = f.Value.(func(string))
					}
				}
				pattern := strings.Replace(action.Param, paramArg, param, -1)
				pattern = strings.Replace(pattern, paramTorrent, context, -1)
				if reg, err := regexp.Compile("(?s)" + pattern); err == nil {
					matches := reg.FindAllStringSubmatch(param, -1)
					if matches != nil {
						for _, match := range matches {
							if match != nil && len(match) > 1 {
								if nextFunc != nil {
									tmpF := f
									nextFunc(match[1])
									f = tmpF
								}
							}
							if stop {
								break
							}
						}
					}
				} else {
					intl.Logger.Warning(err)
				}
			}
		case actCheck:
			currentFunc = func(param string) {
				var nextFunc func(string)
				if f != nil {
					f = f.Next()
					if f != nil {
						nextFunc = f.Value.(func(string))
					}
				}
				pattern := strings.Replace(action.Param, paramArg, param, -1)
				pattern = strings.Replace(pattern, paramTorrent, context, -1)
				if reg, err := regexp.Compile("(?s)" + pattern); err == nil {
					if matches := reg.FindStringSubmatch(param); matches != nil && len(matches) > 0 {
						if nextFunc != nil {
							nextFunc(param)
						}
					}
				} else {
					intl.Logger.Warning(err)
				}
			}
		case actReturn:
			currentFunc = func(param string) {
				res = param
				stop = true
			}
		}
		funcs.PushFront(currentFunc)
	}
	if funcs.Len() > 0 {
		f = funcs.Front()
		f.Value.(func(string))("")
	}
	return html.UnescapeString(res), lastUrl
}
