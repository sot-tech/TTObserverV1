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

package notifier

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/op/go-logging"
	"sort"
	tto "sot-te.ch/TTObserverV1"
	"sync"
	"text/template"
)

var logger = logging.MustGetLogger("notifier")

type Notifier interface {
	New(string, *tto.Observer, *tto.Database) (Notifier, error)
	Notify(bool, tto.TorrentInfo)
	NxGet(uint)
	Close()
}

type Config struct {
	Type       string `json:"type"`
	ConfigPath string `json:"configpath"`
}

var notifi = make(map[string]Notifier)
var notifiMu sync.Mutex

func RegisterNotifier(name string, n Notifier) {
	notifiMu.Lock()
	defer notifiMu.Unlock()
	if len(name) == 0 {
		panic("unspecified notifier name")
	} else if n == nil {
		panic("unspecified notifier ref instance")
	} else {
		notifi[name] = n
	}
}

func FormatMessage(tmpl *template.Template, values map[string]interface{}) (string, error) {
	var err error
	var res string
	if tmpl != nil {
		buf := bytes.Buffer{}
		if err = tmpl.Execute(&buf, values); err == nil {
			res = buf.String()
		}
	} else {
		err = errors.New("template not initiated")
	}
	return res, err
}

func FormatFileSize(size uint64) string {
	const base = 1024
	const suff = "KMGTPEZY"
	var res string
	if size < base {
		res = fmt.Sprintf("%d B", size)
	} else {
		d, e := uint64(base), 0
		for n := size / base; n >= base; n /= base {
			d *= base
			e++
		}
		s := '?'
		if e < len(suff) {
			s = rune(suff[e])
		}
		res = fmt.Sprintf("%.2f %ciB", float64(size)/float64(d), s)
	}
	return res
}

func GetNewFilesIndexes(files map[string]bool) []int {
	indexes := make([]int, 0, len(files))
	if len(files) > 0 {
		paths := make([]string, 0, len(files))
		for k, _ := range files {
			paths = append(paths, k)
		}
		sort.Strings(paths)
		for i, path := range paths{
			if files[path] {
				indexes = append(indexes, i + 1)
			}
		}
	}
	return indexes
}

type Announcer struct {
	Notifiers []Config
	notifiers []Notifier
}

func (a *Announcer) Init(observer *tto.Observer, db *tto.Database) error {
	var err error
	a.notifiers = make([]Notifier, 0)
	if len(a.Notifiers) > 0 {
		for i, n := range a.Notifiers {
			if ni := notifi[n.Type]; ni != nil {
				var nn Notifier
				if nn, err = ni.New(n.ConfigPath, observer, db); err == nil {
					if nn != nil {
						a.notifiers = append(a.notifiers, nn)
					} else {
						err = errors.New(fmt.Sprint("unable to construct notifier #", i))
					}
				}
			} else {
				err = errors.New(fmt.Sprint("notifier #", i, " unknown type: ", n.Type))
			}
			if err != nil {
				break
			}
		}
	}
	return err
}

func (a Announcer) Notify(new bool, torrent tto.TorrentInfo) {
	for _, n := range a.notifiers {
		n.Notify(new, torrent)
	}
}

func (a Announcer) NxGet(offset uint) {
	for _, n := range a.notifiers {
		n.NxGet(offset)
	}
}

func (a *Announcer) Close() {
	for _, n := range a.notifiers {
		n.Close()
	}
}
