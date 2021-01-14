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
	"sot-te.ch/TTObserverV1/producer"
	s "sot-te.ch/TTObserverV1/shared"
	"sync"
	"sync/atomic"
	"time"
)

var logger = logging.MustGetLogger("stan")

func init() {
	producer.RegisterProducer("stan", &Notifier{})
}

type Notifier struct {
	URL           string `json:"url"`
	ClusterId     string `json:"clusterid"`
	ClientId      string `json:"clientid"`
	Subject       string `json:"subject"`
	PingSleep     int    `json:"pingsleep"`
	PingMax       int    `json:"pingmax"`
	db            *s.Database
	clientStorage *atomic.Value
	clientMu      *sync.Mutex
	clientOpts    []stan.Option
}

func (st Notifier) client() stan.Conn {
	st.clientMu.Lock()
	defer st.clientMu.Unlock()
	return st.clientStorage.Load().(stan.Conn)
}

func (st *Notifier) reconnect(_ stan.Conn, cause error) {
	st.clientMu.Lock()
	defer st.clientMu.Unlock()
	err := cause
	var conn stan.Conn
	logger.Error("STAN Connection lost ", err)
	for i := 0; i < stan.DefaultPingMaxOut && err != nil; i++ {
		time.Sleep(stan.DefaultConnectWait)
		if conn, err = stan.Connect(st.ClusterId, st.ClientId, st.clientOpts...);
			err != nil {
			logger.Error("Unable to reconnect ", err)
		}
	}
	if err == nil {
		st.clientStorage.Store(conn)
	} else {
		logger.Panic("Unable to reconnect after wait ", err)
	}
}

func (st Notifier) New(configPath string, db *s.Database) (producer.Producer, error) {
	var err error
	n := Notifier{
		db:       db,
		clientMu: &sync.Mutex{},
	}
	var confBytes []byte
	if confBytes, err = ioutil.ReadFile(filepath.Clean(configPath)); err == nil {
		if err = json.Unmarshal(confBytes, &n); err == nil {
			if len(n.ClusterId) > 0 && len(n.ClientId) > 0 && len(n.URL) > 0 {
				var conn stan.Conn
				n.clientOpts = []stan.Option {
					stan.NatsURL(n.URL),
					stan.Pings(n.PingSleep, n.PingMax),
					stan.SetConnectionLostHandler(n.reconnect),
				}
				if conn, err = stan.Connect(n.ClusterId, n.ClientId, n.clientOpts...); err == nil {
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

func (st Notifier) Send(_ bool, torrent s.TorrentInfo) {
	var err error
	bb := bytes.Buffer{}
	enc := gob.NewEncoder(&bb)
	if err = enc.Encode(torrent); err == nil {
		err = errors.New("")
		for i := 0; i < stan.DefaultPingMaxOut && err != nil; i++ {
			err = st.client().Publish(st.Subject, bb.Bytes())
			time.Sleep(stan.DefaultConnectWait)
		}
	}
	if err != nil {
		logger.Error(err)
	}
}

func (st Notifier) Close() {
	if st.clientStorage != nil {
		if err := st.client().Close(); err != nil {
			logger.Error(err)
		}
	}
}

func (st Notifier) SendNxGet(uint) {}
