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

## 安装系统安装包
`hd install` 同样支持主流的系统安装包格式，例如：`deb`、`rpm`、`msi`、`pkg`（macOS）、`apk`（Alpine）以及 `pacman` 包。
这一过程会复用操作系统自带的包管理工具（例如：`apt-get`、`dnf`、`msiexec`、`installer`），不会重复造轮子。

```shell
# 安装一个 deb 格式的 release 附件
hd install xxx/yyy --format deb

# 或者，直接给一个安装包的下载地址
hd install https://example.com/foo_1.0.0_amd64.deb
```

在 Linux 和 macOS 上，如果当前用户不是 root，包管理工具会通过 `sudo` 执行；在 Windows 上，`msiexec` 可能需要管理员权限的终端。

## 从系统软件包仓库安装
主流软件包仓库中的软件包可以通过 `管理器:名称` 的格式来安装：

```shell
hd install apt:jenkins
hd install winget:Git.Git
hd install brew:wget
```

支持的包管理器有：`apt`、`dnf`、`yum`、`apk`、`pacman`、`zypper`、`snap`、`flatpak`、`brew`、`winget`、`choco`、`scoop` 和 `npm`。
当前平台上没有安装的管理器会被直接拒绝；`hd remove` 会通过同一个包管理器来卸载这类软件包。

## 搜索
首次使用前，请先执行命令初始化：`hd fetch`

然后就可以通过关键字搜索：`hd search jenkins`

你也可以搜索已安装的系统包管理器仓库中的软件包，搜索结果会带上 `管理器:名称` 前缀，可以直接安装：

```shell
hd search jenkins --repos
hd search jenkins --repos --managers apt,winget
```

输出示例：

```shell
$ hd search jenkins --repos
linuxsuren/jenkins-cli
apt:jenkins (2.426) - Jenkins continuous integration server
winget:Jenkins.Project.Jenkins - Jenkins
```

# 功能

* 基于 HTTP 协议下载文件的 Golang 工具库
* 多线程
* 断点续传 (TODO)
* 对 GitHub release 文件下载（安装）友好

## 管理已安装的工具
通过 `hd install` 安装的每一个工具都会被记录到 `~/.config/hd-installed.yaml` 中，你可以用以下的命令来管理它们：

查看已安装的工具（支持表格和 JSON 输出）：

```shell
hd list
hd list -o json
```

删除已安装的工具（删除二进制文件和记录）：

```shell
hd remove jcli
hd remove jcli --keep-record
```

升级已安装的工具到最新版本：

```shell
hd upgrade
hd upgrade jcli
```

## 网页控制台
把浏览器作为工具的统一使用入口：

```shell
hd web
```

打开 http://127.0.0.1:8080 即可：

* 在一个列表里看到全部工具，已安装与未安装混排展示，绿色徽标区分安装状态，列表支持按名称双向排序
* 搜索可安装的软件包，或者按分类浏览，支持表格与卡片两种展示方式
* 打开“系统仓库”开关后，可以同时搜索 `apt`、`winget`、`brew` 等包管理器仓库中的软件包，并直接安装
* 安装任意发布版本，升级与降级都支持
* 为下载选择 GitHub 加速代理，候选项来自 hd 的配置和 hd-home，也支持自定义地址
* 在新的终端窗口中打开已安装的工具
* 直接打开 web 工具的链接，软件包可以在 [hd-home](https://github.com/LinuxSuRen/hd-home) 中通过 `homepage` 字段声明入口链接

安装/升级的日志会实时输出到页面中。

可以用 `--port` 修改端口，用 `--listen` 修改监听地址。服务默认只监听 127.0.0.1 且没有鉴权，还能在本机安装、卸载和启动程序，请不要将其暴露到不受信任的网络。

## 使用多阶段构建
你想要在 Docker 构建中下载工具吗？这个很容易的，请查看下面的例子：

```dockerfile
FROM ghcr.io/linuxsuren/hd:v0.0.42 as downloader
RUN hd install kubesphere-sigs/ks@v0.0.50

FROM alpine:3.10
COPY --from=downloader /usr/local/bin/ks /usr/local/bin/ks
CMD ["ks"]
```
