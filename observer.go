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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/op/go-logging"
	"html"
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"sot-te.ch/HTExtractor"
	"sot-te.ch/TTObserverV1/producer"
	_ "sot-te.ch/TTObserverV1/producer/file"
	_ "sot-te.ch/TTObserverV1/producer/nats"
	_ "sot-te.ch/TTObserverV1/producer/stan"
	_ "sot-te.ch/TTObserverV1/producer/tg"
	_ "sot-te.ch/TTObserverV1/producer/vk"
	s "sot-te.ch/TTObserverV1/shared"
	"strings"
	"time"
)

type Observer struct {
	Log struct {
		File  string `json:"file"`
		Level string `json:"level"`
	} `json:"log"`
	Crawler struct {
		BaseURL        string                      `json:"baseurl"`
		ContextURL     string                      `json:"contexturl"`
		Limit          uint64                      `json:"limit"`
		Delay          uint                        `json:"delay"`
		Threshold      uint                        `json:"threshold"`
		Anniversary    uint                        `json:"anniversary"`
		MetaActions    []HTExtractor.ExtractAction `json:"metaactions"`
		MetaRetry      uint                        `json:"metaretry"`
		ImageMetaField string                      `json:"imagemetafield"`
		ImageThumb     uint                        `json:"imagethumb"`
		metaExtractor  *HTExtractor.Extractor
	} `json:"crawler"`
	Producers []producer.Config `json:"producers"`
	DBFile    string            `json:"dbfile"`
	db        *s.Database
	producer  *producer.Announcer
}

var logger = logging.MustGetLogger("observer")

func ReadConfig(path string) (*Observer, error) {
	var config Observer
	confData, err := ioutil.ReadFile(filepath.Clean(path))
	if err == nil {
		err = json.Unmarshal(confData, &config)
	}
	return &config, err
}

func (cr *Observer) Init() error {
	var err error
	if cr.db, err = s.ConnectDB(cr.DBFile); err != nil {
		return err
	}
	logger.Debug("Initiating meta extractor")
	if len(cr.Crawler.MetaActions) > 0 {
		ex := HTExtractor.New()
		if err := ex.Compile(cr.Crawler.MetaActions); err == nil {
			ex.IterationLimit, ex.StackLimit = cr.Crawler.Limit, cr.Crawler.Limit
			cr.Crawler.metaExtractor = ex
		} else {
			return err
		}
	} else {
		return errors.New("extract actions not set")
	}
	logger.Debug("Initiating notifiers")
	cr.producer, err = producer.New(cr.Producers, cr.db)
	return err
}

func (cr Observer) Engage() {
	var err error
	var nextOffset uint
	if nextOffset, err = cr.db.GetCrawlOffset(); err == nil {
		for {
			newNextOffset := nextOffset
			for offsetToCheck := nextOffset; offsetToCheck < nextOffset+cr.Crawler.Threshold; offsetToCheck++ {
				if cr.CheckTorrent(offsetToCheck) {
					newNextOffset = offsetToCheck + 1
				}
			}
			if newNextOffset > nextOffset {
				nextOffset = newNextOffset
				if err = cr.db.UpdateCrawlOffset(nextOffset); err != nil {
					logger.Error(err)
				}
			}
			sleepTime := time.Duration(rand.Intn(int(cr.Crawler.Delay)) + int(cr.Crawler.Delay))
			logger.Debugf("Sleeping %d sec", sleepTime)
			time.Sleep(sleepTime * time.Second)
		}
	} else {
		logger.Fatal(err)
	}
}

func (cr *Observer) Close() {
	cr.producer.Close()
	cr.db.Close()
}

func (cr Observer) CheckTorrent(offset uint) bool {
	var res bool
	logger.Debug("Checking offset ", offset)
	fullContext := fmt.Sprintf(cr.Crawler.ContextURL, offset)
	if torrent, err := s.GetTorrent(cr.Crawler.BaseURL + fullContext); err == nil {
		if torrent != nil {
			logger.Info("New file", torrent.Name)
			newSize := torrent.Length
			logger.Info("New torrent size", newSize)
			if newSize > 0 {
				var torrentId int64
				var isNew bool
				if torrentId, err = cr.db.GetTorrent(torrent.Name); err == nil {
					var existFiles []s.DBTorrentFile
					if existFiles, err = cr.db.GetTorrentFiles(torrentId); err == nil {
						if len(existFiles) > 0 {
							for _, file := range existFiles {
								if _, ok := torrent.Files[file.Name]; ok {
									torrent.Files[file.Name] = false
								}
							}
						}
					} else {
						logger.Error(err)
					}
				} else {
					logger.Error(err)
				}
				isNew = torrentId == s.InvalidDBId
				if torrentId, err = cr.db.AddTorrent(torrent.Name, torrent.Data, torrent.NewFiles()); err != nil {
					logger.Error(err)
				}
				torrent.Id = torrentId
				cr.Notify(torrent, fullContext, isNew)
				if offset > 0 && offset%cr.Crawler.Anniversary == 0 {
					cr.producer.SendNxGet(offset)
				}
				res = true
			} else {
				logger.Error("Zero torrent size, offset", offset)
			}
		}
	}
	return res
}

func (cr Observer) Notify(torrent *s.TorrentInfo, context string, isNew bool) {
	if torrent != nil {
		var err error
		var upstreamMeta, existingMeta map[string]string
		var torrentImageUrl string
		if cr.Crawler.metaExtractor != nil {
			var rawMeta map[string][]byte
			logger.Debug("Extracting meta for torrent ", torrent.Name)
			rawMeta, err = cr.Crawler.metaExtractor.ExtractData(cr.Crawler.BaseURL, context)
			if err != nil || len(rawMeta) == 0 {
				logger.Error("Meta fetch error: ", err, " got meta len ", len(rawMeta))
				if cr.Crawler.MetaRetry > 0 {
					time.Sleep(time.Duration(cr.Crawler.MetaRetry) * time.Second)
					rawMeta, err = cr.Crawler.metaExtractor.ExtractData(cr.Crawler.BaseURL, context)
				}
			}
			if err == nil && len(rawMeta) > 0 {
				upstreamMeta = make(map[string]string, len(rawMeta))
				for k, v := range rawMeta {
					if len(k) > 0 {
						unescapedValue := strings.TrimSpace(html.UnescapeString(string(v)))
						upstreamMeta[k] = unescapedValue
						if k == cr.Crawler.ImageMetaField {
							torrentImageUrl = unescapedValue
						}
					}
				}
			}
		}
		if err != nil {
			logger.Error(err)
		}
		if existingMeta, err = cr.db.GetTorrentMeta(torrent.Id); err != nil {
			logger.Error(err)
			existingMeta = make(map[string]string)
		}
		if len(upstreamMeta) > 0 {
			logger.Debug("Updating meta")
			if err = cr.db.AddTorrentMeta(torrent.Id, upstreamMeta); err != nil {
				logger.Error(err)
			}
		} else {
			logger.Warning("Upstream meta is empty, using cached")
			upstreamMeta = existingMeta
		}
		var torrentImage []byte
		if torrentImage, err = cr.db.GetTorrentImage(torrent.Id); err == nil {
			if len(torrentImage) == 0 || existingMeta[cr.Crawler.ImageMetaField] != torrentImageUrl {
				if len(torrentImageUrl) > 0 {
					logger.Info("Reloading torrent image")
					if strings.Index(torrentImageUrl, cr.Crawler.BaseURL) < 0 {
						torrentImageUrl = cr.Crawler.BaseURL + torrentImageUrl
					}
					if err, torrentImage = s.GetTorrentPoster(torrentImageUrl, cr.Crawler.ImageThumb); err == nil {
						err = cr.db.AddTorrentImage(torrent.Id, torrentImage)
					}
				}
			}
		}
		if err != nil {
			logger.Error(err)
		}
		torrent.Meta = upstreamMeta
		torrent.Image = torrentImage
		torrent.URL = cr.Crawler.BaseURL + context
		cr.producer.Send(isNew, torrent)
	}
}
