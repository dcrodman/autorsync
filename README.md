# autorsync

A simple utility to automatically `rsync` directories in response to file change events. Similar to
[lsyncd](https://github.com/axkibe/lsyncd), just loads simpler and less sophisticated.

## Installation

    go get github.com/dcrodman/autorsync

## Usage

```
Usage of autorsync:
  -config string
        Config file (default is .autorsync)
  -logfile string
        Log file to use (default is stdout)
  -rsync string
        rsync executable to use (default /usr/bin/rsync)
```

By default, `autorsync` looks for a config file called `.autorsync` in the current working directory. This 
is expected to be a JSON-formatted file containing any settings for the tool as well as a definition of which
directories to map.

| Key | Description |
| --- | ----------- |
| settings | Object for settings that control `autorsync`'s behavior |
| settings.interval | The frequency with which rsync will run after a change |
| settings.rsync_args | Additional arguments to pass to `rsync` |
| mappings | Array of definitions for which files/directories to sync |
| mappings[].source | Source directory to sync. Same rules as the `SRC` arg in `rsync` |
| mappings[].target | Destination for the sync. Same rules as the `DEST` arg in `rsync` |
| mappings[].exclusions | Directories in source that should be ignored while syncing | 

Environment variables can be used in `settings.rsync_args`, `mappings.source`, and `mappings.target`; their values
will be set from your current session.

Example:
```
{
    "settings": {
        "interval": "3s",
        "rsync_args": [
            "--dry-run",
            "-e 'ssh -i ~/.ssh/some_key'"
        ]
    },
    "mappings": [
        {
            "source": "$HOME/testdir",
            "target": "127.0.0.1:/tmp/testdir",
            "exclusions": [
                ".ignoreme/",
            ]
        }
    ]
}
```
