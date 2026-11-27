[![](https://goreportcard.com/badge/linuxsuren/http-downloader)](https://goreportcard.com/report/linuxsuren/github-go)
[![](http://img.shields.io/badge/godoc-reference-5272B4.svg?style=flat-square)](https://godoc.org/github.com/linuxsuren/http-downloader)
[![Contributors](https://img.shields.io/github/contributors/linuxsuren/http-downloader.svg)](https://github.com/linuxsuren/github-go/graphs/contributors)
[![GitHub release](https://img.shields.io/github/release/linuxsuren/http-downloader.svg?label=release)](https://github.com/linuxsuren/github-go/releases/latest)
![GitHub All Releases](https://img.shields.io/github/downloads/linuxsuren/http-downloader/total)

<p align="center"><img src="docs/logo.svg" width="120" alt="http-downloader logo"></p>

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

默认安装目录会根据目标平台自动选择（Linux/macOS 为 `/usr/local/bin`，Windows 为当前用户可写目录）。可以一次性修改 CLI 和网页控制台共用的默认目录：

```shell
hd setup --install-dir /opt/hd/bin
```

单次安装也可以用 `--target` 覆盖目录。

软件包支持不同的来源。npm 包会通过 `npm` 全局安装：

```shell
hd install --source npm chalk
hd install --source native git
```

默认安装到本机。使用 `--mode ssh` 可以通过 SSH 将二进制文件安装到远程 Linux 服务器（支持密码、私钥和 `SSH_AUTH_SOCK`）：

```shell
hd install --mode ssh --ssh-host 10.0.0.1 --ssh-user root --ssh-key ~/.ssh/id_rsa jcli
```

这些设置也可以通过 `hd setup --mode ssh --ssh-host ...` 或网页控制台保存到 `~/.config/hd.yaml`。

搜索会同时覆盖 hd-home、npm 和系统包管理器，结果会标注来源并支持过滤：

```shell
hd search --source npm chalk
hd search --source hd-home jcli
```

# 功能

* 基于 HTTP 协议下载文件的 Golang 工具库
* 多线程
* 断点续传 (TODO)
* 对 GitHub release 文件下载（安装）友好
* 按平台选择安装目录，支持通过 CLI 和网页控制台调整默认值
* 支持本地安装和通过 SSH 安装到远程服务器
* 支持 hd-home、npm 和多个系统包管理器，并区分软件包来源

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

如果端口已被占用，`hd web` 会自动尝试后续可用端口（最多 100 个连续端口），并打印实际监听地址。使用 `--port-drift=false` 可关闭自动漂移，使用 `--port-drift-attempts N` 可修改尝试次数。

打开 http://127.0.0.1:8080 即可：

* 在一个列表里看到全部工具，已安装与未安装混排展示，绿色徽标区分安装状态，列表支持按名称双向排序
* 搜索可安装的软件包，或者按分类浏览，支持表格与卡片两种展示方式，可以按来源筛选 hd-home、npm 和系统包
* 安装任意发布版本，升级与降级都支持
* 为下载选择 GitHub 加速代理，候选项来自 hd 的配置和 hd-home，也支持自定义地址
* 在设置中修改默认安装目录，以及切换本机安装和 SSH 安装，设置与 CLI 共用 `~/.config/hd.yaml`
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
