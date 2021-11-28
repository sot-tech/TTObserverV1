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
	"time"
)

var logger = logging.MustGetLogger("stan")

func init() {
	producer.RegisterFactory("stan", Notifier{})
}

type Notifier struct {
	URL             string `json:"url"`
	ClusterId       string `json:"clusterid"`
	ClientId        string `json:"clientid"`
	Subject         string `json:"subject"`
	PingSleep       int    `json:"pingsleep"`
	PingMax         int    `json:"pingmax"`
	db              *s.Database
	client          stan.Conn
	reconnectWaiter *sync.WaitGroup
	clientOpts      []stan.Option
}

func (st *Notifier) reconnect(_ stan.Conn, cause error) {
	st.reconnectWaiter.Add(1)
	defer st.reconnectWaiter.Done()
	err := cause
	logger.Error("STAN connection lost ", err)
	for i := 0; i < stan.DefaultPingMaxOut && err != nil; i++ {
		time.Sleep(stan.DefaultConnectWait)
		err = st.init()
	}
	if err != nil {
		logger.Panic("Unable to reconnect to Stan ", err)
	}
}

func (st *Notifier) init() error {
	var err error
	if len(st.ClusterId) > 0 && len(st.ClientId) > 0 && len(st.URL) > 0 {
		if len(st.clientOpts) == 0 {
			st.clientOpts = []stan.Option{
				stan.NatsURL(st.URL),
				stan.Pings(st.PingSleep, st.PingMax),
				stan.SetConnectionLostHandler(st.reconnect),
			}
		}
		st.client, err = stan.Connect(st.ClusterId, st.ClientId, st.clientOpts...)
	} else {
		err = errors.New("required parameters not set")
	}
	return err
}

func (st Notifier) New(configPath string, db *s.Database) (producer.Producer, error) {
	var err error
	n := &Notifier{
		db:              db,
		reconnectWaiter: &sync.WaitGroup{},
	}
	var confBytes []byte
	if confBytes, err = ioutil.ReadFile(filepath.Clean(configPath)); err == nil {
		if err = json.Unmarshal(confBytes, n); err == nil {
			if len(n.ClusterId) > 0 && len(n.ClientId) > 0 && len(n.URL) > 0 {
				err = n.init()
			} else {
				err = errors.New("required parameters not set")
			}
		}
	}
	return n, err
}

func (st Notifier) Send(_ bool, torrent *s.TorrentInfo) {
	var err error
	bb := new(bytes.Buffer)
	enc := gob.NewEncoder(bb)
	if err = enc.Encode(torrent); err == nil {
		err = errors.New("")
		for i := 0; i < stan.DefaultPingMaxOut && err != nil; i++ {
			st.reconnectWaiter.Wait()
			if err = st.client.Publish(st.Subject, bb.Bytes()); err != nil {
				time.Sleep(stan.DefaultConnectWait)
			}
		}
	}
	if err != nil {
		logger.Error(err)
	}
}

func (st *Notifier) Close() {
	if st.client != nil {
		if err := st.client.Close(); err != nil{
			logger.Warning(err)
		}
	}
}

func (_ Notifier) SendNxGet(uint) {}
