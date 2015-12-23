#!/bin/sh

set -e

go test -c
exec ./gocalm.test -test.v -logtostderr=true -stderrthreshold=INFO -v=1
