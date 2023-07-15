# atvloadly

[![platform](https://img.shields.io/badge/platform-linux%20%7C%20openwrt-989898)](https://github.com/bitxeno/atvloadly/releases)
[![release](https://ghcr-badge.egpl.dev/bitxeno/atvloadly/latest_tag?label=docker%20latest)](https://github.com/bitxeno/atvloadly/pkgs/container/atvloadly)
[![image size](https://ghcr-badge.egpl.dev/bitxeno/atvloadly/size)](https://github.com/bitxeno/atvloadly/pkgs/container/atvloadly)
[![license](https://img.shields.io/github/license/bitxeno/atvloadly)](https://github.com/bitxeno/atvloadly/blob/master/LICENSE) 

> ⚠️ **不支持 tvOS 16.5以上系统** ⚠️

atvloadly 是一个支持在 AppleTV 上侧载应用的 web 服务。底层通过使用 [AltServer](https://github.com/NyaMisty/AltServer-Linux) 实现侧载，并会自动刷新 App 以保证其长期可用性。


## 主要功能

* docker 运行 (只支持 Linux/OpenWrt x86 平台)
* 支持 AppleTV 配对
* 支持自动刷新 app
* 支持同时使用多个 Apple ID 帐号

## 截图

<p align="center">
  <img width="600" src="./doc/preview/1.png">
</p>
<p align="center">
  <img width="600" src="./doc/preview/2.png">
</p>

## 安装

> :pensive: **只支持 Linux/OpenWrt 等 x86 系统，不支持 Mac/Windws/ARM Linux 系统**

1. Linux/OpenWrt 宿主机需要安装 `avahi-deamon` 服务
   
   OpenWrt：
   ```
   opkg install avahi-dbus-daemon
   /etc/init.d/avahi-daemon start
   ```
   
   Ubuntu；
   ```
   sudo apt-get -y install avahi-daemon
   sudo systemctl restart avahi-daemon
   ```

2. 请参考下面的命令进行安装，记得修改下 mount 目录
   
   ```
   docker run -d --name=atvloadly --restart=always -p 5533:80 -v /path/to/mount/dir:/data -v /var/run/dbus:/var/run/dbus -v /var/run/avahi-daemon:/var/run/avahi-daemon  ghcr.io/bitxeno/atvloadly:latest
   ```
   
   镜像名称：`ghcr.io/bitxeno/atvloadly:latest`，需要使用这个带域名的完整名称才能pull下来。
   
   宿主机的 `/var/run/dbus` 和`/var/run/avahi-daemon` 需要共享给 docker 容器使用



## 使用方法

### 前期准备 (非常重要:bangbang:)

* 专用的 Apple ID 安装帐号，免费或开发者帐号都可以（**为了安全考虑，请不要使用常用帐号安装！**)
* 登录了安装帐号的 iPhone 手机（用于授权信任 atvloadly，会虚拟为一台 MacBook，**超时不验证授权验证码，会导致帐号被临时冻结！需要重置密码才能恢复**）


### 操作流程

1. 打开 Apple TV 设置菜单，选择 `遥控器与设备 -> 遥控器App与设备`，进入配对模式
1. 打开 Web 管理页面，正常会显示可配对的 `AppleTV`
1. 点击 `AppleTV` 设备进入配对页面，并完成配对操作。
1. 配对成功后返回首页，将显示已连接的 `AppleTV` 
1. 点击已连接的 `AppleTV` 进入侧载安装页面，选择需要侧载的 IPA 文件并点击`安装`。

## 常见问题

1、免费帐号可以安装多少个应用

> 每个 Apple ID 最多可以同时激活 3 个应用，安装超过 3 个后，会导致前面已安装的 App 变为不可用

2、升级系统后安装失败

> 升级系统后需要重新配对，一般新出的系统都不支持，建议关闭系统自动更新

3、密码可以使用App-specific password吗，这样安全些

> AltServer 目前不支持


## 推荐开源 App

[>> wiki](https://github.com/bitxeno/atvloadly/wiki/AppleTV-App)


## 如何开发编译

[>> wiki](https://github.com/bitxeno/atvloadly/wiki/How-to-build)

## 免责声明

本软件仅供学习交流使用，作者不对用户因使用本软件造成的安全风险或损失承担任何法律责任。请在操作过程中保持小心谨慎！
