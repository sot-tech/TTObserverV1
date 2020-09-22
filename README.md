# TTObserverV1
Torrent tracker site release watcher and telegram notifier.
Can:

 - Watch tracker site for new torrent releases (it enumerates serial IDs and searches for torrent-like data)
 - Notify Telegram chats about new release
 - Determine pretty name of release from tracker site
 - Fetch and send release poster to telegram's message 

Uses:

 - [op/go-logging (BSD-3)](https://github.com/op/go-logging)
 - [zeebo/bencode (MIT)](https://github.com/zeebo/bencode)
 - [mattn/go-sqlite3 (MIT)](https://github.com/mattn/go-sqlite3)
 - [nfnt/resize (MIT-style)](https://github.com/nfnt/resize)
 - sot-te.ch/GoHTExtractor (BSD-3)
 - sot-te.ch/GoMTHelper (BSD-3) only for telegram
 - [azzzak/vkapi (MIT)](https://github.com/azzzak/vkapi) only for vk.com
 
# Usage
## Quick start

1. Compile sources from `cmd/ttobserver` with `make`
2. Copy example config and database from `conf` to place you want
3. Rename and modify `example.json` with your values
4. Create modules' configs and set in your `example.json`
5. Run

```
./ttobserver /etc/ttobserver.json
```

## Configuration
 - log - file to store error and warning messages
	- file - string - file to store messages
	- level - string - minimum log level to store (DEBUG, NOTICE, INFO, WARNING, ERROR)
 - crawler
	- baseurl - string - base url (`http://site.local`)
	- contexturl - string - torrent context respectively to `base` (`/catalog/%d`, `%d` - is the place to insert id)
	- threshold - uint - number to id's to check in one try. If current id is 1000 and `threshold` set to 3, observer will check 1000, 1001, 1002
    - delay - uint - minimum delay between two checks, real delay is random between value and 2*value
	- anniversary - uint - notify about every N'th release as anniversary
	- metaactions - list of actions to extract meta info release (see GoHTExtractor readme)
    - imagemetafield - string - name of field from extracted by `metaactions` where picture data stored
    - imagethumb - uint - maximum image size (in pixels) to store in db and send through notifiers 
 - announcers - list of notifiers to send release info through
    - type - string - type of notifier, registered in the observer (look to notifier documentation)
    - configpath - string - path to notifier's config file
 - dbfile - string - path to database

## Modules
TTObserver notifies about release only if there is at least one notifier imported in `observer.go`.
When notifier imported, it registers itself into notifiers list and if it's type declared in
`announcers` config list, notificator executes needed commands.
Any notifier has it's own configuration file, so look into `notifier\*` subdirectory fot additional info. 

## Differences between V0 and V1
1. V0 could notify about releases, but also upload torrent to remote transmission server, V1 can't (and not planned)
3. V1 could freely notify any chat or channel, in V0 every chat needs verification by OTP holder
4. V1 could extract name and image from site and sent it within announce, V0 can't
