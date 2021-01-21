[![](https://goreportcard.com/badge/linuxsuren/http-downloader)](https://goreportcard.com/report/linuxsuren/github-go)
[![](http://img.shields.io/badge/godoc-reference-5272B4.svg?style=flat-square)](https://godoc.org/github.com/linuxsuren/http-downloader)
[![Contributors](https://img.shields.io/github/contributors/linuxsuren/http-downloader.svg)](https://github.com/linuxsuren/github-go/graphs/contributors)
[![GitHub release](https://img.shields.io/github/release/linuxsuren/http-downloader.svg?label=release)](https://github.com/linuxsuren/github-go/releases/latest)
![GitHub All Releases](https://img.shields.io/github/downloads/linuxsuren/http-downloader/total)

# 入门

`hd` 是一个基于 HTTP 协议的下载工具。

通过命令：`brew install linuxsuren/linuxsuren/hd` 来安装

# 用法

```
hd https://github.com/jenkins-zh/jenkins-cli/releases/latest/download/jcli-linux-amd64.tar.gz -o jcli.tar.gz --thread 10
```

# 功能

* 基于 HTTP 协议下载文件的 Golang 工具库
* 多线程
* 断点续传 (TODO)
* 对 GitHub release 文件下载友好 (TODO)
