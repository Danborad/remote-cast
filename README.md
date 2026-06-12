# remote-cast

`remote-cast` 是一个给 OpenWrt 使用的远程 DLNA/UPnP 投屏发现代理。它适合这样的场景：

- 家里 OpenWrt 已经接入 EasyTier。
- 手机也通过 EasyTier 进入了虚拟网络。
- 手机能访问家里电视的原始局域网 IP，但投屏 App 扫描不到电视。

插件会把手机侧的 SSDP 搜索请求转发到家里 LAN，再把电视的 SSDP 响应转发回手机。它只解决“发现电视”的问题，不伪造电视 IP，也不转发视频流。

## 功能

- 支持 DLNA/UPnP/SSDP 发现代理。
- 支持 OpenWrt 后台服务 `remote-castd`。
- 支持 LuCI 网页配置页面。
- 支持电视 IP 白名单，避免暴露整个局域网设备。
- 支持状态页查看最近发现的设备。
- 支持手动测试扫描。
- 默认构建目标为 OpenWrt 24.10 x86_64。

## 已打包 IPK

仓库 `bin/` 目录里包含已经构建好的 OpenWrt 24.10 x86_64 IPK：

- `bin/remote-cast_0.1.0-r1_x86_64.ipk`
- `bin/luci-app-remote-cast_0.1.0-r1_all.ipk`

两个都建议安装：

- `remote-cast`：核心后台服务。
- `luci-app-remote-cast`：LuCI 网页管理界面。

## 安装

把两个 IPK 上传到 OpenWrt，然后执行：

```sh
opkg install ./remote-cast_0.1.0-r1_x86_64.ipk ./luci-app-remote-cast_0.1.0-r1_all.ipk
/etc/init.d/rpcd restart
```

安装后进入 LuCI：

```text
服务 -> 远程投屏
```

## 配置说明

### LAN 接口

OpenWrt 连接家里局域网和电视的接口，常见是：

```text
br-lan
```

可以在 OpenWrt SSH 中查看：

```sh
ip -br addr
```

如果看到类似：

```text
br-lan UP 192.168.31.66/24
```

那 LAN 接口就填 `br-lan`。

### EasyTier 接口

OpenWrt 上 EasyTier 创建的虚拟网卡。常见名称可能是：

```text
tun0
easytier0
tap0
```

查看方式：

```sh
ip -br addr
```

如果看到类似：

```text
tun0 UNKNOWN 192.168.11.4/24
```

那 EasyTier 接口就填 `tun0`。

### 手机网段

这里填手机在 EasyTier 里的虚拟 IP 网段，不是手机流量公网 IP，也不是手机当前 Wi-Fi IP。

例如手机 EasyTier IP 是：

```text
192.168.11.131
```

则填写：

```text
192.168.11.0/24
```

如果 EasyTier 使用 `10.x.x.x` 网段，也可以填：

```text
10.0.0.0/8
```

### 电视 IP 白名单

填写电视或投屏设备的原始 LAN IP，例如：

```text
192.168.31.48
```

如果状态页一直没有发现，可以先临时把日志里出现过的 SSDP 响应 IP 加入白名单，用来判断哪个设备实际提供 DLNA 服务。

## 启动与调试

保存 LuCI 配置后，可以在 SSH 中执行：

```sh
uci commit remote-cast
/etc/init.d/remote-cast enable
/etc/init.d/remote-cast restart
/etc/init.d/remote-cast status
```

查看日志：

```sh
logread -e remote-cast
```

手动触发扫描：

```sh
remote-castctl scan
```

查看状态文件：

```sh
cat /var/run/remote-cast/devices.json
```

## 抓包排查

如果服务运行但状态页没有发现设备，可以安装 `tcpdump`：

```sh
opkg update
opkg install tcpdump
```

监听 LAN 侧 SSDP：

```sh
tcpdump -ni br-lan 'udp port 1900'
```

然后在 LuCI 状态页点击“测试扫描”。

如果能看到 OpenWrt 发往 `239.255.255.250:1900`，但电视没有回包，通常是电视没有开启 DLNA/UPnP/多屏互动，或者实际响应 DLNA 的不是你填写的电视 IP。

## EasyTier 与旁路由注意事项

OpenWrt 可以是旁路由，不一定要是主路由。关键是：

- OpenWrt 要和电视在同一个 LAN 网段。
- OpenWrt 要有 EasyTier 虚拟接口。
- 手机 EasyTier IP 要允许访问 OpenWrt 的 UDP 1900。
- 播放阶段如果电视需要反向访问手机 EasyTier 网段，主路由可能需要静态路由。

例如：

```text
手机 EasyTier 网段：192.168.11.0/24
OpenWrt LAN IP：192.168.31.66
```

主路由可添加静态路由：

```text
目标网段：192.168.11.0/24
网关：192.168.31.66
```

## 构建

当前项目提供 Docker 构建脚本，适合 Windows 环境。

默认构建 OpenWrt 24.10.7 x86_64：

```powershell
.\build.ps1
```

指定 OpenWrt 24.10 其它小版本：

```powershell
.\build.ps1 -OpenWrtVersion 24.10.6
```

构建完成后，IPK 输出到：

```text
bin/
```

## 目录结构

```text
package/remote-cast/           后台服务 OpenWrt package
package/luci-app-remote-cast/  LuCI 页面 OpenWrt package
scripts/build-openwrt-ipk.sh   Docker 内部构建脚本
build.ps1                      Windows 一键构建脚本
bin/                           已构建 IPK
```

## 限制

- 当前版本只支持 DLNA/UPnP/SSDP 发现。
- 不支持 AirPlay/mDNS。
- 不支持 Miracast/Wi-Fi Display。
- 插件只负责发现设备，不负责视频流转发。
- 某些 App 可能用 SSDP 发现，但播放阶段走私有协议，这种情况可能需要额外排查播放链路。
