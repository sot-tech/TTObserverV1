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

package main

import (
	"sot-te.ch/TTObserverV1"
	s "sot-te.ch/TTObserverV1/shared"
	"sot-te.ch/TTObserverV1/shared/redis"
	"sot-te.ch/TTObserverV1/shared/sqlite"
)

func migrate(tt *TTObserver.Observer) {
	var err error
	var oldDb, newDb s.Database
	if oldDb, err = s.Connect(sqlite.DBDriver, tt.DB.Parameters); err != nil {
		logger.Fatal("!Unable to connect to old database", err)
	}
	defer oldDb.Close()
	if newDb, err = s.Connect(redis.DBDriver, tt.DB.Parameters); err != nil {
		logger.Fatal("!Unable to connect to new database", err)
	}
	defer newDb.Close()
	logger.Info("+ Connection succeeded")

	if offset, err := oldDb.GetCrawlOffset(); err != nil {
		logger.Fatal("! Unable to get offset", err)
	} else if err := newDb.UpdateCrawlOffset(offset); err != nil {
		logger.Fatal("! Unable to migrate offset", err)
	}

	logger.Info("+ Offset migrated")

	if chats, err := oldDb.GetChats(); err != nil {
		logger.Fatal("! Unable to get chats", err)
	} else {
		if len(chats) > 0 {
			for _, c := range chats {
				if err = newDb.AddChat(c); err != nil {
					logger.Fatal("! Unable to migrate chats", err)
				}
			}
		}
	}

	logger.Info("+ Chats migrated")

	if admins, err := oldDb.GetAdmins(); err != nil {
		logger.Fatal("! Unable to get admins", err)
	} else {
		if len(admins) > 0 {
			for _, c := range admins {
				if err = newDb.AddAdmin(c); err != nil {
					logger.Fatal("! Unable to migrate admins", err)
				}
			}
		}
	}

	logger.Info("+ Admins migrated")

	if ts, err := oldDb.MGetTorrents(); err != nil {
		logger.Fatal("! Unable to get torrents", err)
	} else {
		if len(ts) > 0 {
			for _, t := range ts {
				if fs, err := oldDb.GetTorrentFiles(t.Id); err != nil {
					logger.Fatal("! Unable to get torrent ", t.Id, " files", err)
				} else if err := newDb.MPutTorrent(t, fs); err != nil {
					logger.Fatal("! Unable to migrate torrent ", t.Id, err)
				} else if meta, err := oldDb.GetTorrentMeta(t.Id); err != nil {
					logger.Fatal("! Unable to get torrent ", t.Id, " meta", err)
				} else if err = newDb.AddTorrentMeta(t.Id, meta); err != nil {
					logger.Fatal("! Unable to migrate torrent ", t.Id, " meta", err)
				}
				logger.Info("Torrent ", t.Id, " migrated")
			}
		} else {
			logger.Warning("- There is no torrents to migrate")
		}
	}
	logger.Info("+ Migration complete")
}
