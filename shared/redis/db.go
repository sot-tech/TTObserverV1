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
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"

	s "sot-te.ch/TTObserverV1/shared"
)

var ctx = context.Background()

const (
	DBDriver      = "redis"
	ParamAddress  = "address"
	ParamPassword = "password"
	ParamDB       = "db"

	sChat  = "tt_chat"
	sAdmin = "tt_adm"

	kConfOffset   = "tt_offset"
	kTorrentIndex = "tt_idx"

	hTorrentId   = "tt_ti"
	hTorrent     = "tt_t_"
	hTorrentFile = "tt_t_f_"
	hTorrentMeta = "tt_t_m_"

	fIndex = "idx"
	fName  = "name"
	fData  = "data"
	fImage = "img"
)

var ErrTorrentNotFound = errors.New("unable to find torrent hash by id")

type database struct {
	con *redis.Client
}

func asNil(err error) error {
	if errors.Is(err, redis.Nil) {
		err = nil
	}
	return err
}

func init() {
	s.RegisterFactory(DBDriver, func(m map[string]any) (s.Database, error) {
		var err error
		var db *database
		if v, ok := m[ParamAddress]; ok && v != nil {
			db = new(database)
			opts := &redis.Options{Addr: v.(string)}
			if v, ok = m[ParamPassword]; ok && v != nil {
				opts.Password = v.(string)
			}
			if v, ok = m[ParamDB]; ok && v != nil {
				if dbNum, ok := v.(float64); ok {
					opts.DB = int(dbNum)
				} else {
					return nil, s.ErrRequiredParameters
				}
			}
			db.con = redis.NewClient(opts)
			err = db.con.Ping(ctx).Err()
		} else {
			err = errors.New("address not set")
		}
		return db, err
	})
}

func (d *database) AddAdmin(id int64) error {
	return d.con.SAdd(ctx, sAdmin, strconv.FormatInt(id, 10)).Err()
}

func (d *database) AddChat(chat int64) error {
	return d.con.SAdd(ctx, sChat, strconv.FormatInt(chat, 10)).Err()
}

func (d *database) AddTorrentImage(id int64, image []byte) (err error) {
	if len(image) > 0 {
		var hash string
		if hash, err = d.con.HGet(ctx, hTorrentId, strconv.FormatInt(id, 10)).Result(); err == nil || asNil(err) == nil {
			if len(hash) > 0 {
				err = d.con.HSet(ctx, hash, fImage, image).Err()
			} else {
				err = ErrTorrentNotFound
			}
		}
	}
	err = asNil(err)
	return
}

func (d *database) AddTorrentMeta(id int64, meta map[string]string) (err error) {
	l := len(meta)
	if l > 0 {
		m := make(map[string]any, l)
		for k, v := range meta {
			m[k] = v
		}
		err = d.con.HSet(ctx, hTorrentMeta+strconv.FormatInt(id, 10), m).Err()
	}
	return
}

func (d *database) tx(txf func(tx redis.Pipeliner) error) (err error) {
	if pipe, txErr := d.con.TxPipelined(ctx, txf); txErr == nil {
		errs := make([]string, 0)
		for _, c := range pipe {
			if err := c.Err(); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if len(errs) > 0 {
			err = errors.New(strings.Join(errs, "; "))
		}
	} else {
		err = txErr
	}
	return
}

func (d *database) AddTorrent(name string, data []byte, files []string) (int64, error) {
	id := new(int64)
	err := d.tx(func(tx redis.Pipeliner) (err error) {
		hkey := hTorrent + name
		if err = d.con.HSet(ctx, hkey, fName, name, fData, data).Err(); err == nil {
			var sid string
			if sid, err = d.con.HGet(ctx, hkey, fIndex).Result(); err == nil || asNil(err) == nil {
				if len(sid) == 0 {
					if *id, err = d.con.Incr(ctx, kTorrentIndex).Result(); err == nil {
						sid = strconv.FormatInt(*id, 10)
						if err = d.con.HSet(ctx, hkey, fIndex, sid).Err(); err == nil {
							err = d.con.HSet(ctx, hTorrentId, sid, hkey).Err()
						}
					}
				} else {
					*id, err = strconv.ParseInt(sid, 10, 64)
				}
				if err == nil {
					l := len(files)
					if l > 0 {
						ifs := make([]any, l)
						for i := 0; i < l; i++ {
							ifs[i] = files[i]
						}
						err = d.con.SAdd(ctx, hTorrentFile+sid, ifs...).Err()
					}
				}
			}
		}
		return
	})
	if err != nil {
		*id = s.InvalidDBId
	}
	return *id, err
}

func (d *database) CheckTorrent(id int64) (bool, error) {
	exist, err := d.con.HExists(ctx, hTorrentId, strconv.FormatInt(id, 10)).Result()
	return exist, asNil(err)
}

func (d *database) Close() {
	if d.con != nil {
		_ = d.con.Close()
	}
}

func (d *database) DelAdmin(id int64) error {
	return asNil(d.con.SRem(ctx, sAdmin, strconv.FormatInt(id, 10)).Err())
}

func (d *database) DelChat(chat int64) error {
	return asNil(d.con.SRem(ctx, sChat, strconv.FormatInt(chat, 10)).Err())
}

func (d *database) GetAdminExist(chat int64) (bool, error) {
	isMember, err := d.con.SIsMember(ctx, sAdmin, strconv.FormatInt(chat, 10)).Result()
	return isMember, asNil(err)
}

func (d *database) GetAdmins() (out []int64, err error) {
	return d.getIntList(sAdmin)
}

func (d *database) getIntList(hash string) (out []int64, err error) {
	var ints []string
	if ints, err = d.con.SMembers(ctx, hash).Result(); err == nil {
		if l := len(ints); l > 0 {
			out = make([]int64, 0, l)
			for _, a := range ints {
				if id, idErr := strconv.ParseInt(a, 10, 64); err == nil {
					out = append(out, id)
				} else {
					err = idErr
					break
				}
			}
		}
	} else {
		err = asNil(err)
	}
	return
}

func (d *database) GetChatExist(chat int64) (bool, error) {
	exist, err := d.con.SIsMember(ctx, sChat, strconv.FormatInt(chat, 10)).Result()
	return exist, asNil(err)
}

func (d *database) GetChats() ([]int64, error) {
	out, err := d.getIntList(sChat)
	return out, err
}

func (d *database) GetCrawlOffset() (uint, error) {
	out, err := d.con.Get(ctx, kConfOffset).Uint64()
	return uint(out), asNil(err)
}

func (d *database) GetTorrentFiles(id int64) ([]string, error) {
	out, err := d.con.SMembers(ctx, hTorrentFile+strconv.FormatInt(id, 10)).Result()
	return out, asNil(err)
}

func (d *database) GetTorrentImage(id int64) ([]byte, error) {
	var data []byte
	hash, err := d.con.HGet(ctx, hTorrentId, strconv.FormatInt(id, 10)).Result()
	err = asNil(err)
	if err == nil {
		if len(hash) > 0 {
			data, err = d.con.HGet(ctx, hash, fImage).Bytes()
			err = asNil(err)
		} else {
			err = ErrTorrentNotFound
		}
	}
	return data, err
}

func (d *database) GetTorrentMeta(id int64) (map[string]string, error) {
	out, err := d.con.HGetAll(ctx, hTorrentMeta+strconv.FormatInt(id, 10)).Result()
	return out, asNil(err)
}

func (d *database) GetTorrent(name string) (id int64, err error) {
	id = s.InvalidDBId
	var sid string
	if sid, err = d.con.HGet(ctx, hTorrent+name, fIndex).Result(); err == nil {
		id, err = strconv.ParseInt(sid, 10, 64)
	} else {
		err = asNil(err)
	}
	return
}

func (d *database) UpdateCrawlOffset(offset uint) error {
	return d.con.Set(ctx, kConfOffset, strconv.FormatUint(uint64(offset), 10), 0).Err()
}

func (d *database) MGetTorrents() (tt []s.DBTorrent, err error) {
	var tMap map[string]string
	if tMap, err = d.con.HGetAll(ctx, hTorrentId).Result(); asNil(err) == nil {

		for sid, hKey := range tMap {
			id, _ := strconv.ParseInt(sid, 10, 64)
			t := s.DBTorrent{Id: id}
			if t.Name, err = d.con.HGet(ctx, hKey, fName).Result(); asNil(err) != nil {
				break
			}
			if t.Data, err = d.con.HGet(ctx, hKey, fData).Bytes(); asNil(err) != nil {
				break
			}
			if t.Image, err = d.con.HGet(ctx, hKey, fImage).Bytes(); asNil(err) != nil {
				break
			}
			tt = append(tt, t)
		}
	}
	err = asNil(err)
	return
}

func (d *database) MPutTorrent(t s.DBTorrent, fs []string) error {
	return d.tx(func(tx redis.Pipeliner) (err error) {
		hKey := hTorrent + t.Name
		if err = d.con.HSet(ctx, hKey, fName, t.Name, fData, t.Data, fImage, t.Image).Err(); err == nil {
			sid := strconv.FormatInt(t.Id, 10)
			l := len(fs)
			if l > 0 {
				fKey := hTorrentFile + sid
				ifs := make([]any, l)
				for i := 0; i < l; i++ {
					ifs[i] = fs[i]
				}
				err = d.con.SAdd(ctx, fKey, ifs...).Err()
			}
			if err == nil {
				if err = d.con.HSet(ctx, hTorrentId, sid, hKey).Err(); err == nil {
					err = d.con.Set(ctx, kTorrentIndex, sid, 0).Err()
				}
			}
		}
		return
	})
}
