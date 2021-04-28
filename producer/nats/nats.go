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

package nats

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"github.com/nats-io/nats.go"
	"github.com/op/go-logging"
	"io/ioutil"
	"path/filepath"
	"sot-te.ch/TTObserverV1/producer"
	s "sot-te.ch/TTObserverV1/shared"
	"time"
)

var logger = logging.MustGetLogger("nats")

const (
	maxMessageSize      = 10485760 //10 MiB
	maxMessagesInBuffer = 10
	maxMessageAge       = 24 * time.Hour
)

func init() {
	producer.RegisterFactory("nats", Notifier{})
}

type Notifier struct {
	URL           string `json:"url"`
	Stream        string `json:"stream"`
	Subject       string `json:"subject"`
	PingInterval  int    `json:"pinginterval"`
	MaxReconnects int    `json:"maxreconnects"`
	db            *s.Database
	client        *nats.Conn
	js            nats.JetStreamContext
}

func (nc *Notifier) init() error {
	var err error
	if len(nc.URL) > 0 {
		clientOpts := []nats.Option{
			nats.ReconnectWait(time.Duration(nc.PingInterval) * time.Second),
			nats.PingInterval(time.Duration(nc.PingInterval) * time.Second),
			nats.MaxReconnects(nc.MaxReconnects),
			nats.ReconnectBufSize(maxMessagesInBuffer * maxMessageSize),
		}
		if nc.client, err = nats.Connect(nc.URL, clientOpts...); err == nil {
			if len(nc.Stream) > 0 {
				if nc.js, err = nc.client.JetStream(); err == nil {
					var exist bool
					for sn := range nc.js.StreamNames() {
						if nc.Stream == sn {
							exist = true
							break
						}
					}
					logger.Debug("Checking stream ", nc.Stream, " exist: ", exist)
					if !exist {
						if _, err = nc.js.AddStream(&nats.StreamConfig{
							Name:       nc.Stream,
							Subjects:   []string{nc.Subject},
							Retention:  nats.WorkQueuePolicy,
							MaxAge:     maxMessageAge,
							MaxMsgSize: maxMessageSize,
						}); err == nil {
							logger.Info("Created new stream: ", nc.Stream)
						} else {
							logger.Error("Unable to create stream: ", nc.Stream, " error: ", err)
						}
					}
				} else {
					logger.Warning("Unable to get JetStream context: ", err, " falling back to pure NATS")
					err = nil
				}
			}
		}
	} else {
		err = errors.New("required parameters not set")
	}
	return err
}

func (nc Notifier) New(configPath string, db *s.Database) (producer.Producer, error) {
	var err error
	n := &Notifier{db: db}
	var confBytes []byte
	if confBytes, err = ioutil.ReadFile(filepath.Clean(configPath)); err == nil {
		if err = json.Unmarshal(confBytes, n); err == nil {
			err = n.init()
		}
	}
	return n, err
}

func (nc Notifier) Send(_ bool, torrent *s.TorrentInfo) {
	var err error
	bb := bytes.Buffer{}
	enc := gob.NewEncoder(&bb)
	if err = enc.Encode(torrent.Name); err == nil {
		msg := &nats.Msg{
			Subject: nc.Subject,
			Data:    bb.Bytes(),
		}
		if nc.js == nil {
			err = nc.client.PublishMsg(msg)
		} else {
			_, err = nc.js.PublishMsg(msg)
		}
	}
	if err != nil {
		logger.Error(err)
	}
}

func (nc *Notifier) Close() {
	if nc.client != nil {
		nc.client.Close()
	}
}

func (nc Notifier) SendNxGet(uint) {}
