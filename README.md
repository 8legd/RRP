# Rapid Response Proxy [![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](http://godoc.org/github.com/8legd/RRP)

Speed up your web service / api requests with Rapid Response Proxy

## Overview

Rapid Response Proxy (RRP) acts as a proxy for web service / api requests with the aim of speeding up the time taken to transport each request and return a response

At the moment this is facilitated through batch processing of requests but the goal of the project is broader and aims to explore other strategies to provide a more "rapid response"

A list of client libraries to RRP are maintained in the [Wiki](https://github.com/8legd/RRP/wiki/Client-Libraries)

### Batch processing

RRP provides support for batch processing HTTP requests with a syntax similar to [Google Cloud's batch processing](https://cloud.google.com/storage/docs/json_api/v1/how-tos/batch) which was in turn based on the [OData syntax](http://www.odata.org/documentation/odata-version-3-0/batch-processing/)

A batch request is a single HTTP request which acts as a container using the multipart/mixed content type to aggregate individual HTTP requests as parts

Below is an example of what the raw multipart/mixed batch request looks like.

NOTE:
  * The `x-rrp-timeout` header specifies a timeout in seconds which is applied to all the requests contained in the batch
  * The individual requests making up the batch are included using the `application/http` content type
  * The individual requests must contain a `Forwarded` header specifying what protocol RRP is to use when making the request (http/https)


```
POST http://127.0.0.1:8000/batch/multipartmixed HTTP/1.1
Content-Type: multipart/mixed; boundary=----2272f4a9-1e7c-4ef6-9b60-caddcf53de65
x-rrp-timeout: 20
Host: 127.0.0.1:8000
Content-Length: 550
Expect: 100-continue
Connection: Keep-Alive

------2272f4a9-1e7c-4ef6-9b60-caddcf53de65
Content-Type: application/http

POST /route1 HTTP/1.1
Host: www.example1.com
Content-Type: text/xml; charset=utf-8
Content-Length: 40
Forwarded: proto=https

<XMLContent name="Bob"></XMLContent>

------2272f4a9-1e7c-4ef6-9b60-caddcf53de65
Content-Type: application/http

POST /route2 HTTP/1.1
Host: www.example2.com
Content-Type: application/json; charset=utf-8
Content-Length: 38
Forwarded: proto=https

{JSONContent: {"name": "Alice"}}

------2272f4a9-1e7c-4ef6-9b60-caddcf53de65--
```

The batch response is returned in a similar fashion again using the multipart/mixed content type to act as a container for the individual HTTP responses which are returned in the same sequence as their associated requests.

NOTE:
  * Errors in transport are returned as HTTP status messages. For example timeouts are returned as 400 (Bad Request) errors e.g. `HTTP/1.1 400 net/http: timeout awaiting response headers`

```
HTTP/1.1 200 OK
Content-Type: multipart/mixed; boundary=9c1c0d27a7292aa27aec4ea3c9eb8f125620686a87bc38d036911c014e36
Date: Sat, 15 Aug 2015 19:02:10 GMT
Transfer-Encoding: chunked

5200
--9c1c0d27a7292aa27aec4ea3c9eb8f125620686a87bc38d036911c014e36
Content-Type: application/http

HTTP/1.1 200 OK
Cache-Control: no-store, no-cache, must-revalidate, post-check=0, pre-check=0
Content-Type: text/html; charset=UTF-8
Date: Sat, 15 Aug 2015 19:07:11 GMT
Expires: Thu, 14 Nov 1981 08:52:00 GMT
Pragma: no-cache
Server: Apache/2.2.15 (CentOS)
Vary: Accept-Encoding

<XMLContent result="Hello Bob"></XMLContent>

--9c1c0d27a7292aa27aec4ea3c9eb8f125620686a87bc38d036911c014e36
Content-Type: application/http

HTTP/1.1 400 timeout reading response body


--9c1c0d27a7292aa27aec4ea3c9eb8f125620686a87bc38d036911c014e36
```

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
