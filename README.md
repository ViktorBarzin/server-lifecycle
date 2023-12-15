# What?

Lifecycle management script for iDRAC machines using REDFISH.

Gracefully shutdown a server if its redundant power supply goes out.

# Usage

go run . -help

# Installation

To build and install on Synology:

```
GOOS=linux GOARCH=arm CGO_ENABLED=0 go build -o server-powercheck .
```
