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

package stan

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"github.com/nats-io/stan.go"
	"github.com/op/go-logging"
	"io/ioutil"
	"path/filepath"
	"sot-te.ch/TTObserverV1/notifier"
	s "sot-te.ch/TTObserverV1/shared"
	"sync/atomic"
)

var logger = logging.MustGetLogger("stan")

func init() {
	notifier.RegisterNotifier("stan", &Notifier{})
}

type Notifier struct {
	URL           string `json:"url"`
	ClusterId     string `json:"clusterid"`
	ClientId      string `json:"clientid"`
	Subject       string `json:"subject"`
	db            *s.Database
	clientStorage *atomic.Value
}

func (st Notifier) client() stan.Conn {
	return st.clientStorage.Load().(stan.Conn)
}

func (st *Notifier) reconnect(prevConn stan.Conn, cause error) {
	logger.Error("STAN Connection lost ", cause)
	if err := prevConn.Close(); err != nil {
		logger.Warning("Unable to close previous connection ", err)
	}
	if conn, err := stan.Connect(st.ClusterId, st.ClientId, stan.NatsURL(st.URL)); err == nil {
		st.clientStorage.Store(conn)
	} else {
		logger.Panic("Unable to reconnect ", err)
	}
}

func (st Notifier) New(configPath string, db *s.Database) (notifier.Notifier, error) {
	var err error
	n := Notifier{
		db: db,
	}
	var confBytes []byte
	if confBytes, err = ioutil.ReadFile(filepath.Clean(configPath)); err == nil {
		if err = json.Unmarshal(confBytes, &n); err == nil {
			if len(n.ClusterId) > 0 && len(n.ClientId) > 0 && len(n.URL) > 0 {
				var conn stan.Conn
				if conn, err = stan.Connect(n.ClusterId, n.ClientId, stan.NatsURL(n.URL),
					stan.SetConnectionLostHandler(n.reconnect)); err == nil {
					n.clientStorage = &atomic.Value{}
					n.clientStorage.Store(conn)
				} else {
					logger.Error("Unable to connect ", err)
				}
			} else {
				err = errors.New("required parameters not set")
			}
		}
	}
	return n, err
}

func (st Notifier) Notify(_ bool, torrent s.TorrentInfo) {
	var err error
	bb := bytes.Buffer{}
	enc := gob.NewEncoder(&bb)
	if err = enc.Encode(torrent); err == nil {
		var id string
		if id, err = st.client().PublishAsync(st.Subject, bb.Bytes(), func(id string, err error) {
			if err == nil{
				logger.Debugf("Transmission of torrent %d, finished: %s\n", torrent.Id, id)
			} else{
				logger.Errorf("Transmission of torrent %d, FAILED: %v\n", torrent.Id, err)
			}
		}); err == nil{
			logger.Debugf("Torrent %d has been sent to server, id: %s\n", torrent.Id, id)
		}
	}
	if err != nil {
		logger.Error(err)
	}
}

func (st Notifier) Close() {
	if err := st.client().Close(); err != nil {
		logger.Error(err)
	}
}

func (st Notifier) NxGet(uint) {}
