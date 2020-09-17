/*
 * BSD-3-Clause
 * Copyright 2020 sot (PR_713, C_rho_272)
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

package vk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/azzzak/vkapi"
	"github.com/op/go-logging"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sot-te.ch/TTObserverV1/notifier"
	s "sot-te.ch/TTObserverV1/shared"
	"strings"
	tmpl "text/template"
)

var logger = logging.MustGetLogger("vk")

func init() {
	notifier.RegisterNotifier("vkcom", &Notifier{})
}

type Notifier struct {
	Token           string `json:"token"`
	GroupIds        []uint `json:"groupids"`
	IgnoreUnchanged bool   `json:"ignoreunchanged"`
	Messages        struct {
		Announce            string `json:"announce"`
		announceTmpl        *tmpl.Template
		Nx                  string `json:"n1x"`
		nxTmpl              *tmpl.Template
		Replacements        map[string]string `json:"replacements"`
		Added               string            `json:"added"`
		Updated             string            `json:"updated"`
		SingleIndex         string            `json:"singleindex"`
		singleIndexTmpl     *tmpl.Template
		MultipleIndexes     string `json:"multipleindexes"`
		multipleIndexesTmpl *tmpl.Template
	} `json:"msg"`
	db *s.Database
}

func (vk Notifier) New(configPath string, db *s.Database) (notifier.Notifier, error) {
	var err error
	n := Notifier{
		db: db,
	}
	var confBytes []byte
	if confBytes, err = ioutil.ReadFile(configPath); err == nil {
		if err = json.Unmarshal(confBytes, &n); err == nil {
			if len(n.Token) > 0 {
				var subErr error
				if n.Messages.announceTmpl, subErr = tmpl.New("announce").Parse(n.Messages.Announce); subErr != nil {
					logger.Error(subErr)
				}
				if n.Messages.nxTmpl, subErr = tmpl.New("n1000").Parse(n.Messages.Nx); subErr != nil {
					logger.Error(subErr)
				}
				if n.Messages.singleIndexTmpl, subErr = tmpl.New("singleIndex").Parse(n.Messages.SingleIndex); subErr != nil {
					logger.Error(subErr)
				}
				if n.Messages.multipleIndexesTmpl, subErr = tmpl.New("multipleIndexes").Parse(n.Messages.MultipleIndexes); subErr != nil {
					logger.Error(subErr)
				}
			} else {
				err = errors.New("unable to initialize vk client, tokens are empty")
			}
		}
	}
	return n, err
}

func uploadImage(api *vkapi.API, photo []byte, groupId uint) (string, error){
	var err error
	var photoAttachment string
	var uploadServerResult *vkapi.PhotosGetWallUploadServerResp
	if uploadServerResult, err = api.PhotosGetWallUploadServer(vkapi.PhotosGetWallUploadServerParams{GroupID: int(groupId)}); err == nil {
		var uploadPhotoResult *vkapi.UploadWallResp
		var photoName string
		if exts := strings.Split(http.DetectContentType(photo), "/"); len(exts) > 1 {
			photoName = fmt.Sprintf("%d.%s", rand.Int(), exts[1])
		}
		bb := bytes.Buffer{}
		bb.Write(photo)
		if uploadPhotoResult, err = vkapi.UploadWall(uploadServerResult.UploadURL, photoName, &bb); err == nil {
			var photos []vkapi.Photo
			if photos, err = api.PhotosSaveWallPhoto(vkapi.PhotosSaveWallPhotoParams{
				GroupID: groupId,
				Photo:   uploadPhotoResult.Photo,
				Server:  uploadPhotoResult.Server,
				Hash:    uploadPhotoResult.Hash,
			}); err == nil {
				if len(photos) > 0 {
					photoAttachment = vkapi.MakeAttachment("photo", photos[0].OwnerID, photos[0].ID)
				} else {
					err = errors.New("unable to get uploaded photo info")
				}
			}
		}
	}
	return photoAttachment, err
}

func (vk Notifier) Notify(isNew bool, torrent s.TorrentInfo) {
	var err error
	if len(vk.Messages.Announce) > 0 {
		if api := vkapi.NewClient(vk.Token); api != nil {
			for _, groupId := range vk.GroupIds {
				var photoAttachment string
				if len(torrent.Image) > 0 {
					photoAttachment, err = uploadImage(api, torrent.Image, groupId)
				}
				if err != nil {
					logger.Error(err)
				}
				action := vk.Messages.Updated
				if isNew {
					action = vk.Messages.Added
				}
				logger.Debugf("Announcing %s for %s", action, torrent.Name)
				name := torrent.Name
				if len(vk.Messages.Replacements) > 0 {
					for k, v := range vk.Messages.Replacements {
						name = strings.Replace(name, k, v, -1)
					}
				}
				newIndexes, err := notifier.FormatIndexesMessage(torrent.Files, vk.Messages.singleIndexTmpl,
					vk.Messages.multipleIndexesTmpl, notifier.MsgNewIndexes)
				if err != nil {
					logger.Error(err)
				}
				if msg, err := notifier.FormatMessage(vk.Messages.announceTmpl, map[string]interface{}{
					notifier.MsgAction:     action,
					notifier.MsgName:       name,
					notifier.MsgSize:       notifier.FormatFileSize(torrent.Length),
					notifier.MsgUrl:        torrent.URL,
					notifier.MsgFileCount:  len(torrent.Files),
					notifier.MsgMeta:       torrent.Meta,
					notifier.MsgNewIndexes: newIndexes,
				}); err == nil {
					params := vkapi.WallPostParams{
						OwnerID:     -int(groupId),
						FromGroup:   true,
						Message:     msg,
						Attachments: photoAttachment,
					}
					var wallResp *vkapi.WallPostResp
					if wallResp, err = api.WallPost(params); err == nil {
						logger.Debug(wallResp.PostID)
					}
				}

				if err != nil {
					logger.Error(err)
				}
			}
		}
	} else {
		logger.Warning("Announce message not set")
	}
}
func (vk Notifier) NxGet(_ uint) {}

func (vk Notifier) Close() {}
