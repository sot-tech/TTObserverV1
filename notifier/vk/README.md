# VK.com notifier
Posts wall message into groups.
Notifier type (need to be passed into `notifiers.type` config): `vkcom`
Config file type: `json`

## Configuration structure
- token - string - **user's** implicit flow token received from vk.com with next scopes: photos,wall,groups,offline 
- groupids - array of uint - list of group ids to post wall notifications
- ignoreunchanged - bool - if `true` notify only if there is at least one updated file in torrent
- ignoreregexp - string - regexp to check if torrent name should be ignored
- msg
    - added - string - text literal for `{{.action}}` placeholder if release is new
    - updated - string - text literal for `{{.action}}` placeholder if release updated
    - singleindex - string - text template if only one file in torrent updated. Possible placeholders:
        - `{{.newindexes}}` - index of new file
    - multipleindexes - string - same as `singleindex` but if update more than one file. Possible placeholders:
        - `{{.newindexes}}` - indexes of new files separated by `, ` 
    - replacements - string map - list of literal replacements for `{{.name}}` placeholder
    - n1x - string - message template about anniversary. Possible placeholders:
        - `{{.index}}` - next check index
    - tags - map of string-bool - list of `meta` keys to format #hashtags value of map is flag if current `meta` is multivalued and should be separated by `msg.tagsseparator`
    - tagsseparator - string - separator of multivalued `meta`
    - announce - string - message template about new release. Possible placeholders:
        - `{{.meta.*}}` - value from extracted meta (instead of `*`)
        - `{{.name}}` - name of primary file/directory from torrent
        - `{{.action}}` - literal value from `added` or `updated`
        - `{{.size}}` - pretty size literal of torrent files (1 MiB, 2 GiB...)
        - `{{.filecount}}` - count of all files in torrent
        - `{{.url}}` - direct URL to release torrent
        - `{{.newindexes}}` - info about updated files' indexes formatted by `singleindex` or `multipleindexes`
        - `{{.tags}}` - tags formatted from `msg.tags` config 