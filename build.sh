#!/usr/bin/env bash
go generate 
go get -d ./... 
go build . 
go install .
