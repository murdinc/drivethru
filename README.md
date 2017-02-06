# drivethru
> Latest Release File Server

[![Build Status](https://travis-ci.org/murdinc/drivethru.svg)](https://travis-ci.org/murdinc/drivethru)


## Intro
**drivethru** does the heavy-lifting of presenting end-users with a one-line-command for installing your application. For example: `curl -s http://dl.sudoba.sh/get/drivethru | sh`


## Features
**installation scripts** - drivethru generates an installation script that determines the OS and Architecture of the machine it is running on, and serves it at: `/get/:appName`

**auto tarball** - drivethru serves binary files as tarballs on-the-fly, so that your build scripts don't have to, and serves it at: `/download/:appName/:os/:arch`


## Installation
**drivethru** installs itself via this command:
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

The structure of the source_folder for the configuration above should look similar to this:

```
source/
├── awsm
│   ├── Darwin
│   │   └── amd64
│   │       └── awsm
│   ├── Linux
│   │   ├── amd64
│   │   │   └── awsm
│   │   ├── arm
│   │   │   └── awsm
│   │   └── arm64
│   │       └── awsm
│   └── Windows
│       └── amd64
│           └── awsm
├── crusher
│   ├── Darwin
│   │   └── amd64
│   │       └── crusher
│   ├── Linux
│   │   ├── amd64
│   │   │   └── crusher
│   │   ├── arm
│   │   │   └── crusher
│   │   └── arm64
│   │       └── crusher
│   └── Windows
│       └── amd64
│           └── crusher
├── drivethru
│   ├── Darwin
│   │   └── amd64
│   │       └── drivethru
│   ├── Linux
│   │   ├── amd64
│   │   │   └── drivethru
│   │   ├── arm
│   │   │   └── drivethru
│   │   └── arm64
│   │       └── drivethru
│   └── Windows
│       └── amd64
│           └── drivethru
└── isosceles
    ├── Darwin
    │   └── amd64
    │       └── isosceles
    ├── Linux
    │   ├── amd64
    │   │   └── isosceles
    │   ├── arm
    │   │   └── isosceles
    │   └── arm64
    │       └── isosceles
    └── Windows
        └── amd64
            └── isosceles

```


## Example Installation Script URL Output
This is an example installation script, generated for the installation of drivethru via the URL `http://dl.sudoba.sh/get/drivethru`
```
#!/bin/sh

FORMAT="tar.gz"
TARBALL="drivethru-$$.$FORMAT"
OS=$(uname)
ARCH=$(uname -m)
URL="http://dl.sudoba.sh/download/drivethru/$OS/$ARCH/"
DEST=/usr/local/bin

echo "Downloading $URL"

curl -o $TARBALL -L -f $URL
if [ $? -eq 0 ]
then
    echo "Copying drivethru binary into $DEST"
    sudo mkdir -p $DEST/
    tar -xzf $TARBALL && sudo mv -f drivethru $DEST/
    if [ $? -eq 0 ]
    then
        rm $TARBALL
        echo "drivethru has been installed into $DEST/drivethru"
        exit 0
    fi
else
    echo "Failed to determine your platform.\nTry downloading from https://github.com/murdinc/drivethru"
fi

exit 1
```