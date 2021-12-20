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
	"errors"
	"github.com/go-redis/redis/v8"
	"github.com/minio/sha256-simd"
	"github.com/op/go-logging"
	"github.com/zeebo/bencode"
	"io/ioutil"
	"path/filepath"
	"sot-te.ch/TTObserverV1/producer"
	s "sot-te.ch/TTObserverV1/shared"
)

var (
	logger = logging.MustGetLogger("redis")
	ctx    = context.Background()
)

const (
	v1Field       = "v1"
	v2Field       = "v2"
	hybridField   = "v2to1"
	hybridHashLen = 20
)

func init() {
	producer.RegisterFactory("redis", Notifier{})
}

type BABencode []byte

func (ba *BABencode) UnmarshalBencode(in []byte) error {
	*ba = append([]byte(nil), in...)
	return nil
}

type torrent struct {
	Info BABencode `bencode:"info"`
}

type Notifier struct {
	Address     string `json:"address"`
	Password    string `json:"password,omitempty"`
	DB          int    `json:"db,omitempty"`
	Hash        string `json:"hash"`
	CalculateV2 bool   `json:"calculatev2"`
	con         *redis.Client
}

func (r Notifier) New(configPath string, _ *s.Database) (producer.Producer, error) {
	var err error
	n := new(Notifier)
	var confBytes []byte
	if confBytes, err = ioutil.ReadFile(filepath.Clean(configPath)); err == nil {
		if err = json.Unmarshal(confBytes, n); err == nil {
			if len(n.Address) > 0 && len(n.Hash) > 0 {
				n.con = redis.NewClient(&redis.Options{
					Addr:     n.Address,
					Password: n.Password,
					DB:       n.DB,
				})
				err = n.con.Ping(ctx).Err()
			} else {
				err = errors.New("required parameters not set")
			}
		}
	}
	return n, err
}

func (r Notifier) Send(isNew bool, t *s.TorrentInfo) {
	if !isNew {
		if prevHashes, err := r.con.HMGet(ctx, t.Name, v1Field, v2Field, hybridField).Result(); err != nil {
			logger.Error(err)
		} else {
			if len(prevHashes) > 0 {
				fields := make([]string, 0, len(prevHashes))
				for _, h := range prevHashes {
					if h != nil {
						switch f := h.(type) {
						case string:
							fields = append(fields, f)
						default:
							logger.Warning(h, " is not string type")
						}
					}
				}
				if len(fields) > 0 {
					if err = r.con.HDel(ctx, r.Hash, fields...).Err(); err != nil {
						logger.Error(err)
					}
				}
			}
		}
	}
	torrent := new(torrent)
	var err error
	if err = bencode.DecodeBytes(t.Data, torrent); err == nil {
		values := make([]interface{}, 0, 6)
		s1 := sha1.New()
		s1.Write(torrent.Info)
		values = append(values, string(s1.Sum(nil)), t.Name)
		if r.CalculateV2 {
			s2 := sha256.New()
			s2.Write(torrent.Info)
			v2Sum := s2.Sum(nil)
			values = append(values, string(v2Sum), t.Name, string(v2Sum[:hybridHashLen]), t.Name)
		}
		if err = r.con.HSet(ctx, r.Hash, values...).Err(); err == nil {
			values[0], values[1] = v1Field, values[0]
			if r.CalculateV2 {
				values[2], values[3] = v2Field, values[2]
				values[4], values[5] = hybridField, values[4]
			}
			err = r.con.HSet(ctx, t.Name, values...).Err()
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

func (_ Notifier) SendNxGet(uint) {}
