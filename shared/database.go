/*
 * BSD-3-Clause
 * Copyright 2020 sot (PR_713, C_rho_272)
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

package shared

import (
	"errors"
	"sync"
)

const InvalidDBId = -1

type DBFactory func(map[string]any) (Database, error)

var (
	dbFactories             = make(map[string]DBFactory)
	dbFactoriesMu           sync.Mutex
	ErrRequiredParameters   = errors.New("required parameters not set")
	ErrUnsupportedOperation = errors.New("unsupported operation")
)

func RegisterFactory(name string, n DBFactory) {
	dbFactoriesMu.Lock()
	defer dbFactoriesMu.Unlock()
	if len(name) == 0 {
		panic("unspecified driver name")
	} else if n == nil {
		panic("unspecified driver ref instance")
	} else {
		dbFactories[name] = n
	}
}

type DBTorrent struct {
	Id          int64
	Name        string
	Data, Image []byte
}

type Database interface {
	AddAdmin(id int64) error
	AddChat(chat int64) error
	AddTorrentImage(id int64, image []byte) error
	AddTorrentMeta(id int64, meta map[string]string) error
	AddTorrent(name string, data []byte, files []string) (int64, error)
	CheckTorrent(id int64) (bool, error)
	Close()
	DelAdmin(id int64) error
	DelChat(chat int64) error
	GetAdminExist(chat int64) (bool, error)
	GetAdmins() ([]int64, error)
	GetChatExist(chat int64) (bool, error)
	GetChats() ([]int64, error)
	GetCrawlOffset() (uint, error)
	GetTorrentFiles(torrent int64) ([]string, error)
	GetTorrentImage(id int64) ([]byte, error)
	GetTorrentMeta(id int64) (map[string]string, error)
	GetTorrent(torrent string) (int64, error)
	UpdateCrawlOffset(offset uint) error
	MGetTorrents() ([]DBTorrent, error)
	MPutTorrent(torrent DBTorrent, files []string) error
}

func Connect(driver string, params map[string]any) (db Database, err error) {
	if len(driver) > 0 && params != nil {
		if fac := dbFactories[driver]; fac != nil {
			db, err = fac(params)
		} else {
			err = errors.New("driver not registered")
		}
	} else {
		err = ErrRequiredParameters
	}
	return
}
