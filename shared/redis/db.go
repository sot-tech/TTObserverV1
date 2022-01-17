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

package redis

import (
	"context"
	"errors"
	"github.com/go-redis/redis/v8"
	s "sot-te.ch/TTObserverV1/shared"
	"strconv"
)

var ctx = context.Background()

const (
	DBDriver      = "redis"
	ParamAddress  = "address"
	ParamPassword = "password"
	ParamDB       = "db"

	prefix        = "tt_"
	sChat         = prefix + "chat"
	sAdmin        = prefix + "adm"
	hConfOffset   = prefix + "offset"
	hTorrent      = prefix + "t_"
	hTorrentId    = prefix + "ti"
	hTorrentFile  = hTorrent + "f_"
	hTorrentImage = hTorrent + "i_"
	hTorrentMeta  = hTorrent + "m_"
	pIndex        = "idx"
	pName         = "name"
	pData         = "data"
)

type database struct {
	con *redis.Client
}

func init() {
	s.RegisterFactory(DBDriver, func(m map[string]interface{}) (s.Database, error) {
		var err error
		var db *database
		if v, ok := m[ParamAddress]; ok && v != nil {
			db = new(database)
			opts := &redis.Options{Addr: v.(string)}
			if v, ok = m[ParamPassword]; ok && v != nil {
				opts.Password = v.(string)
			}
			if v, ok = m[ParamDB]; ok && v != nil {
				opts.DB = v.(int)
			}
			db.con = redis.NewClient(opts)
			err = db.con.Ping(ctx).Err()
		} else {
			err = errors.New("address not set")
		}
		return db, err
	})
}

func (d database) AddAdmin(id int64) error {
	return d.con.SAdd(ctx, sAdmin, strconv.FormatInt(id, 10)).Err()
}

func (d database) AddChat(chat int64) error {
	return d.con.SAdd(ctx, sChat, strconv.FormatInt(chat, 10)).Err()
}

func (d database) AddTorrentImage(id int64, image []byte) error {
	return d.con.Set(ctx, hTorrentImage+strconv.FormatInt(id, 10), image, 0).Err()
}

func (d database) AddTorrentMeta(id int64, meta map[string]string) error {
	return d.con.HSet(ctx, hTorrentMeta+strconv.FormatInt(id, 10), meta).Err()
}

func (d database) AddTorrent(name string, data []byte, files []string) (id int64, err error) {
	hkey := hTorrent + name
	if err = d.con.HSet(ctx, hkey, pName, name, pData, data).Err(); err == nil {
		var sid string
		if sid, err = d.con.HGet(ctx, hkey, pIndex).Result(); err == nil {
			if len(sid) == 0 {
				if id, err = d.con.Incr(ctx, hTorrent+pIndex).Result(); err == nil {
					sid = strconv.FormatInt(id, 10)
					if err = d.con.HSet(ctx, hkey, pIndex, sid).Err(); err == nil {
						err = d.con.HSet(ctx, hTorrentId, sid, hkey).Err()
					}
				}
				if err != nil {
					id = s.InvalidDBId
					return
				}
			} else {
				if id, err = strconv.ParseInt(sid, 10, 64); err != nil {
					id = s.InvalidDBId
					return
				}
			}
			l := len(files)
			if l > 0 {
				ifs := make([]interface{}, l)
				for i := 0; i < l; l++ {
					ifs[i] = files[i]
				}
				err = d.con.SAdd(ctx, hTorrentFile+sid, ifs...).Err()
			}
		}
	}
	return
}

func (d database) CheckTorrent(id int64) (bool, error) {
	return d.con.HExists(ctx, hTorrent+strconv.FormatInt(id, 10), pName).Result()
}

func (d *database) Close() {
	if d.con != nil {
		_ = d.con.Close()
	}
}

func (d database) DelAdmin(id int64) error {
	return d.con.SRem(ctx, sAdmin, strconv.FormatInt(id, 10)).Err()
}

func (d database) DelChat(chat int64) error {
	return d.con.SRem(ctx, sChat, strconv.FormatInt(chat, 10)).Err()
}

func (d database) GetAdminExist(chat int64) (bool, error) {
	return d.con.SIsMember(ctx, sAdmin, strconv.FormatInt(chat, 10)).Result()
}

func (d database) GetAdmins() (out []int64, err error) {
	return d.getIntList(sAdmin)
}

func (d database) getIntList(hash string) (out []int64, err error) {
	var admins []string
	if admins, err = d.con.SMembers(ctx, hash).Result(); err == nil {
		if l := len(admins); l > 0 {
			out = make([]int64, 0, l)
			for _, a := range admins {
				if id, idErr := strconv.ParseInt(a, 10, 64); err == nil {
					out = append(out, id)
				} else {
					err = idErr
					break
				}
			}
		}
	}
	return
}

func (d database) GetChatExist(chat int64) (bool, error) {
	return d.con.SIsMember(ctx, sChat, strconv.FormatInt(chat, 10)).Result()
}

func (d database) GetChats() ([]int64, error) {
	return d.getIntList(sChat)
}

func (d database) GetCrawlOffset() (uint, error) {
	offset, err := d.con.Get(ctx, hConfOffset).Uint64()
	return uint(offset), err
}

func (d database) GetTorrentFiles(torrent int64) ([]string, error) {
	return d.con.SMembers(ctx, hTorrentFile+strconv.FormatInt(torrent, 10)).Result()
}

func (d database) GetTorrentImage(id int64) ([]byte, error) {
	return d.con.Get(ctx, hTorrentImage+strconv.FormatInt(id, 10)).Bytes()
}

func (d database) GetTorrentMeta(id int64) (map[string]string, error) {
	return d.con.HGetAll(ctx, hTorrentMeta+strconv.FormatInt(id, 10)).Result()
}

func (d database) GetTorrent(torrent string) (id int64, err error) {
	id = s.InvalidDBId
	var sid string
	if sid, err = d.con.HGet(ctx, hTorrent+torrent, pIndex).Result(); err == nil && len(sid) > 0 {
		id, err = strconv.ParseInt(sid, 10, 64)
	}
	return
}

func (d database) UpdateCrawlOffset(offset uint) error {
	return d.con.Set(ctx, hConfOffset, strconv.FormatUint(uint64(offset), 10), 0).Err()
}
