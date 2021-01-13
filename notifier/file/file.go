/*
 * BSD-3-Clause
 * Copyright 2021 sot (PR_713, C_rho_272)
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

package file

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/op/go-logging"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"sot-te.ch/TTObserverV1/notifier"
	s "sot-te.ch/TTObserverV1/shared"
	"strconv"
	tmpl "text/template"
)

var logger = logging.MustGetLogger("file")

func init() {
	notifier.RegisterNotifier("file", &Notifier{})
}

type Notifier struct {
	NameTemplate string `json:"nametemplate"`
	Permissions  string `json:"permissions"`
	nameTemplate *tmpl.Template
	perm         uint64
}

func (st Notifier) New(configPath string, _ *s.Database) (notifier.Notifier, error) {
	var err error
	n := Notifier{}
	var confBytes []byte
	if confBytes, err = ioutil.ReadFile(filepath.Clean(configPath)); err == nil {
		if err = json.Unmarshal(confBytes, &n); err == nil {
			var stat os.FileInfo
			if stat, err = os.Stat(filepath.Dir(n.NameTemplate)); err == nil {
				if stat.IsDir() {
					if n.nameTemplate, err = tmpl.New(fmt.Sprint("file_", rand.Uint32())).Parse(n.NameTemplate); err == nil{
						if len(n.Permissions) == 0 {
							logger.Warning("Permissions parameter not set, falling to 0644")
							n.perm = 0644
						} else {
							n.perm, err = strconv.ParseUint(n.Permissions, 8, 32)
						}
					}
				} else {
					err = errors.New("invalid path")
				}
			}
		}
	}
	return n, err
}

func (st Notifier) Notify(_ bool, torrent s.TorrentInfo) {
	var err error
	var fileName string
	if fileName, err = notifier.FormatMessage(st.nameTemplate, map[string]interface{}{
		notifier.MsgName: torrent.Name,
		notifier.MsgIndex: torrent.Id,
	}); err == nil{
		if fileName = filepath.Clean(fileName); len(fileName) > 0{
			err = ioutil.WriteFile(fileName, torrent.Data, os.FileMode(st.perm))
		} else{
			err = errors.New("filename is empty")
		}
	}
	if err != nil{
		logger.Error(err)
	}
}

func (st Notifier) Close() {}

func (st Notifier) NxGet(uint) {}
