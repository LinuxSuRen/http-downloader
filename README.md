[![](https://goreportcard.com/badge/linuxsuren/http-downloader)](https://goreportcard.com/report/linuxsuren/github-go)
[![](http://img.shields.io/badge/godoc-reference-5272B4.svg?style=flat-square)](https://godoc.org/github.com/linuxsuren/http-downloader)
[![Contributors](https://img.shields.io/github/contributors/linuxsuren/http-downloader.svg)](https://github.com/linuxsuren/github-go/graphs/contributors)
[![GitHub release](https://img.shields.io/github/release/linuxsuren/http-downloader.svg?label=release)](https://github.com/linuxsuren/github-go/releases/latest)
![GitHub All Releases](https://img.shields.io/github/downloads/linuxsuren/http-downloader/total)

# Get started
`hd` is a HTTP download tool.

Install it via: `brew install linuxsuren/linuxsuren/hd`

Or download it directly (for Linux):
```
curl -L https://github.com/linuxsuren/http-downloader/releases/latest/download/hd-linux-amd64.tar.gz | tar xzv
mv hd /usr/local/bin
```

# Usage

## Download
```
hd get https://github.com/jenkins-zh/jenkins-cli/releases/latest/download/jcli-linux-amd64.tar.gz --thread 6
```

Or use a simple way instead of typing the whole URL:

```
hd get jenkins-zh/jenkins-cli/jcli -t 6
```

## Install
You can also install a package from GitHub:

```
hd install jenkins-zh/jenkins-cli/jcli -t 6
```

## Search
hd can download or install via the format of `$org/$repo`. If you find that it's not working. It might because of there's 
no record in [hd-home](https://github.com/LinuxSuRen/hd-home). You're welcome to help us to maintain it.

When you first run it, please init via: `hd fetch`

then you can search it by a keyword: `hd search jenkins`

# Features
* go library for HTTP
* multi-thread
* continuously (TODO)
* GitHub release asset friendly
