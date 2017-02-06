# drivethru
> Latest Release File Server

## Intro
**drivethru** does the heavy-lifting of presending end-users with a one line command for installing your application.


## Features
**installation scripts** - drivethru generates an installation script that determines the OS and Architecture of the machine it is running on, and serves it at: `/get/:appName`

**auto tarball** - drivethru serves binary files as tarballs on-the-fly, so that your build scripts don't have to, and serves it at: `/download/:appName/:os/:arch`


## Installation
```
curl -s http://dl.sudoba.sh/get/drivethru | sh
```

## Configuration
The configuration file is loaded from: `/etc/drivethru/drivethru.conf` and the options are very simple to configure. Example:

```
url = dl.sudoba.sh

host = localhost
port = 2020
source_folder = /etc/drivethru/source

[drivethru]
folder = drivethru
github = https://github.com/murdinc/drivethru

[awsm]
folder = awsm
github = https://github.com/murdinc/awsm

[crusher]
folder = crusher
github = https://github.com/murdinc/crusher

[isosceles]
folder = isosceles
github = https://github.com/murdinc/isosceles

```