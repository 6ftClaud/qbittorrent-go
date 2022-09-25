module github.com/6ftclaud/qbittorrent-go

go 1.19

replace github.com/6ftclaud/qbittorrent-go/modules => ./modules

require (
	github.com/pkg/errors v0.9.1
	golang.org/x/net v0.0.0-20220923203811-8be639271d50
)

require (
	github.com/sirupsen/logrus v1.9.0 // indirect
	golang.org/x/sys v0.0.0-20220728004956-3c1f35247d10 // indirect
)
