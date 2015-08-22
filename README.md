# Rapid Response Proxy [![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](http://godoc.org/github.com/8legd/RRP)

Speed up your web service / api requests with Rapid Response Proxy

## Overview

Rapid Response Proxy (RRP) acts as a proxy for web service / api requests with the aim of speeding up the time taken to transport each request and return a response.

At the moment this is facilitated through batch processing of requests but the goal of the project is broader and aims to explore other strategies to provide a more "rapid response".

### Batch processing

RRP provides support for batch processing HTTP requests with a syntax similar to [Google Cloud's batch processing] (https://cloud.google.com/storage/docs/json_api/v1/how-tos/batch) which was in turn based on the OData syntax (http://www.odata.org/documentation/odata-version-3-0/batch-processing/).

A batch request is a single HTTP request which acts as a container using the multipart/mixed content type to aggregate individual HTTP requests as parts.

## Installation
Like most Go programs RRP runs as a self contained binary. For distributions see [releases] (https://github.com/8legd/RRP/releases)

### Windows
The recommended Windows setup is to run RRP as a service using [NSSM] (http://www.nssm.cc/)

1. Install [NSSM] (http://www.nssm.cc/download).
2. Download the binary distribution from [releases] (https://github.com/8legd/RRP/releases)
3. Setup RRP to run as a service through NSSM:

`nssm install RRP`

![3.1](http://d2jyigzo9dzbko.cloudfront.net/8legd/RRP/doc/nssm/1.jpg)
![3.2](http://d2jyigzo9dzbko.cloudfront.net/8legd/RRP/doc/nssm/2.jpg)
![3.3](http://d2jyigzo9dzbko.cloudfront.net/8legd/RRP/doc/nssm/3.jpg)
![3.4](http://d2jyigzo9dzbko.cloudfront.net/8legd/RRP/doc/nssm/4.jpg)
![3.5](http://d2jyigzo9dzbko.cloudfront.net/8legd/RRP/doc/nssm/5.jpg)

Log files can be rotated through a scheduled task :

`nssm rotate RRP`
