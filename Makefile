#!/usr/bin/env gmake -f

GOOPTS=CGO_ENABLED=0
BUILDOPTS=-ldflags="-s -w" -a -gcflags=all=-l

all: clean build

build:
	${GOOPTS} go build ${BUILDOPTS} -o aleesa-irc-go types.go globals.go collection.go irc-user-mode.go \
	    settings-db-util.go util.go msg-parsers.go aleesa-irc-go-lib.go main.go

clean:
	go clean

wipe:
	go clean
	rm -rf go.{mod,sum}

prep:
	go mod init main
	go mod tidy -compat=1.18

# vim: set ft=make noet ai ts=4 sw=4 sts=4:
