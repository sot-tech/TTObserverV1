/*
 * BSD-3-Clause
 * Copyright 2022 sot (aka PR_713, C_rho_272)
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

package lmdb

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	lmdbp "github.com/bmatsuo/lmdb-go/lmdb"
	"github.com/op/go-logging"

	"sot-te.ch/TTObserverV1/producer"
	s "sot-te.ch/TTObserverV1/shared"
)

var logger = logging.MustGetLogger("lmdb")

func init() {
	producer.RegisterFactory("lmdb", conf{})
}

type conf struct {
	Path        string
	MaxSize     int64  `json:"maxsize"`
	DBName      string `json:"dbname"`
	CalculateV2 bool   `json:"calculatev2"`
}

type mdb struct {
	*lmdbp.Env
	dbi    *lmdbp.DBI
	calcV2 bool
}

func (conf) New(path string, _ s.Database) (producer.Producer, error) {
	var err error
	cfg := new(conf)
	var db *mdb
	var confBytes []byte
	if confBytes, err = os.ReadFile(filepath.Clean(path)); err == nil {
		if err = json.Unmarshal(confBytes, cfg); err == nil {
			if len(cfg.Path) > 0 && len(cfg.DBName) > 0 {
				db = &mdb{calcV2: cfg.CalculateV2}
				db.Env, err = lmdbp.NewEnv()
				if err == nil {
					err = db.Env.SetMaxDBs(1)
					if cfg.MaxSize <= 0 {
						cfg.MaxSize = 1 << 30
					}
					err = db.Env.SetMapSize(cfg.MaxSize)
					if _, err = os.Stat(cfg.Path); err != nil && errors.Is(err, os.ErrNotExist) {
						err = os.Mkdir(cfg.Path, 0700)
					}
				}
				if err == nil {
					err = db.Open(cfg.Path, 0, 0640)
				}
				if err == nil {
					err = db.Update(func(txn *lmdbp.Txn) (err error) {
						db.dbi, err = txn.CreateDBI(cfg.DBName)
						return
					})
				}
				if err != nil {
					err = db.Env.Close()
				}
			} else {
				err = s.ErrRequiredParameters
			}
		}
	}
	return db, err
}

func (d *mdb) Send(_ bool, t *s.TorrentInfo) {
	var err error
	var h1, h2 []byte
	if h1, h2, err = s.GenerateTorrentInfoHash(t.Data, d.calcV2); err == nil {
		err = d.Update(func(txn *lmdbp.Txn) (err error) {
			v := []byte(t.Name)
			if err = txn.Put(d.dbi, h1, v, 0); err != nil {
				return
			}
			if len(h2) > 0 {
				if err = txn.Put(d.dbi, h2, v, 0); err == nil {
					err = txn.Put(d.dbi, h2[:sha1.Size], v, 0)
				}
			}
			return
		})
	}
	if err != nil {
		logger.Error(err)
	}
}

func (*mdb) SendNxGet(uint) {}

func (d *mdb) Close() {
	if d.Env != nil {
		_ = d.Env.Close()
	}
}
