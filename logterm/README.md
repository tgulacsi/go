# logterm
Logterm is a tail enhanced with Prometheus scraper, or
a Prometheus metric viewer accompanied by a tail.

Tails logterm's stdin, and scrapes Prometheus metrics from `-addr`,
and shows the metrics given as arguments.

# Usage

    start-my-parallel-mssql-dump 2>&1 | logterm -addr=:9100 mssql_dump_records

To show the export logs with the export metrics.

For example

	node_exporter &
	journalctl -f | logterm -addr=:9100 'node_cpu{mode="system"}'

will start [node_exporter](https://github.com/prometheus/node_exporter),
and show the cpu statistics at top of the screen, and the system logs
under.

# Install

    go get -u github.com/tgulacsi/go/logterm
	go generate github.com/tgulacsi/go/logterm
	go install github.com/tgulacsi/go/logterm
