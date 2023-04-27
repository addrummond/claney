#!/bin/sh
set -e
cd ../../
go install
cd js/react_example
$(go env GOPATH)/bin/claney -input routes -output-prefix "export const routes = " -output routes.js
