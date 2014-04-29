#!/bin/sh -e
# This file copies the needed source files from the installed Go sources.
eval $(go env | grep GOROOT=)
DN=$GOROOT/src/pkg/archive/zip
cd $(dirname $0)
for nm in register struct; do
    sed -e 's/^package zip$/package zip2/' $DN/${nm}.go > ${nm}.go
done
sed -e 's/^package zip$/package zip2/;s!import (!import (\n\t"archive/zip"\n!' \
    $DN/writer.go > writer.go
gofmt -r 'ErrAlgorithm -> zip.ErrAlgorithm' -w writer.go
