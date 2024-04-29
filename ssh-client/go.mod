module ssh-client

go 1.22.2

require (
	github.com/mattn/go-tty v0.0.5
	golang.org/x/crypto v0.22.0
)

require (
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.19.0 // indirect
)

replace github.com/mattn/go-tty => github.com/edp1096/go-tty v0.0.0-20240427140603-5244c02fcc96
