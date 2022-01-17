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
	"errors"
	"github.com/nats-io/nats.go"
	"math/rand"
	"time"
)

type Cluster struct {
	NatsURL                string        `json:"url"`
	MasterSubject          string        `json:"mastersubject"`
	ProposeSubject         string        `json:"masterproposesubject"`
	MasterPingInterval     time.Duration `json:"masterpinginterval"`
	MaxWait                time.Duration `json:"msgmaxwait"`
	MasterRetryCount       uint32        `json:"masterretrycount"`
	StartFunction          func() error  `json:"-"`
	SuspendFunction        func()        `json:"-"`
	client                 *nats.Conn
	masterSub, proposedSub *nats.Subscription
	stopped                chan interface{}
}

const (
	DefaultPingInterval = time.Duration(10)
	pingMsg             = "PING"
	answerMsg           = "PONG"
)

var (
	errNATSConfigNotSet = errors.New("nats URL or subjects not set")
	errFunctionsNotSet  = errors.New("function for start or suspend not set")
)

func (cl *Cluster) Start() error {
	if len(cl.NatsURL) == 0 || len(cl.MasterSubject) == 0 || len(cl.ProposeSubject) == 0 {
		return errNATSConfigNotSet
	}
	if cl.StartFunction == nil || cl.SuspendFunction == nil {
		return errFunctionsNotSet
	}
	if cl.MasterPingInterval <= 0 {
		logger.Warning("MasterPingInterval not set, using ", DefaultPingInterval)
		cl.MasterPingInterval = DefaultPingInterval
	}
	clientOpts := []nats.Option{
		nats.ReconnectWait(cl.MasterPingInterval * time.Second / 3),
		nats.PingInterval(cl.MasterPingInterval * time.Second / 3),
		nats.MaxReconnects(-1),
	}
	var err error
	if cl.client, err = nats.Connect(cl.NatsURL, clientOpts...); err == nil {
		cl.stopped = make(chan interface{})
		var errorCount uint32 = 0
		for {
			select {
			default:
				if resp, err := cl.client.Request(cl.MasterSubject, []byte(pingMsg), cl.MaxWait*time.Millisecond); err == nil {
					errorCount = 0
					logger.Info("Master alive")
					if len(resp.Data) == 0 || string(resp.Data) != answerMsg {
						logger.Warning("Unexpected response received: ", resp.Data, " ignoring")
					}
				} else if errors.Is(nats.ErrNoResponders, err) || errors.Is(nats.ErrTimeout, err) {
					logger.Warning("Master did not respond")
					errorCount++
					if errorCount >= cl.MasterRetryCount {
						if err = cl.asMaster(); err != nil {
							logger.Error(err)
						}
					}
				} else {
					logger.Error("Error received while master ping: ", err)
				}
			case <-cl.stopped:
				cl.SuspendFunction()
				return nil
			}
			time.Sleep((cl.MasterPingInterval + time.Duration(rand.Int63n(int64(cl.MasterPingInterval)))) * time.Second)
		}
	}
	return err
}

func (cl *Cluster) asMaster() error {
	var err error
	logger.Info("Begin master propose")
	ownId := make([]byte, 32)
	rand.Read(ownId)
	if cl.proposedSub, err = cl.client.Subscribe(cl.ProposeSubject, func(msg *nats.Msg) {
		if msg != nil && !bytes.Equal(ownId, msg.Data) {
			logger.Notice("Received message from another node ", msg.Data)
			_ = msg.Respond(ownId)
		}
	}); err != nil {
		return err
	}
	_, reqErr := cl.client.Request(cl.ProposeSubject, ownId, cl.MaxWait*time.Millisecond)
	master := errors.Is(nats.ErrNoResponders, reqErr) || errors.Is(nats.ErrTimeout, reqErr)
	if master {
		logger.Notice("Become a master")
		cl.masterSub, err = cl.client.Subscribe(cl.MasterSubject, func(msg *nats.Msg) {
			if msg != nil {
				if err = msg.Respond([]byte(answerMsg)); err != nil {
					logger.Error(err)
				}
			}
		})
		if err == nil {
			err = cl.StartFunction()
		}
	} else {
		logger.Notice("Found another master propose")
	}
	_ = cl.proposedSub.Unsubscribe()
	cl.proposedSub = nil
	return err
}

func (cl *Cluster) Stop() {
	if cl.stopped != nil {
		close(cl.stopped)
	}
	if cl.proposedSub != nil {
		_ = cl.proposedSub.Unsubscribe()
	}
	if cl.masterSub != nil {
		_ = cl.masterSub.Unsubscribe()
	}
	if cl.client != nil && cl.client.IsConnected() {
		cl.client.Close()
	}
}
