/*
 * BSD-3-Clause
 * Copyright 2022 sot (PR_713, C_rho_272)
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
	"database/sql"
	"errors"
	"strconv"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	s "sot-te.ch/TTObserverV1/shared"
)

const (
	SQLLiteDriver  = "sqlite3"
	PgDriver       = "postgres"
	dbFileParam    = "dbfile"
	dbAddressParam = "dbaddress"

	selectChats = "SELECT ID FROM TT_CHAT"
	insertChat  = "INSERT INTO TT_CHAT(ID) VALUES ($1)"
	delChat     = "DELETE FROM TT_CHAT WHERE ID = $1"
	existChat   = "SELECT TRUE FROM TT_CHAT WHERE ID = $1"

	selectAdmins = "SELECT ID FROM TT_ADMIN"
	insertAdmin  = "INSERT INTO TT_ADMIN(ID) VALUES ($1)"
	delAdmin     = "DELETE FROM TT_ADMIN WHERE ID = $1"
	existAdmin   = "SELECT 1 FROM TT_ADMIN WHERE ID = $1"

	selectTorrents = "SELECT ID, NAME, DATA, IMAGE FROM TT_TORRENT"

	selectTorrentId       = "SELECT ID FROM TT_TORRENT WHERE NAME = $1"
	existTorrent          = "SELECT 1 FROM TT_TORRENT WHERE ID = $1"
	insertTorrent         = "INSERT INTO TT_TORRENT(ID, NAME, DATA, IMAGE) VALUES ($1, $2, $3, $4)"
	insertOrUpdateTorrent = "INSERT INTO TT_TORRENT(NAME, DATA) VALUES ($1, $2) ON CONFLICT(NAME) DO UPDATE SET DATA = EXCLUDED.DATA"

	selectTorrentMeta = "SELECT NAME, VALUE FROM TT_TORRENT_META WHERE TORRENT = $1"
	insertTorrentMeta = "INSERT INTO TT_TORRENT_META(TORRENT, NAME, VALUE) VALUES($1, $2, $3) ON CONFLICT(TORRENT,NAME) DO UPDATE SET VALUE = EXCLUDED.VALUE"

	selectTorrentFiles = "SELECT NAME FROM TT_TORRENT_FILE WHERE TORRENT = $1"
	insertTorrentFile  = "INSERT INTO TT_TORRENT_FILE(TORRENT, NAME) VALUES ($1, $2) ON CONFLICT (TORRENT,NAME) DO NOTHING"

	selectTorrentImage = "SELECT IMAGE FROM TT_TORRENT WHERE ID = $1"
	insertTorrentImage = "UPDATE TT_TORRENT SET IMAGE = $1 WHERE ID = $2"

	selectConfig         = "SELECT VALUE FROM TT_CONFIG WHERE NAME = $1"
	insertOrUpdateConfig = "INSERT INTO TT_CONFIG(NAME, VALUE) VALUES ($1, $2) ON CONFLICT(NAME) DO UPDATE SET VALUE = EXCLUDED.VALUE"

	confCrawlOffset = "CRAWL_OFFSET"
)

func init() {
	s.RegisterFactory(SQLLiteDriver, func(m map[string]any) (db s.Database, err error) {
		if v, exist := m[dbFileParam]; exist && v != nil {
			db, err = newDb(SQLLiteDriver, v.(string))
		} else {
			err = s.ErrRequiredParameters
		}
		return
	})
	s.RegisterFactory(PgDriver, func(m map[string]any) (db s.Database, err error) {
		if v, exist := m[dbAddressParam]; exist && v != nil {
			db, err = newDb(PgDriver, v.(string))
		} else {
			err = s.ErrRequiredParameters
		}
		return
	})
}

func newDb(driver, param string) (s.Database, error) {
	var err error
	var db *database
	if len(param) > 0 {
		db = new(database)
		if db.con, err = sql.Open(driver, param); err == nil {
			err = db.checkConnection()
		}
	} else {
		err = s.ErrRequiredParameters
	}
	return db, err
}

type database struct {
	con *sql.DB
}

func (db database) checkConnection() error {
	var err error
	if db.con == nil {
		err = errors.New("connection not initialized")
	} else {
		err = db.con.Ping()
	}
	return err
}

func (db database) getNotEmpty(query string, args ...any) (bool, error) {
	val := false
	var err error
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.con.Query(query, args...)
		if err == nil && rows != nil {
			defer rows.Close()
			val, err = rows.Next(), rows.Err()
		}
	}
	return val, err
}

func (db database) GetChatExist(chat int64) (bool, error) {
	return db.getNotEmpty(existChat, chat)
}

func (db database) execNoResult(query string, args ...any) error {
	var err error
	err = db.checkConnection()
	if err == nil {
		_, err = db.con.Exec(query, args...)
	}
	return err
}

func (db database) getIntArray(query string, args ...any) (arr []int64, err error) {
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.con.Query(query, args...)
		if err == nil && rows != nil {
			defer rows.Close()
			for rows.Next() {
				var element int64
				if err = rows.Scan(&element); err == nil {
					arr = append(arr, element)
				} else {
					break
				}
			}
			if err == nil {
				err = rows.Err()
			}
		}
	}
	return
}

func (db database) GetChats() ([]int64, error) {
	return db.getIntArray(selectChats)
}

func (db database) AddChat(chat int64) error {
	var exist bool
	var err error
	if exist, err = db.GetChatExist(chat); err == nil && !exist {
		err = db.execNoResult(insertChat, chat)
	}
	return err
}

func (db database) DelChat(chat int64) error {
	return db.execNoResult(delChat, chat)
}

func (db database) GetAdmins() ([]int64, error) {
	return db.getIntArray(selectAdmins)
}

func (db database) GetAdminExist(chat int64) (bool, error) {
	return db.getNotEmpty(existAdmin, chat)
}

func (db database) AddAdmin(id int64) error {
	var exist bool
	var err error
	if exist, err = db.GetAdminExist(id); err == nil && !exist {
		err = db.execNoResult(insertAdmin, id)
	}
	return err
}

func (db database) DelAdmin(id int64) error {
	return db.execNoResult(delAdmin, id)
}

func (db database) CheckTorrent(id int64) (bool, error) {
	return db.getNotEmpty(existTorrent, id)
}

func (db database) GetTorrent(torrent string) (torrentId int64, err error) {
	torrentId = s.InvalidDBId
	if err = db.checkConnection(); err == nil {
		var rows *sql.Rows
		rows, err = db.con.Query(selectTorrentId, torrent)
		if err == nil && rows != nil {
			defer rows.Close()
			if rows.Next() {
				err = rows.Scan(&torrentId)
			}
			if err == nil {
				err = rows.Err()
			}
		}
	}
	return torrentId, err
}

func (db database) AddTorrent(name string, data []byte, files []string) (int64, error) {
	var err error
	var id int64
	if err = db.execNoResult(insertOrUpdateTorrent, name, data); err == nil {
		if id, err = db.GetTorrent(name); err == nil {
			for _, file := range files {
				err = db.execNoResult(insertTorrentFile, id, file)
			}
		}
	}
	return id, err
}

func (db database) GetTorrentFiles(torrent int64) (files []string, err error) {
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.con.Query(selectTorrentFiles, torrent)
		if err == nil && rows != nil {
			defer rows.Close()
			for rows.Next() {
				var file string
				if err = rows.Scan(&file); err == nil {
					files = append(files, file)
				} else {
					files = []string{}
					break
				}
			}
			if err == nil {
				err = rows.Err()
			}
		}
	}
	return files, err
}

func (db database) getConfigValue(name string) (string, error) {
	var val string
	var err error
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.con.Query(selectConfig, name)
		if err == nil && rows != nil {
			defer rows.Close()
			if rows.Next() {
				err = rows.Scan(&val)
			}
			if err == nil {
				err = rows.Err()
			}
		}
	}
	return val, err
}

func (db database) updateConfigValue(name, val string) error {
	return db.execNoResult(insertOrUpdateConfig, name, val)
}

func (db database) GetCrawlOffset() (uint, error) {
	var res uint64
	var val string
	var err error
	if val, err = db.getConfigValue(confCrawlOffset); err == nil {
		res, err = strconv.ParseUint(val, 10, 64)
	}
	return uint(res), err
}

func (db database) UpdateCrawlOffset(offset uint) error {
	return db.updateConfigValue(confCrawlOffset, strconv.FormatUint(uint64(offset), 10))
}

func (db database) GetTorrentMeta(id int64) (map[string]string, error) {
	var err error
	meta := make(map[string]string)
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.con.Query(selectTorrentMeta, id)
		if err == nil && rows != nil {
			defer rows.Close()
			for rows.Next() {
				var name, value string
				if err = rows.Scan(&name, &value); err == nil {
					meta[name] = value
				} else {
					break
				}
			}
			if err == nil {
				err = rows.Err()
			}
		}
	}
	return meta, err
}

func (db database) AddTorrentMeta(id int64, meta map[string]string) error {
	var err error
	for k, v := range meta {
		if err = db.execNoResult(insertTorrentMeta, id, k, v); err != nil {
			break
		}
	}
	return err
}

func (db database) GetTorrentImage(id int64) ([]byte, error) {
	var err error
	image := make([]byte, 0)
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.con.Query(selectTorrentImage, id)
		if err == nil && rows != nil {
			defer rows.Close()
			if rows.Next() {
				err = rows.Scan(&image)
			}
			if err == nil {
				err = rows.Err()
			}
		}
	}
	return image, err
}

func (db database) AddTorrentImage(id int64, image []byte) error {
	return db.execNoResult(insertTorrentImage, image, id)
}

func (db database) Close() {
	if db.con != nil {
		_ = db.con.Close()
	}
}

func (db database) MGetTorrents() (out []s.DBTorrent, err error) {
	out = make([]s.DBTorrent, 0)
	if err = db.checkConnection(); err == nil {
		var rows *sql.Rows
		if rows, err = db.con.Query(selectTorrents); err == nil {
			if err == nil && rows != nil {
				defer rows.Close()
				for rows.Next() {
					t := s.DBTorrent{
						Data:  make([]byte, 0),
						Image: make([]byte, 0),
					}
					if err = rows.Scan(&t.Id, &t.Name, &t.Data, &t.Image); err == nil {
						out = append(out, t)
					} else {
						break
					}
				}
				if err == nil {
					err = rows.Err()
				}
			}
		}
	}
	return
}

func (db database) MPutTorrent(t s.DBTorrent, files []string) (err error) {
	if err = db.checkConnection(); err == nil {
		if err = db.execNoResult(insertTorrent, t.Id, t.Name, t.Data, t.Image); err == nil {
			for _, f := range files {
				if err = db.execNoResult(insertTorrentFile, t.Id, f); err != nil {
					break
				}
			}
		}
	}
	return
}
