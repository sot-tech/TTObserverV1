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

package sqldb

import (
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/op/go-logging"

	"sot-te.ch/TTObserverV1/producer"
	s "sot-te.ch/TTObserverV1/shared"
)

var logger = logging.MustGetLogger("sqldb")

func init() {
	producer.RegisterFactory("sqldb", DB{})
}

type DB struct {
	Driver      string
	Address     string
	DeleteQuery string `json:"deletequery"`
	InsertQuery string `json:"insertquery"`
	CalculateV2 bool   `json:"calculatev2"`
}

func (DB) New(path string, _ s.Database) (producer.Producer, error) {
	var err error
	n := new(DB)
	var confBytes []byte
	if confBytes, err = os.ReadFile(filepath.Clean(path)); err == nil {
		if err = json.Unmarshal(confBytes, n); err == nil {
			if len(n.Driver) > 0 && len(n.Address) > 0 && len(n.DeleteQuery) > 0 {
				var con *sql.DB
				if con, err = sql.Open(n.Driver, n.Address); err == nil {
					defer con.Close()
					err = con.Ping()
				}
			} else {
				err = s.ErrRequiredParameters
			}
		}
	}
	return n, err
}

func (d DB) Send(_ bool, t *s.TorrentInfo) {
	var err error
	var h1, h2 []byte
	if h1, h2, err = s.GenerateTorrentInfoHash(t.Data, d.CalculateV2); err == nil {
		var con *sql.DB
		if con, err = sql.Open(d.Driver, d.Address); err == nil {
			defer con.Close()
			err = d.ExecDB(con, t.Name, h1, h2)
		}
	}
	if err != nil {
		logger.Error(err)
	}
}

func (d DB) ExecDB(con *sql.DB, name string, h1, h2 []byte) (err error) {
	if err = con.Ping(); err == nil {
		var tx *sql.Tx
		if tx, err = con.Begin(); err == nil {
			_, err = tx.Exec(d.DeleteQuery, name)
			if err == nil {
				var st *sql.Stmt
				if st, err = tx.Prepare(d.InsertQuery); err == nil {
					_, err = st.Exec(h1, name)
					if len(h2) > 0 && err == nil {
						if _, err = st.Exec(h2, name); err == nil {
							_, err = st.Exec(h2[:sha1.Size], name)
						}
					}
					_ = st.Close()
				}
			}
			if err == nil {
				err = tx.Commit()
			} else {
				logger.Error(err)
				err = tx.Rollback()
			}
		}
	}
	return
}

func (DB) SendNxGet(uint) {}

func (DB) Close() {}
