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

package producer

import (
	"errors"
	"fmt"

	tts "sot-te.ch/TTObserverV1/shared"
)

type Announcer struct {
	producers []Producer
	db        tts.Database
}

var producers = make(map[string]Producer)

func New(configs []Config, db tts.Database) (*Announcer, error) {
	var err error
	a := &Announcer{
		producers: make([]Producer, 0),
		db:        db,
	}
	if len(configs) > 0 {
		for i, conf := range configs {
			if fac := factories[conf.Type]; fac != nil {
				var producer Producer
				var exist bool
				if len(conf.Id) == 0 {
					logger.Warning("id not set, using '", conf.Type, "', it may make collisions")
					conf.Id = conf.Type
				}
				if producer, exist = producers[conf.Id]; exist && producer != nil {
					logger.Notice("Using already initiated producer ", conf.Id)
				} else {
					logger.Debug("Initiating new producer ", conf.Type)
					if producer, err = fac.New(conf.ConfigPath, db); err == nil {
						if producer != nil {
							a.producers = append(a.producers, producer)
							producers[conf.Id] = producer
						} else {
							err = errors.New(fmt.Sprint("unable to construct producer #", i, " type: ", conf.Type))
						}
					}
				}
			} else {
				err = errors.New(fmt.Sprint("producer #", i, " unknown type: ", conf.Type))
			}
			if err != nil {
				logger.Error(err)
				break
			}
		}
	} else {
		logger.Warning("No producers specified")
	}
	return a, err
}

func (a Announcer) Send(new bool, torrent *tts.TorrentInfo) {
	if torrent != nil {
		for _, n := range a.producers {
			go n.Send(new, torrent)
		}
	}
}

func (a Announcer) SendNxGet(offset uint) {
	for _, n := range a.producers {
		go n.SendNxGet(offset)
	}
}

func (a *Announcer) Close() {
	for _, n := range a.producers {
		n.Close()
	}
}
