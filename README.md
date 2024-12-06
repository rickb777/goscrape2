# goscrape - create offline browsable copies of websites

[![Build status](https://github.com/rickb777/goscrape2/actions/workflows/go.yaml/badge.svg?branch=main)](https://github.com/rickb777/goscrape2/actions)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/rickb777/goscrape2)
[![Go Report Card](https://goreportcard.com/badge/github.com/rickb777/goscrape2)](https://goreportcard.com/report/github.com/rickb777/goscrape2)

A web scraper built with [Go](https://go.dev/). It downloads the content of a website and allows it to be archived and read offline.

## Features

Features and advantages over existing tools like wget, httrack, Teleport Pro:

* Free and open source
* Available for all platforms that Go supports
* Files are downloaded concurrently as required
* Downloaded asset files are skipped in a new scraper run if unchanged
* JPEG and PNG images can be converted down in quality to save disk space
* Excluded URLS will not be fetched (unlike [wget](https://savannah.gnu.org/bugs/?20808))
* No incomplete temporary files are left on disk
* Assets from external domains are downloaded automatically
* Sane default values
* Built-in webserver provides easy local access to the downloaded files

## Limitations

* No GUI version, console only

## Installation

There are 2 options to install `goscrape2`:

1. Download and unpack a binary release from [Releases](https://github.com/rickb777/goscrape2/releases)

or

2. Compile the latest release from source:

```
go install github.com/rickb777/goscrape2@latest
```

Compiling the tool from source code needs to have a recent version of [Go](https://go.dev/) installed.

## Usage

Scrape a website by running
```
goscrape2 http://website.com/interesting/stuff
```

To serve the downloaded website directory in a local run webserver use
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
  -proxy string
    	HTTP proxy to use for scraping
  -savecookiefile string
    	file to save the cookie content
  -serve
    	serve the website using a webserver.
    	Scraping will happen only on demand using the first URL you provide.
  -timeout duration
    	time limit (with units, e.g. 1s) for each HTTP request to connect and read the response
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

 * GOSCRAPE_URLS adds URLs to the list to process (use a space separated list)
 * GOSCRAPE_INCLUDE adds regular expressions to the -i include list (use a space separated list)
 * GOSCRAPE_EXCLUDE adds regular expressions to the -x exclude list (use a space separated list)

## Cookies

Cookies can be passed in a file using the `--cookiefile` parameter and a file containing
cookies in the following format:

```
[{"name":"user","value":"123"},{"name":"sessioe","value":"sid"}]
```

## Conditional requests: ETags and last-modified

HTTP uses ETags to tag the version of each resource. Each ETag is a hash constructed by 
the server somehow. Also, each file usually has a last-modified date.

`goscrape2` will use both of these items provided by the server to reduce the amount of
work needed if multiple sessions of downloading are run on the same start URL. Any file 
that is not modified doesn't need to be downloaded more than once.

A small database containing ETags is stored in `~/.config/goscrape2-etags.txt`, which can
be manually deleted to purge this cache. It is automatically purged if the output directory 
doesn't exist when `goscrape2` is started.

## Thanks

This tool was derived from github.com/cornelk/goscrape with thanks to the developers.
