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

package redis

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/go-redis/redis/v8"
	"github.com/op/go-logging"

	"sot-te.ch/TTObserverV1/producer"
	s "sot-te.ch/TTObserverV1/shared"
)

var (
	logger = logging.MustGetLogger("redis")
	ctx    = context.Background()
)

const (
	defaultNamePrefix = "TT_N_"
	v1Field           = "v1"
	v2Field           = "v2"
	hybridField       = "v2to1"
)

func init() {
	producer.RegisterFactory("redis", Notifier{})
}

type Notifier struct {
	Address       string `json:"address"`
	Password      string `json:"password"`
	DB            int    `json:"db"`
	HashKey       string `json:"hashkey"`
	NameKeyPrefix string `json:"namekeyprefix"`
	CalculateV2   bool   `json:"calculatev2"`
	con           *redis.Client
}

func (r Notifier) New(configPath string, _ s.Database) (producer.Producer, error) {
	var err error
	n := new(Notifier)
	var confBytes []byte
	if confBytes, err = ioutil.ReadFile(filepath.Clean(configPath)); err == nil {
		if err = json.Unmarshal(confBytes, n); err == nil {
			if len(n.Address) > 0 && len(n.HashKey) > 0 {
				n.con = redis.NewClient(&redis.Options{
					Addr:     n.Address,
					Password: n.Password,
					DB:       n.DB,
				})
				err = n.con.Ping(ctx).Err()
			} else {
				err = s.ErrRequiredParameters
			}
			if len(n.NameKeyPrefix) == 0 {
				logger.Warning("Name key prefix not set, using default: ", defaultNamePrefix)
				n.NameKeyPrefix = defaultNamePrefix
			}
		}
	}
	return n, err
}

func (r Notifier) Send(isNew bool, t *s.TorrentInfo) {
	torrentNameKey := r.NameKeyPrefix + t.Name
	if !isNew {
		if prevHashes, err := r.con.HMGet(ctx, torrentNameKey, v1Field, v2Field, hybridField).Result(); err != nil {
			logger.Error(err)
		} else {
			if len(prevHashes) > 0 {
				fields := make([]string, 0, len(prevHashes))
				for _, h := range prevHashes {
					if h != nil {
						if f, isOk := h.(string); isOk {
							fields = append(fields, f)
						} else {
							logger.Warning(h, " is not string type")
						}
					}
				}
				if len(fields) > 0 {
					if err = r.con.HDel(ctx, r.HashKey, fields...).Err(); err != nil {
						logger.Error(err)
					}
				}
			}
		}
	}
	var err error
	values := make([]any, 0, 6)
	var h1, h2 []byte
	if h1, h2, err = s.GenerateTorrentInfoHash(t.Data, r.CalculateV2); err == nil {
		values = append(values, string(h1), t.Name)
		if r.CalculateV2 {
			values = append(values, string(h2), t.Name, string(h2[:sha1.Size]), t.Name)
		}
		if err = r.con.HSet(ctx, r.HashKey, values...).Err(); err == nil {
			values[0], values[1] = v1Field, values[0]
			if r.CalculateV2 {
				values[2], values[3] = v2Field, values[2]
				values[4], values[5] = hybridField, values[4]
			}
			err = r.con.HSet(ctx, torrentNameKey, values...).Err()
		}
	}
	if err != nil {
		logger.Error(err)
	}
}

func (r *Notifier) Close() {
	if r.con != nil {
		_ = r.con.Close()
	}
}

func (Notifier) SendNxGet(uint) {}
