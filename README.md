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
 - [Go-telegram-bot-api (MIT)](https://github.com/go-telegram-bot-api/telegram-bot-api)
 - [gOTP (MIT)](https://github.com/xlzd/gotp)
 - [Go-SQLLite3 (MIT)](https://github.com/mattn/go-sqlite3)
 
# Usage
## Quick start
1. Compile sources from `cmd/ttobserver` with `make`
2. Copy example config and database from `conf` to place you want
3. Rename and modify `example.json` with your values
4. Run

```
/src/ttobserverv1/cmd/ttobserver/ttobserver /etc/ttobserver.json
```

## Configuration

 - log - file to store error and warning messages
	- file - string - file to store messages
	- level - string - minimum log level to store (DEBUG, NOTICE, INFO, WARNING, ERROR)
 - crawler
	- url
		- base - string - base url (`http://site.local`)
		- torrent - string - torrent context respectively to `base` (`/catalog/%d`, `%d` - is the place to insert id)
		- extractnameactions - list of actions to extract pretty name of release
			- action - string - `go`, `extract`, `check`, `return` (see below)
			- param - string - parameter for action (see below)
		- extractimageactions - list of actions to extract image (structure is the same as `extractnameactions`)
	- threshold - uint - number to id's to check in one try. If current id is 1000 and `threshold` set to 3, observer will check 1000, 1001, 1002
	- errorthreshold - uint - number of connection errors to to notify admins about target error
	- delay - uint - minimum delay between two checks, real delay is random between value and 2*value
 - tryextractprettyname - bool - extract or not pretty name (do `extractnameactions`)
 - tryextractimage - bool - extract or not release image (do `extractimageactions`)
 - sizethreshold - float - size diff between current torrest and previous to notify. If diff less than `sizethreshold`, observer will not notify chats
 - telegramtoken - string - telegram bot token, recieved from @BotFather
 - adminotpseed - string - base32-encoded random bytes to init TOTP
 - dbfile - string - path to database
 - msg
	- announce - message Markdown template for about new releases. If empty - announce disabled. Possible placeholders:
		- `${action}` - see `added`, `updated` below
		- `${name}` - name of release (if pretty name not empty, it formated by `Pretty Name (name from torrent file)`)
		- `${size}` - pretty size (10 Mib, 20.6 GiB...)
		- `${url}` - direct url to torrent file in the site
		- `${publisherurl}` - Publisher URL from torrent file or URL to page pretty name got from (if enabled)
	- n1000 - string - message Markdown template about anniversary (every 1000th id)
	- added - string - value to set in `announce` template if torrent is new release (name from torrent file not found in DB)
	- updated - string - value to set in `announce` template if torrent was updated (name from torrent file not found in DB)
	- error - string - message template to send if commands failed or target is unavailable
	- replacements - map of strings - string blocks to replace in `${name}` (i.e. `"_"` replace with `" "` etc.)
	- cmds - command responses
		- start - string - response to `/start` command
		- attach - string - response to `/attach` command if succeeded
		- detach - string - response to `/detach` command if succeeded
		- setadmin - string - response to `/setadmin` command if succeeded
		- rmadmin - string - response to `/rmadmin` command if succeeded
		- state - string - response template to `/state` command. Possible placeholders:
			- `${admin}` - is this chat has admin privilegies
			- `${watch}` - is this chat subscribed to announces
			- `${index}` - next check index
		- auth - string - response to `/setadmin` or `/rmadmin` if unauthorized (OTP invalid)
		- unknown - string - response to unsupported command

### Extract name and image actions
If `tryextractprettyname` or `tryextractimage`  set to `true`, observer tries to get value from tracker site.
Main idea was to find needed page from torrent site by enumerating last records and cheking if particular page contains
link to torrent release (which found by enumerating ids).
Mechanism of getting data is the same  both for pretty name and image.


 - `go` - just get data from URL (HTTP GET), `param` - is context respectively to `crawler.url.base` to get data from.
 If there is no HTTP or carrier errors, next action is called
 Possible placeholders for `param`:
	- `${arg}` - value recieved from parent call (if any)
	- `${torrent}` - formatted `crawler.url.torrent` context (with torrent id)
 - `check` - checks data, recieved from parent call, if it contains data with regexp, provided in `param`, or if `param` is empty, 
 just checks that data not empty. If check succeded, next action is called. Possible placeholders for `param`:
	- `${torrent}` - formatted `crawler.url.torrent` context (with torrent id)
 - `extract` - extract substring from data, recieved from parent call with regexp provided in `param`. 
 If there is more than one match, action iterates over every match and send it to next action until the end, or until `return`
 action is reached.
 If there is more than one group in match, only first group will be sent to next action.
 Possible placeholders for `param`:
	- `${torrent}` - formatted `crawler.url.torrent` context (with torrent id)
 - `return` - stores data, recieved from parent action and stops iteration of all extracts (if any)
 

Actions described in `crawler.url.extractnameactions` and `crawler.url.extractimageactions` are executing sequentially,
but every next action is executing inside current action. If we have `go - extract - check - return` sequence, it means, that
data which have been recieved by `go` action transmitted to `extract` action, then every data, that extracted by `extract` transmitted to
`check`, and if `check` susseded, `return` is called. I.e. we have next actions with params:

```
{
	"action": "go",
	"param": "/catalog"
},
{
	"action": "extract",
	"param": "<a href=\"(.*?)\">"
},
{
	"action": "go",
	"param": "/releases/${arg}"
},
{
	"action": "check",
	"param": "<a class=\"button some_button\" href=\"\\Q${torrent}\\E\">"
},
{
	"action": "extract",
	"param": "<div class=\"main_title\">(.*?)</div>"
},
{
	"action": "return",
	"param": ""
}
```

1. Program goes to `/catalog` context of `crawler.url.base` site, gets all page content (let's say simple html page),
and sends it to `extract` action as argument
2. Program extracts all substrings of data from `go` action, that match `<a href="(.*?)">` regexp and for _every_ match, calls
`go` action with substring as an argument (let's say `release1`, then `release2`)
3. Program goes to page of `crawler.url.base` site, with context `/releases/release1` (`${arg}` placeholder in `param`),
gets all page data and sends it to `check` action as argument, then again for `/releases/release2`, if `return` did not reached
4. Program checks if data from `go` action (3) contains substring with `<a class="button some_button" href="\Qsome/torrent/200\E">` (`${torrent}` placeholder in `param`) regexp,
if so, calls `extract` with data, recieved from `go` action (3)
5. Program extracts substring from data that match `<div class=\"main_title\">(.*?)</div>` regexp and calls `return`
6. Program stops iteration of `extract` action (2) and stores data.

### Admins
In TTObserverV1, administrators are (currently) only chats, that recieve messages about target unavailable.
To become admin, chat should call `/setadmin 123456` in telegram, where 123456 - is an OTP, seeded by `adminotpseed`,
to revoke admin call `/rmadmin 123456`.

## Differences between V0 and V1
1. V0 could notify about releases, but also upload torrent to remote transmission server, V1 can't (and not planned)
2. V0 could contain plugins for notifying custom services, V1 can't (and not planned)
3. V1 could freely notify any chat or channel, in V0 every chat needs verification by OTP holder
4. V1 could extract name and image from site and sent it within announce, V0 can't
5. V1 could notify admins if target is unavailable, V0 can't
