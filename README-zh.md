[![](https://goreportcard.com/badge/linuxsuren/http-downloader)](https://goreportcard.com/report/linuxsuren/github-go)
[![](http://img.shields.io/badge/godoc-reference-5272B4.svg?style=flat-square)](https://godoc.org/github.com/linuxsuren/http-downloader)
[![Contributors](https://img.shields.io/github/contributors/linuxsuren/http-downloader.svg)](https://github.com/linuxsuren/github-go/graphs/contributors)
[![GitHub release](https://img.shields.io/github/release/linuxsuren/http-downloader.svg?label=release)](https://github.com/linuxsuren/github-go/releases/latest)
![GitHub All Releases](https://img.shields.io/github/downloads/linuxsuren/http-downloader/total)

# 入门

`hd` 是一个基于 HTTP 协议的下载工具。

通过命令：`brew install linuxsuren/linuxsuren/hd` 来安装

或者，对于 Linux 用户可以直接通过命令下载：
```shell
curl -L https://github.com/linuxsuren/http-downloader/releases/latest/download/hd-linux-amd64.tar.gz | tar xzv
mv hd /usr/local/bin
```

想要浏览该项目的代码吗？[GitPod](https://gitpod.io/#https://github.com/linuxsuren/http-downloader) 绝对可以帮助你！

# 用法

```shell
hd get https://github.com/jenkins-zh/jenkins-cli/releases/latest/download/jcli-linux-amd64.tar.gz --thread 6
```

或者，用一个更加简便的办法：

```shell
hd get jenkins-zh/jenkins-cli/jcli -t 6
```

获取，你也可以安装一个来自 GitHub 的软件包：

```shell
hd install jenkins-zh/jenkins-cli/jcli -t 6
```

或者，你也可以从 GitHub 上下载预发布的二进制包：

```shell
hd get --pre ks
```

# 功能

* 基于 HTTP 协议下载文件的 Golang 工具库
* 多线程
* 断点续传 (TODO)
* 对 GitHub release 文件下载（安装）友好

## 使用多阶段构建
你想要在 Docker 构建中下载工具吗？这个很容易的，请查看下面的例子：

```dockerfile
FROM ghcr.io/linuxsuren/hd:v0.0.42 as downloader
RUN hd install kubesphere-sigs/ks@v0.0.50

FROM alpine:3.10
COPY --from=downloader /usr/local/bin/ks /usr/local/bin/ks
CMD ["ks"]
```
