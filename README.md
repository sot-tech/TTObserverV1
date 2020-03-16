# TTObserverV1
Torrent tracker site release watcher and telegram notifier.
Can:

 - Watch tracker site for new torrent releases (it enumerates serial IDs and searches for torrent-like data)
 - Notify Telegram chats about new release
 - Determine pretty name of release from tracker site
 - Fetch and send release poster to telegram's message 

Uses:

 - [go-logging (BSD-3)](https://github.com/op/go-logging)
 - [Go Bencode (MIT)](https://github.com/zeebo/bencode)
 - [Go-SQLLite3 (MIT)](https://github.com/mattn/go-sqlite3)
 - sot-te.ch/GoMTHelper (BSD-3)
 - sot-te.ch/GoHTExtractor (BSD-3)
 
# Usage
## Quick start

1. Compile sources from `cmd/ttobserver` with `make`
2. Copy example config and database from `conf` to place you want
3. Rename and modify `example.json` with your values
4. Run

```
./ttobserver /etc/ttobserver.json
```

## Configuration

 - log - file to store error and warning messages
	- file - string - file to store messages
	- level - string - minimum log level to store (DEBUG, NOTICE, INFO, WARNING, ERROR)
 - crawler
	- url
	- baseurl - string - base url (`http://site.local`)
	- contexturl - string - torrent context respectively to `base` (`/catalog/%d`, `%d` - is the place to insert id)
	- threshold - uint - number to id's to check in one try. If current id is 1000 and `threshold` set to 3, observer will check 1000, 1001, 1002
    - delay - uint - minimum delay between two checks, real delay is random between value and 2*value
	- anniversary - uint - notify about every N'th release as anniversary
	- metaactions - list of actions to extract meta info release (see GoHTExtractor readme)
    - imagemetafield - string - name of field from extracted by `metaactions` where picture data stored
 - telegram
    - apiid - int - API ID received from [telegram](https://my.telegram.org/apps)
    - apihash - string - API HASH received from [telegram](https://my.telegram.org/apps)
    - bottoken - string
    - dbpath - string - TDLib's DB path (used to store session data)
    - filestorepath - string - TDLib's file store path (can be temporary)
    - otpseed - string - base32 encoded random bytes to init TOTP (for admin auth)
    - msg
        - error - string - message prepended to error
        - auth - string - response to `/setadmin` or `/rmadmin` if unauthorized (OTP invalid)
        - cmds - command responses
            - start - string - response to `/start` command
            - attach - string - response to `/attach` command if succeeded
            - detach - string - response to `/detach` command if succeeded
            - setadmin - string - response to `/setadmin` command if succeeded
            - rmadmin - string - response to `/rmadmin` command if succeeded
            - unknown - string - response to unsupported command
        - state - string - response template to `/state` command. Possible placeholders:
            - `{{.admin}}` - is this chat has admin privilegies
            - `{{.watch}}` - is this chat subscribed to announces
            - `{{.index}}` - next check index
        - added - string - text literal for `{{.action}}` placeholder if release is new
        - updated - string - text literal for `{{.action}}` placeholder if release updated
        - replacements - string map - list of literal replacements for `{{.name}}` placeholder
        - n1x - string - message template about anniversary. Possible placeholders:
            - `{{.index}}` - next check index
        - announce - string - message template about new release. Possible placeholders:
            - `{{.meta.*}}` - value from extracted meta (instead of `*`)
            - `{{.name}}` - name of primary file/directory from torrent
            - `{{.action}}` - literal value from `added` or `updated`
            - `{{.size}}` - pretty size literal of torrent files (1 MiB, 2 GiB...)
            - `{{.filecount}}` - count of all files in torrent
            - `{{.url}}` - direct URL to release torrent
 - dbfile - string - path to database

### Admins
In TTObserverV1, administrators are (currently) only chats, who can list chats and other admins, which subscribed to announce bot.
To become admin, chat should call `/setadmin 123456` in telegram, where 123456 - is an OTP, seeded by `adminotpseed`,
to revoke admin call `/rmadmin`,
to list admins - `/lsadmins`,
to list chats - `/lschats`.

## Differences between V0 and V1
1. V0 could notify about releases, but also upload torrent to remote transmission server, V1 can't (and not planned)
2. V0 could contain plugins for notifying custom services, V1 can't (and not planned)
3. V1 could freely notify any chat or channel, in V0 every chat needs verification by OTP holder
4. V1 could extract name and image from site and sent it within announce, V0 can't
5. V1 could notify admins if target is unavailable, V0 can't
