# logterm
Logterm is a tail enhanced with Prometheus scraper, or
a Prometheus metric viewer accompanied by a tail.

Tails logterm's stdin, and scrapes Prometheus metrics from `-addr`,
and shows the metrics given as arguments.

# Install

    go get -u github.com/tgulacsi/go/logterm
	go generate github.com/tgulacsi/go/logterm
	go install github.com/tgulacsi/go/logterm
