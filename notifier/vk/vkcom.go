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
	"regexp"
	"sot-te.ch/TTObserverV1/notifier"
	s "sot-te.ch/TTObserverV1/shared"
	"strings"
	tmpl "text/template"
)

const (
	msgTags = "tags"
)

var logger = logging.MustGetLogger("vk")
var nonLetterNumberSpaceRegexp = regexp.MustCompile(`(?m)[^\p{L}\p{N}_\s]`)
var isEmptyRegexp = regexp.MustCompile("^$")
var allSpacesRegexp = regexp.MustCompile(`(?m)\s`)

func init() {
	notifier.RegisterNotifier("vkcom", &Notifier{})
}

type Notifier struct {
	//we need scopes: photos,wall,groups,offline
	Token           string `json:"token"`
	GroupIds        []uint `json:"groupids"`
	IgnoreUnchanged bool   `json:"ignoreunchanged"`
	IgnoreRegexp    string `json:"ignoreregexp"`
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
		Tags                map[string]bool `json:"tags"`
		TagsSeparator       string          `json:"tagsseparator"`
	} `json:"msg"`
	ignorePattern *regexp.Regexp
	db            *s.Database
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
				if len(n.IgnoreRegexp) == 0 {
					n.ignorePattern = isEmptyRegexp //is empty
				} else {
					n.ignorePattern, err = regexp.Compile(n.IgnoreRegexp)
				}
				if err == nil {
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
				}
			} else {
				err = errors.New("unable to initialize vk client, tokens are empty")
			}
		}
	}
	return n, err
}

func uploadImage(api *vkapi.API, photo []byte, groupId uint) (string, error) {
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

func (vk Notifier) buildHashTags(meta map[string]string) string {
	sb := strings.Builder{}
	if len(meta) > 0 && len(vk.Messages.Tags) > 0 {
		for tag, multivalued := range vk.Messages.Tags {
			m := meta[tag]
			if len(m) > 0 {
				if multivalued {
					for _, mPart := range strings.Split(m, vk.Messages.TagsSeparator) {
						mPart = strings.TrimSpace(mPart)
						if len(mPart) > 0 {
							mPart = nonLetterNumberSpaceRegexp.ReplaceAllString(mPart, "")
							mPart = allSpacesRegexp.ReplaceAllString(mPart, "_")
							if len(mPart) > 0 {
								sb.WriteRune('#')
								sb.WriteString(mPart)
								sb.WriteRune(' ')
							}
						}
					}
				} else {
					m = strings.TrimSpace(m)
					m = nonLetterNumberSpaceRegexp.ReplaceAllString(m, "")
					m = allSpacesRegexp.ReplaceAllString(m, "_")
					if len(m) > 0 {
						sb.WriteRune('#')
						sb.WriteString(m)
						sb.WriteRune(' ')
					}
				}
			}
		}
	}
	return sb.String()
}

func (vk Notifier) Notify(isNew bool, torrent s.TorrentInfo) {
	var err error
	if len(vk.Messages.Announce) > 0 {
		changedIndexes := notifier.GetNewFilesIndexes(torrent.Files)
		if (vk.IgnoreUnchanged && len(changedIndexes) > 0 || !vk.IgnoreUnchanged) && !vk.ignorePattern.MatchString(torrent.Name) {
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
					newIndexes, err := notifier.FormatIndexesMessage(changedIndexes, vk.Messages.singleIndexTmpl,
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
						msgTags:                vk.buildHashTags(torrent.Meta),
					}); err == nil {
						params := vkapi.WallPostParams{
							OwnerID:     -int(groupId),
							FromGroup:   true,
							Message:     msg,
							Attachments: photoAttachment,
						}
						var wallResp *vkapi.WallPostResp
						if wallResp, err = api.WallPost(params); err == nil {
							logger.Debugf("New post ID %d", wallResp.PostID)
						}
					}

					if err != nil {
						logger.Error(err)
					}
				}
			}
		}
	} else {
		logger.Warning("Announce message not set")
	}
}
func (vk Notifier) NxGet(offset uint) {
	if len(vk.Messages.Nx) > 0 {
		if api := vkapi.NewClient(vk.Token); api != nil {
			for _, groupId := range vk.GroupIds {
				logger.Debugf("Notifying %d GET", offset)
				if msg, err := notifier.FormatMessage(vk.Messages.nxTmpl, map[string]interface{}{
					notifier.MsgIndex: offset,
				}); err == nil {
					params := vkapi.WallPostParams{
						OwnerID:   -int(groupId),
						FromGroup: true,
						Message:   msg,
					}
					var wallResp *vkapi.WallPostResp
					if wallResp, err = api.WallPost(params); err == nil {
						logger.Debugf("New post ID %d", wallResp.PostID)
					}
				} else {
					logger.Error(err)
				}
			}
		}
	}
}

func (vk Notifier) Close() {}