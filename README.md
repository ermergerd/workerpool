workerpool
==========

[![GoDoc Reference](https://godoc.org/github.com/ermergerd/workerpool?status.svg)](https://godoc.org/github.com/ermergerd/workerpool)
[![Build Status](https://travis-ci.org/ermergerd/workerpool.svg?branch=master)](https://travis-ci.org/ermergerd/workerpool)
[![Coverage Status](https://coveralls.io/repos/ermergerd/workerpool/badge.svg?branch=master)](https://coveralls.io/r/ermergerd/workerpool?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/ermergerd/workerpool)](https://goreportcard.com/report/github.com/ermergerd/workerpool)

Description
-----------

workerpool is an implementation of a worker pool of goroutines that can execute a simple function of type func().
The package provides the ability to pre-spawn the workers (to cut down on runtime overhead) as well as the ability to
periodically cull them at a configurable rate.

Installation
------------

This package can be installed with the go get command:

    go get github.com/ermergerd/workerpool
    
See the godoc for futher information on the API and how to use it.
