module github.com/cirias/remote-switch-bot

go 1.13

require (
	github.com/cirias/tgbot v0.0.0-20180905090010-1dc717204812
	github.com/pkg/errors v0.8.1
	github.com/tarm/serial v0.0.0-20180830185346-98f6abe2eb07
	golang.org/x/sys v0.0.0-20191018095205-727590c5006e // indirect
)

replace github.com/cirias/tgbot => ../tgbot
