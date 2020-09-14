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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nfnt/resize"
	"github.com/op/go-logging"
	"html"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sot-te.ch/HTExtractor"
	"sot-te.ch/TTObserverV1/notifier"
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
		Delay          uint                        `json:"delay"`
		Threshold      uint                        `json:"threshold"`
		Anniversary    uint                        `json:"anniversary"`
		MetaActions    []HTExtractor.ExtractAction `json:"metaactions"`
		ImageMetaField string                      `json:"imagemetafield"`
		ImageThumb     uint                        `json:"imagethumb"`
		metaExtractor  *HTExtractor.Extractor
	} `json:"crawler"`
	Announcer notifier.Announcer `json:"announcers"`
	DBFile string `json:"dbfile"`
	db     *Database
}

var logger = logging.MustGetLogger("observer")

func ReadConfig(path string) (*Observer, error) {
	var config Observer
	confData, err := ioutil.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(confData, &config)
	}
	return &config, err
}

func (cr *Observer) Init() error {
	db := &Database{}
	if err := db.Connect(cr.DBFile); err != nil {
		return err
	}
	cr.db = db
	logger.Debug("Initiating meta extractor")
	if len(cr.Crawler.MetaActions) > 0 {
		ex := HTExtractor.New()
		if err := ex.Compile(cr.Crawler.MetaActions); err == nil {
			cr.Crawler.metaExtractor = ex
		} else {
			return err
		}
	} else {
		return errors.New("extract actions not set")
	}
	var err error
	if len(cr.Announcer.Notifiers) > 0{
		err = cr.Announcer.Init(cr, db)
	} else {
		err = errors.New("notifiers not set")
	}
	return err
}

func (cr *Observer) Engage() {
	defer cr.db.Close()
	defer cr.Announcer.Close()
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

func (cr *Observer) CheckTorrent(offset uint) bool {
	var res bool
	logger.Debug("Checking offset ", offset)
	fullContext := fmt.Sprintf(cr.Crawler.ContextURL, offset)
	if torrent, err := GetTorrent(cr.Crawler.BaseURL + fullContext); err == nil {
		if torrent != nil {
			logger.Info("New file", torrent.Name)
			newSize := torrent.Length
			logger.Info("New torrent size", newSize)
			if newSize > 0 {
				var torrentId int64
				var isNew bool
				if torrentId, err = cr.db.GetTorrent(torrent.Name); err == nil {
					var existFiles []DBTorrentFile
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
				isNew = torrentId == invalidId
				if torrentId, err = cr.db.AddTorrent(torrent.Name, torrent.NewFiles()); err != nil {
					logger.Error(err)
				}
				torrent.Id = torrentId
				cr.Notify(*torrent, fullContext, isNew)
				if offset > 0 && offset%cr.Crawler.Anniversary == 0 {
					cr.Announcer.NxGet(offset)
				}

				res = true
			} else {
				logger.Error("Zero torrent size, offset", offset)
			}
		} else {
			logger.Debugf("%s not a torrent", fullContext)
		}
	}
	return res
}

func (cr *Observer) Notify(torrent TorrentInfo, context string, isNew bool) {
	var err error
	var upstreamMeta, existingMeta map[string]string
	var torrentImageUrl string
	if cr.Crawler.metaExtractor != nil {
		var rawMeta map[string][]byte
		if rawMeta, err = cr.Crawler.metaExtractor.ExtractData(cr.Crawler.BaseURL, context);
			err == nil && len(rawMeta) > 0 {
			upstreamMeta = make(map[string]string, len(rawMeta))
			for k, v := range rawMeta {
				if len(k) > 0 {
					s := strings.TrimSpace(html.UnescapeString(string(v)))
					upstreamMeta[k] = s
					if k == cr.Crawler.ImageMetaField {
						torrentImageUrl = s
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
		if err = cr.db.AddTorrentMeta(torrent.Id, upstreamMeta); err != nil {
			logger.Error(err)
		}
	} else {
		upstreamMeta = existingMeta
	}
	var torrentImage []byte
	if torrentImage, err = cr.db.GetTorrentImage(torrent.Id); err == nil {
		if len(torrentImage) == 0 || existingMeta[cr.Crawler.ImageMetaField] != torrentImageUrl {
			err, torrentImage = cr.UpdateImage(torrent.Id, torrentImageUrl)
		}
	}
	if err != nil {
		logger.Error(err)
	}
	torrent.Meta = upstreamMeta
	torrent.Image = torrentImage
	torrent.URL = cr.Crawler.BaseURL + context
	cr.Announcer.Notify(isNew, torrent)
}

func (cr *Observer) UpdateImage(torrentId int64, imageUrl string) (error, []byte) {
	var err error
	var torrentImage []byte
	if len(imageUrl) > 0 {
		if strings.Index(imageUrl, cr.Crawler.BaseURL) < 0 {
			imageUrl = cr.Crawler.BaseURL + imageUrl
		}
		if resp, httpErr := http.Get(imageUrl); httpErr == nil && resp != nil && resp.StatusCode < 400 {
			defer resp.Body.Close()
			var img image.Image
			if img, _, err = image.Decode(resp.Body); err == nil && img != nil {
				if cr.Crawler.ImageThumb > 0 {
					img = resize.Thumbnail(cr.Crawler.ImageThumb, cr.Crawler.ImageThumb, img, resize.Bicubic)
				}
				imgBuffer := bytes.Buffer{}
				if err = jpeg.Encode(&imgBuffer, img, nil); err == nil {
					if torrentImage, err = ioutil.ReadAll(&imgBuffer); err == nil {
						err = cr.db.AddTorrentImage(torrentId, torrentImage)
					}
				}
			}
		} else {
			errMsg := "crawling: "
			if httpErr != nil {
				errMsg += httpErr.Error()
			} else if resp == nil {
				errMsg += "empty response"
			} else {
				errMsg += resp.Status
			}
			err = errors.New(errMsg)
		}
	} else {
		err = errors.New("invalid image url")
	}
	return err, torrentImage
}
