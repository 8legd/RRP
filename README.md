# Rapid Response Proxy [![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](http://godoc.org/github.com/8legd/RRP)

Speed up your web service / api requests with Rapid Response Proxy

## Features
- Simple cross platform installation
- Batch processing support

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
