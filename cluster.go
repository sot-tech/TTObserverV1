/*
 * BSD-3-Clause
 * Copyright 2021 sot (aka PR_713, C_rho_272)
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
	cr "crypto/rand"
	"errors"
	"github.com/nats-io/nats.go"
	"math/rand"
	"time"
)

type Cluster struct {
	NatsURL                string        `json:"url"`
	MasterSubject          string        `json:"mastersubject"`
	ProposeSubject         string        `json:"masterproposesubject"`
	MasterPingInterval     int64         `json:"masterpinginterval"`
	MaxWait                time.Duration `json:"msgmaxwait"`
	MasterRetryCount       uint32        `json:"masterretrycount"`
	StartFn                func() error  `json:"-"`
	SuspendFn              func()        `json:"-"`
	client                 *nats.Conn
	masterSub, proposedSub *nats.Subscription
	stopped                chan interface{}
}

const (
	DefaultPingInterval = 10
	idLen               = 8
)

var (
	errNATSConfigNotSet = errors.New("nats URL or subjects not set")
	errFunctionsNotSet  = errors.New("function for start or suspend not set")
	ownId               = make([]byte, idLen)
)

func init() {
	//Sometimes the same ID is generated so using secure random
	if _, err := cr.Read(ownId); err != nil {
		logger.Panic(err)
	}
	seed := int64(ownId[0])<<56 | int64(ownId[1])<<48 | int64(ownId[2])<<40 | int64(ownId[3])<<32 |
		int64(ownId[4])<<24 | int64(ownId[5])<<16 | int64(ownId[6])<<8 | int64(ownId[7])
	rand.Seed(seed)
}

func (cl *Cluster) Start() error {
	if len(cl.NatsURL) == 0 || len(cl.MasterSubject) == 0 || len(cl.ProposeSubject) == 0 {
		return errNATSConfigNotSet
	}
	if cl.StartFn == nil || cl.SuspendFn == nil {
		return errFunctionsNotSet
	}
	if cl.MasterPingInterval <= 0 {
		logger.Warning("MasterPingInterval not set, using ", DefaultPingInterval)
		cl.MasterPingInterval = DefaultPingInterval
	}
	pingReconnectWait := (time.Duration(cl.MasterPingInterval) * time.Second) / 3
	clientOpts := []nats.Option{
		nats.ReconnectWait(pingReconnectWait),
		nats.PingInterval(pingReconnectWait),
		nats.MaxReconnects(-1),
	}
	var err error
	if cl.client, err = nats.Connect(cl.NatsURL, clientOpts...); err == nil {
		cl.stopped = make(chan interface{})
		var errorCount uint32 = 0
		var resp *nats.Msg
		for {
			select {
			default:
				if resp, err = cl.client.Request(cl.MasterSubject, ownId, cl.MaxWait*time.Millisecond); err == nil {
					if errorCount > 0 {
						logger.Notice("Master alive, id: ", resp.Data)
						errorCount = 0
					}
				} else if cl.noResponders(err) {
					logger.Warning("Master did not respond")
					errorCount++
					if errorCount >= cl.MasterRetryCount {
						if err = cl.asMaster(); err != nil {
							logger.Error("Master work error ", err, " suspending")
							cl.SuspendFn()
						}
					}
				} else {
					logger.Error("Error received while master ping: ", err)
				}
			case <-cl.stopped:
				cl.SuspendFn()
				return nil
			}
			time.Sleep(time.Duration(cl.MasterPingInterval+rand.Int63n(cl.MasterPingInterval)) * time.Second)
		}
	}
	return err
}

func (cl *Cluster) asMaster() error {
	var err error
	logger.Info("Begin master propose, my id: ", ownId)
	if cl.proposedSub, err = cl.client.Subscribe(cl.ProposeSubject, respondId); err == nil {
		defer cl.unsubPropose()
		var resp *nats.Msg
		if resp, err = cl.client.Request(cl.ProposeSubject, ownId, cl.MaxWait*time.Millisecond); err == nil {
			logger.Notice("Found another master propose from node: ", resp.Data)
		} else if cl.noResponders(err) {
			err = nil
			logger.Notice("Become a master, my id: ", ownId)
			if cl.masterSub, err = cl.client.Subscribe(cl.MasterSubject, respondId); err == nil {
				defer cl.unsubMaster()
				err = cl.StartFn()
			}
		}
	}
	return err
}

func (cl Cluster) noResponders(err error) bool {
	return errors.Is(err, nats.ErrNoResponders) || cl.client.IsConnected() && errors.Is(err, nats.ErrTimeout)
}

func respondId(msg *nats.Msg) {
	if msg != nil && !bytes.Equal(ownId, msg.Data) {
		logger.Debug("Received message from node: ", msg.Data)
		if err := msg.Respond(ownId); err != nil {
			logger.Error(err)
		}
	}
}

func (cl *Cluster) unsubPropose() {
	if cl.proposedSub != nil {
		_ = cl.proposedSub.Unsubscribe()
	}
}

func (cl *Cluster) unsubMaster() {
	if cl.masterSub != nil {
		_ = cl.masterSub.Unsubscribe()
	}
}

func (cl *Cluster) Stop() {
	if cl.stopped != nil {
		close(cl.stopped)
	}
	cl.unsubPropose()
	cl.unsubMaster()
	if cl.client != nil && cl.client.IsConnected() {
		cl.client.Close()
	}
}
