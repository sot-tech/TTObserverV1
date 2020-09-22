# Telegram notifier
Notifier type (needed to be passed into `notifiers.type` config): `telegram`
Config file type: `json`

## Configuration structure
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
        - `{{.admin}}` - is this chat has admin privileges
        - `{{.watch}}` - is this chat subscribed to announce
        - `{{.index}}` - next check index
    - added - string - text literal for `{{.action}}` placeholder if release is new
    - updated - string - text literal for `{{.action}}` placeholder if release updated
    - singleindex - string - text template if only one file in torrent updated. Possible placeholders:
        - `{{.newindexes}}` - index of new file
    - multipleindexes - string - same as `singleindex` but if update more than one file. Possible placeholders:
        - `{{.newindexes}}` - indexes of new files separated by `, ` 
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
        - `{{.newindexes}}` - info about updated files' indexes formatted by `singleindex` or `multipleindexes` 
            
### Feedback
This notifier has feedback functionality to be controlled from telegram.

Public commands:

 - `/attach` - subscribe to announce messages
 - `/detach` - unsubscribe
 - `/state` - show message formatted by `msg.state` config

#### Admin commands
 - `/setadmin 123456` - become an admin, 123456 - is an OTP, seeded by `adminotpseed`,
 - `/rmadmin` - revoke admin rights
 - `/lsadmins` - list admin chats
 - `/lschats` - list subscribers
 - `/lsreleases QUERY` - search release by name, where `QUERY` - is sql `like` search string
 - `/uploadposter 123 https://some/url` - try to update release (123 - is release ID from `lsreleases`) poster by downloading image from specified URL
