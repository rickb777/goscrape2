# goscrape - create offline browsable copies of websites

[![GoDoc](https://img.shields.io/badge/api-Godoc-blue.svg)](http://pkg.go.dev/github.com/rickb777/goscrape2)
[![Go Report Card](https://goreportcard.com/badge/github.com/rickb777/goscrape2)](https://goreportcard.com/report/github.com/rickb777/goscrape2)
[![Build](https://github.com/rickb777/goscrape2/actions/workflows/go.yaml/badge.svg?branch=main)](https://github.com/rickb777/goscrape2/actions)
[![Issues](https://img.shields.io/github/issues/rickb777/goscrape2.svg)](https://github.com/rickb777/goscrape2/issues)

A web scraper built with [Go](https://go.dev/). It downloads the content of a website and allows it to be archived and
read offline.

## Features

Features and advantages over existing tools like wget, httrack, Teleport Pro:

* Free and open source
* Available for all platforms that Go supports
* Files are downloaded concurrently as required
* Downloaded asset files are skipped in a new scraper run if unchanged
* Redirected URLs don't duplicate downloads
* JPEG and PNG images can be converted down in quality to save disk space
* Excluded URLS will not be fetched (unlike [wget](https://savannah.gnu.org/bugs/?20808))
* No incomplete temporary files are left on disk
* Assets from external domains are downloaded automatically
* Sane default values
* Built-in webserver provides easy local access to the downloaded files
* Webserver replays redirections just like the origin server
* Supports logging and logfile rotation - can run as a long-lived service

## Limitations

* No GUI version, console only

## Installation

Compile the latest release from source:

```
go install github.com/rickb777/goscrape2@latest
```

Compiling the tool from source code needs to have a recent version of [Go](https://go.dev/) installed (v1.23 or later).

Alternatively, grab the source code and build it locally.

```
git clone https://github.com/rickb777/goscrape2
cd goscrape2
go install tool
mage
./goscrape2
```

## Usage

Scrape a website by running

```
goscrape2 http://website.com/interesting/stuff
```

To serve the downloaded website directory in a locally-run webserver, use

```
goscrape2 --serve website.com
```

## Options

Options can use single or double dash (e.g. `-v` or `--v`).

```
Usage:
  ./goscrape2 [options] [<url> ...]

  -H value
    	"name:value" HTTP header to use for scraping (can be repeated)
  -concurrency int
    	the number of concurrent downloads (default 1)
  -connect duration
    	time limit (with units, e.g. 1s) for each HTTP request to connect (default 30s)
  -cookies string
    	file containing the cookie content
  -depth int
    	download depth limit (default unlimited)
  -dir directory
    	directory to write files to and to serve files from
  -i regular expression
    	only include URLs that match a regular expression (can be repeated)
  -imagequality int
    	image quality reduction, minimum 1 to maximum 99 (re-encoding disabled by default)
  -laxage duration
    	adds to the 'expires' timestamp specified by the origin server, or creates one if absent.
    	If the origin is too conservative, this helps when doing successive runs; a negative value causes
    	revalidation instead.
  -log string
    	output log file; use "-" for stdout (default "-")
  -loopdelay duration
    	delay (with units, e.g. 1s) used between any two downloads
  -port int
    	port to use for the webserver (default 8080)
  -savecookiefile string
    	file to save the cookie content
  -serve
    	serve the website using a webserver.
    	Scraping will happen only on demand using the first URL you provide.
  -timeout duration
    	overall time limit (with units, e.g. 31s) for each HTTP request to connect and read the response
    	This is dependent on -connect and will always be greater than that timeout. (default 1m0s)
  -tries int
    	the number of tries to download each file if the server gives a 5xx error (default 1)
  -user string
    	user[:password] to use for HTTP authentication
  -useragent string
    	user agent to use for scraping
  -v	verbose output
  -x regular expression
    	exclude URLs that match a regular expression (can be repeated)
  -z	debug output
```

## Environment

These environment variables may be set

* GOSCRAPE_URLS - Adds URLs to the list to process (space separated)
* GOSCRAPE_INCLUDE - Adds regular expressions to the -i include list (space separated)
* GOSCRAPE_EXCLUDE - Adds regular expressions to the -x exclude list (space separated)
* HTTP_PROXY, HTTPS_PROXY - Controls the proxy used for outbound connections: either a complete URL or a "host[:port]",
  in which case the "http" scheme is assumed. Authentication can be included with a complete URL.
* NO_PROXY - A comma-separated list of values specifying hosts that should be excluded from proxying. Each value is
  represented by an IP address prefix (1.2.3.4), an IP address prefix in CIDR notation (1.2.3.4/8), a domain name, or a
  special DNS label (*). An IP address prefix and domain name can also include a literal port number (1.2.3.4:80). A
  domain name matches that name and all subdomains. A domain name with a leading "." matches subdomains only. For
  example "foo.com" matches "foo.com" and "bar.foo.com"; ".y.com" matches "x.y.com" but not "y.com". A single
  asterisk (*) indicates that no proxying should be done.

## Cookies

Cookies can be passed in a file using the `--cookiefile` parameter and a file containing
cookies in the following format:

```
[{"name":"user","value":"123"},{"name":"sessioe","value":"sid"}]
```

## Conditional requests: ETags and last-modified

HTTP uses ETags to tag the version of each resource. Each ETag is a hash constructed by the server somehow. Also, each
file usually has a last-modified date.

`goscrape2` will use both of these items provided by the server to reduce the amount of work needed if multiple sessions
of downloading are run on the same start URL. Any file that is not modified doesn't need to be downloaded more than
once. ETags and other metadata are stored in the state cache.

## State Cache

`goscrape2` keeps its state database in `~/.local/state/goscrape-cache.txt`, which is dependent on the user that is
running `goscrape2` of course. This is only read in when `goscrape2` starts; any external edits will be overwritten
whilst `goscrape2` is running.

Provided `goscrape2` has been stopped first, the cached files (see `-dir`) and state database can be safely moved/copied
between servers, e.g. using `rsync` so that the files retain their timestamps.

The state database is a text file that can be concatenated, in which case any duplicates are resolved by selecting
whichever comes last. It can also be deleted, in which case future revalidation requests to the origin server will be
much less network-efficient. In either case, it will be rebuilt when `goscrape2` is restarted, provided the origin
server is still reachable.

The state database is automatically purged if the output directory doesn't exist when `goscrape2` is started.

## Logfile Rotation

For a long-running service, the logfile should be periodically rotated to avoid filling up the disk. `goscrape2` is
designed to work well with Linux `logrotate`, for example using `-log /var/log/goscrape.log` and this configuration in
`/etc/logrotate.d/goscrape2`

```
`/var/log/goscrape.log` {
  daily
  notifempty
  minsize 1M
  missingok
  rotate 28
  postrotate
    pkill -hup goscrape2
  endscript
  compress
  delaycompress
  nocreate
}
```

Daily, logrotate will check whether the logfile has grown too big and, if so, move it then poke `goscrape2` with SIGHUP.

## SystemD Service

Example SystemD service and configuration files are in the `systemd/` folder, to be deployed as

* `/usr/sbin/goscrape2` binary
* `/var/lib/goscrape` directory tree
* `/var/log/goscrape.log` logfile
* `/etc/default/goscrape.conf` default configuration
* `/etc/logrotate.d/goscrape` log rotation
* `/etc/systemd/system/goscrape.service` service definition

You will need to understand SystemD to use these template files.

## Thanks

This tool was derived from github.com/cornelk/goscrape with thanks to the developers.
