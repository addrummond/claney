#!/bin/sh
set -e
cd ../../
go install
cd js/react_example
$(go env GOPATH)/bin/claney -input routes | (printf "export const routes = " && cat && echo ";") > routes.js
