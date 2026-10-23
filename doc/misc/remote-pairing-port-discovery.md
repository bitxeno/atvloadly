# 只凭 IP 探测 Apple 设备 Remote Pairing 端口

本文记录在**只知道设备 IP**（无 mDNS、无 avahi、无配对记录、无 USB）的情况下，
如何探测出设备的两个 Remote Pairing 服务端口：

- **manual-pairing 端口**：跑 PIN 配对流程的服务（`_remotepairing-manual-pairing._tcp`）
- **隧道 / 安装端口**：跑 `createListener` 与安装隧道的服务（`_remotepairing._tcp`）

典型场景：设备通过 Tailscale / 其它 overlay VPN 接入，mDNS 组播无法跨越，
但 IP 可达。

所有数据均为对一台 Apple TV 4K（`AppleTV11,1`，tvOS 18.0，`192.168.1.229`）
的实测结果。

---

## 1. 结论速览

| 方式 | 需要条件 | 耗时（实测） | 可靠性 |
| --- | --- | --- | --- |
| mDNS 组播浏览（现有实现） | 同广播域 + avahi | 事件驱动 | 跨 VPN 不可用 |
| **mDNS 单播 DNS-SD 查询** | 设备 UDP 5353 可达 | **3–11 ms** | 需设备处于配对模式 |
| **RPPairing 握手扫描** | 设备 TCP 端口可达 | **7–12 s**（全端口段） | 与协议一致，最可靠 |
| TCP connect 扫描 + 分类 | 同上 | 3.6 s + 分类 | 端口可能漏（超时过短时） |

**关键点**：manual-pairing 端口不是固定值，也不是 `62078`。

- `62078` 是 **lockdownd**（经典 lockdown 协议，Apple 固定值），
  对 RPPairing 帧直接 RST。
- manual-pairing 端口是**动态分配**的，每次设备重启服务都会变
  （本次实测为 `61108`，此前另一轮为其它值）。

---

## 2. 协议基础：RPPairing 帧格式

`_remotepairing*` 服务使用的私有协议叫 RPPairing，帧结构极简：

```
+----------------+-------------------+----------------------------+
| "RPPairing"    | length (uint16 BE)| JSON payload (length 字节) |
| 9 bytes ASCII  | 2 bytes           |                            |
+----------------+-------------------+----------------------------+
```

- Magic：`RPPairing`，**9 字节**（容易误当成 8 字节，实测必须精确读 9 字节）。
- 长度：`uint16`，大端，仅表示 JSON 部分长度。
- Payload：JSON（UTF-8），**不是 plist**。

### 2.1 请求信封

```json
{
  "message": {
    "plain": {
      "_0": { "request": { "_0": { "<方法名>": { "_0": { ...参数... } } } } }
    }
  },
  "originatedBy": "host",
  "sequenceNumber": 0
}
```

### 2.2 响应信封

```json
{
  "message": {
    "plain": {
      "_0": {
        "response": {
          "forRequestIdentifier": 0,
          "_1": { "<方法名>": { "_0": { ...结果... } } }
        }
      }
    }
  },
  "originatedBy": "device",
  "sequenceNumber": 0
}
```

- `forRequestIdentifier` 回显请求的 `sequenceNumber`，可用于关联。
- `originatedBy` 恒为 `device`。
- 设备发完一条响应后**关闭连接**（同一 socket 发第二个请求会 EOF），
  所以每个端口探测必须新建连接。

---

## 3. 核心方法：握手探测（handshake）

不需要 PIN、不需要配对记录、不产生副作用，属于只读探测。

### 3.1 请求

方法名 `handshake`，参数：

```json
{
  "hostOptions": { "attemptPairVerify": true },
  "wireProtocolVersion": 19
}
```

完整请求体（紧凑 JSON，177 字节）：

```json
{"message":{"plain":{"_0":{"request":{"_0":{"handshake":{"_0":{"hostOptions":{"attemptPairVerify":true},"wireProtocolVersion":19}}}}}}},"originatedBy":"host","sequenceNumber":0}
```

加上帧头共 188 字节。

#### 参数要求（实测，缺一不可）

| 情况 | 结果 |
| --- | --- |
| `hostOptions` 缺失 | 设备直接断开（EOF） |
| `hostOptions` 为空 `{}` | 设备直接断开（EOF） |
| `hostOptions` 含无关字段 `{"dummy":1}` | 设备直接断开（EOF） |
| `hostOptions.attemptPairVerify` 存在，值为 `true` 或 `false` | **正常响应** |
| `wireProtocolVersion` 缺失 | 设备直接断开（EOF） |
| `wireProtocolVersion` 为 1 / 19 / 200 | 均正常响应（设备按自己的版本回答） |

结论：`hostOptions` 必须**包含 `attemptPairVerify` 这个键**（值无关），
`wireProtocolVersion` 必须存在。

### 3.2 响应（manual-pairing 端口）

```json
{
  "message": {
    "plain": {
      "_0": {
        "response": {
          "forRequestIdentifier": 0,
          "_1": {
            "handshake": {
              "_0": {
                "wireProtocolVersion": 22,
                "minimumSupportedWireProtocolVersion": 8,
                "deviceOptions": {
                  "allowsPairSetup": true,
                  "allowsPinlessPairing": false,
                  "allowsIncomingTunnelConnections": false,
                  "allowsPromptlessAutomationPairingUpgrade": false,
                  "allowsSharingSensitiveInfo": false
                },
                "peerDeviceInfo": {
                  "name": "AppleTV",
                  "udid": "",
                  "identifier": "",
                  "model": "AppleTV11,1",
                  "deviceKVSIncludesSensitiveInfo": false,
                  "deviceKVSData": "<base64 编码的 plist>"
                }
              }
            }
          }
        }
      }
    }
  },
  "originatedBy": "device",
  "sequenceNumber": 0
}
```

响应体 1536 字节左右（因为带 `deviceKVSData`）。

### 3.3 响应（隧道 / 安装端口）

```json
{
  "message": {
    "plain": {
      "_0": {
        "response": {
          "forRequestIdentifier": 0,
          "_1": {
            "handshake": {
              "_0": {
                "wireProtocolVersion": 22,
                "minimumSupportedWireProtocolVersion": 8,
                "deviceOptions": {
                  "allowsIncomingTunnelConnections": true,
                  "allowsSharingSensitiveInfo": true,
                  "allowsPinlessPairing": false,
                  "allowsPromptlessAutomationPairingUpgrade": false,
                  "allowsPairSetup": false
                }
              }
            }
          }
        }
      }
    }
  },
  "originatedBy": "device",
  "sequenceNumber": 0
}
```

响应体 397 字节，**没有 `peerDeviceInfo`**。

### 3.4 分类判据

```
if deviceOptions.allowsPairSetup == true                 -> manual-pairing 端口
if deviceOptions.allowsIncomingTunnelConnections == true  -> 隧道 / 安装端口
```

两种服务互斥（实测同一设备上 `allowsPairSetup` 与
`allowsIncomingTunnelConnections` 不会同时为 true）。

`peerDeviceInfo` 的有无可作为二次确认：只有 manual-pairing 服务返回它。

---

## 4. 从 `peerDeviceInfo` 顺带拿到设备身份

`peerDeviceInfo.deviceKVSData` 是 base64 编码的 plist，解码后可直接读到
机型、系统版本、设备类别——**不需要配对记录**，也**不需要
`plumesign device-info`**：

```python
import base64, plistlib

kv = peer["deviceKVSData"]
info = plistlib.loads(base64.b64decode(kv))
# 根字典有两个键："NULL" 与 "com.apple.mobile.ldwatch"
info = info["NULL"]

print(info["ProductType"])    # AppleTV11,1
print(info["ProductVersion"]) # 18.0
print(info["DeviceClass"])    # AppleTV
print(info["BuildVersion"])   # 22J357
print(info["DeviceName"])     # AppleTV
print(info["HWModelStr"])     # J305AP
print(info["MarketingNameString"])  # Apple TV
```

注意：

- 根字典是 `{"NULL": {...}, "com.apple.mobile.ldwatch": {...}}`，
  设备信息在 `NULL` 键下（字面量就是字符串 `"NULL"`）。
- `peerDeviceInfo.udid` 与 `identifier` 在未配对状态下为**空字符串**，
  真正的 UDID 拿不到；`ProductType` + `DeviceClass` 已足够分类设备。
- 该数据在隧道端口上不存在，所以只能从 manual-pairing 端口获取。

---

## 5. 三种发现路径

### 5.1 mDNS 组播浏览（现有实现）

监听 `_remotepairing-manual-pairing._tcp.local` 的 PTR/SRV 记录。
本仓库已实现：

- `internal/manager/device_manager_avahi.go:24`（avahi 后端）
- `internal/manager/device_manager_mdns.go:27`（纯 Go mDNS 后端）

端口取自 SRV 记录：`internal/manager/device_manager_avahi.go:338`
`Port: service.Port`。

**限制**：mDNS 是链路本地组播，跨 Tailscale / 路由不可用。

### 5.2 mDNS 单播 DNS-SD 查询（新增推荐）

mDNS 协议本身允许**单播查询**：向设备的 UDP 5353 直接发一个标准 DNS 查询，
设备会单播回复。实测 30/30 成功，延迟 3–11 ms。

请求要点：

- 目标：`<device-ip>:5353/udp`
- 标准 DNS 报文头，QDCOUNT=1
- QNAME：`_remotepairing-manual-pairing._tcp.local`
- QTYPE：`PTR`（12）
- QCLASS：`IN`（1），或带 QU 位（`0x8001`）——两者实测都可用
- 报文头 FLAGS 置 RD（`0x0100`）与否都可以

响应（407 字节）包含：

| 记录 | 内容（实测） |
| --- | --- |
| PTR | `AppleTV._remotepairing-manual-pairing._tcp.local` |
| SRV | priority=0 weight=0 **port=61108** target=`AppleTV.local` |
| TXT | `identifier=DFA034B1-…` `authTag=vg65Dzxz` `model=AppleTV11,1` `name=AppleTV` `ver=22` `minVer=17` |
| A | `AppleTV.local` → `192.168.1.229` |

TTL 均为 10 秒，缓存不要超过这个值。

同样的查询对 `_remotepairing._tcp.local` 返回隧道端口（实测 `49152`）。

注意：

- 查询未广播的服务类型（如 `_nonexistent._tcp.local`）会**超时无响应**，
  可用作"设备是否在线"的判据。
- `_remotepairing-manual-pairing._tcp` **只在设备处于配对模式时广播**
  （Apple TV：设置 → 遥控器与设备 → 遥控器 App 与设备）。
  不在配对模式时该查询超时，但 TCP 扫描仍能找到端口——
  所以两者应互为补充。
- 若设备只广播 `_remotepairing._tcp`（部分 tvOS 版本），
  manual-pairing 端口需靠 TCP 扫描发现。
- 该查询能否跨 Tailscale 成功取决于设备是否接受来自非本地子网的
  UDP 5353；本机测试仅覆盖同 LAN。**跨 overlay 场景请以 TCP 扫描为准。**

### 5.3 TCP 握手扫描（无需 mDNS，最可靠）

遍历端口段，对每个端口做第 3 节的握手探测。

---

## 6. 端口段与扫描参数（实测调优）

Apple 设备的动态服务端口取自 Darwin 临时端口段
`net.inet.ip.portrange.first=49152` … `last=65535`。
实测两个目标端口（61108、49152）都落在此段内。

### 6.1 纯 connect 扫描（先找开放端口，再分类）

| 并发 | 单端口超时 | 耗时 | 发现的开放端口 |
| --- | --- | --- | --- |
| 200 | 1.0 s | 33.7 s | 49152, 49153, 60540, 61108 |
| 384 | 0.5 s | 20.1 s | 同上（部分轮次漏 49152/49153） |
| 512 | 0.4 s | 12.2–12.9 s | 60540, 61108（漏 49152） |
| 1024 | 0.25 s | **4.1 s** | 60540, 61108（稳定，5/5 命中 61108） |
| 1536 | 0.3 s | **3.4 s** | 60540, 61108（稳定） |
| 2048 | 0.2 s | 188 s（文件描述符/端口耗尽） | — |

结论：**1024 并发 / 250 ms** 是稳定甜点；2048 并发会因本地临时端口
与 fd 耗尽而急剧退化。

注意 1024/250ms 会漏掉响应较慢的端口（本例 49152/49153 有时不出现），
但目标 manual-pairing 端口 61108 稳定命中。若需要完整开放端口清单
（用于同时识别隧道端口），建议 1024 并发 / 400 ms。

### 6.2 直接握手扫描（一步到位，不先 dial）

对全段每个端口直接发握手帧，读响应：

| 并发 | 超时 | 耗时 | 结果 |
| --- | --- | --- | --- |
| 1024 | 0.8 s | 12–14 s | 61108（setup），49152（tunnel，不稳定） |
| 2048 | 0.8 s | 6.7 s | 61108（setup） |

直接握手更快（省掉一次 dial），但并发过高时隧道端口会漏。

### 6.3 推荐流程（两阶段）

```
阶段 1：1024 并发 / 250 ms，TCP connect 扫 49152–65535
        -> 3.6–4.1 s，得到开放端口列表

阶段 2：对每个开放端口做握手探测（串行或 16 并发）
        -> 5 个端口约 2.2 s

总计：约 7–12 s
```

实测 3 轮结果完全一致：

```
open ports (4.1s): [49152, 49153, 60540, 61108]
  49152  TUNNEL-ONLY     ver=22
  61108  MANUAL-PAIRING  model=AppleTV11,1 name=AppleTV ver=22
```

### 6.4 现有实现的参数问题

`internal/manager/install_port_scan.go:36` 使用 `scanConcurrency = 200`，
`:41` 使用 `networkDialWait = 1s`，对应 33.7 秒的实测耗时。
如果要复用这条路径做配对前扫描，需要把并发提到 1024、超时降到 250–400 ms。

---

## 7. 端口指纹：如何区分"不是目标服务"

同一台设备上实测到的各类端口：

| 端口 | 发送内容 | 响应 | 判定 |
| --- | --- | --- | --- |
| 61108 | RPPairing handshake | 合法响应，`allowsPairSetup=true` | **manual-pairing** |
| 49152 | RPPairing handshake | 合法响应，`allowsIncomingTunnelConnections=true` | **隧道 / 安装** |
| 60540 | 任意字节 | `15 03 01 00 02 02 46`（TLS alert） | TLS-PSK 隧道端口，非配对 |
| 62078 | RPPairing 帧 | `Connection reset by peer` | lockdownd（经典 lockdown） |
| 62078 | lockdown plist | 返回 `com.apple.mobile.lockdown` 的 `QueryType` 响应 | lockdownd 确认 |
| 49153 | 任意字节 | 无响应（静默） | 非目标 |
| 未监听端口 | — | `Connection refused` | 关闭 |

要点：

- **`62078` 不是 manual-pairing 端口**。它是 lockdownd，
  说 plist 协议，发 RPPairing 帧会被 RST。
- TLS 端口（本例 60540）只要收到任何字节就回 TLS alert，
  可据此快速排除，避免误判为 RPPairing。
- 只 connect 不发数据时，所有端口都静默，无法区分——
  必须发探测帧。

### 7.1 `plumesign pair` 的实际行为（验证）

```
$ plumesign pair --ip 192.168.1.229 --port 61108
Starting remote pairing with 192.168.1.229:61108
Enter PIN:
-> Invalid PIN: PIN must contain at least 6 characters   # 说明协议握手成功，进入 PIN 阶段

$ plumesign pair --ip 192.168.1.229 --port 62078
-> Error: Operation timed out

$ plumesign pair --ip 192.168.1.229 --port 49152
-> Invalid PIN: Socket(ConnectionReset)                  # 隧道端口拒绝 pair-setup
```

即：**能进入 `Enter PIN:` 的端口就是 manual-pairing 端口**，
这与第 3 节的 `allowsPairSetup` 判据完全一致。

---

## 8. 在本仓库中的接入建议

### 8.1 修正 `62078` 假设

`internal/manager/pair_manager.go:78-87` 当前对手动设备强制使用
`pairPortFixed`（62078）：

```go
pairPort := device.Port
if device.Manual {
    pairPort = pairPortFixed   // 62078 —— 这是 lockdownd，不是配对服务
}
```

应改为：手动设备在配对前先扫描出 `allowsPairSetup` 的端口，
再传给 `plumesign pair`。

同时 `internal/manager/install_port_scan.go:19-27` 的注释
（"manual-pairing service runs the PIN flow on Apple's fixed lockdown port
62078"）需要更正。

### 8.2 用握手分类替代 plist 校验

`internal/manager/install_port_scan.go:196` `ScanDeviceInstallPort` 与
`:221` `ScanAndImportPairingFile` 目前都依赖配对记录做
`plumesign device-info` 校验，并且：

- 首次导入时没有本机记录（必须用上传的 plist）；
- 最多只试 `maxValidatedPorts = 8` 个候选端口；
- 每个候选要等 `installProbeWait = 6s` 超时。

改用握手探测后：

- 不需要配对记录就能识别端口（`allowsIncomingTunnelConnections`）；
- 单端口探测约 20 ms，而不是 6 s；
- 8 个候选的上限可以取消。

配对记录仍然需要保留，但只用于**真正的隧道握手校验**，
不用于端口发现。

### 8.3 建议的探测函数签名

```go
// ProbeRemotePairingService 发送 RPPairing handshake 并解析设备能力。
// 返回 nil 表示该端口不是 RPPairing 服务（或不可达）。
type PairingServiceInfo struct {
    Port        uint16
    PairSetup   bool   // allowsPairSetup
    Tunnel      bool   // allowsIncomingTunnelConnections
    Model       string // peerDeviceInfo.model（仅 manual-pairing 端口）
    DeviceName  string
    ProtocolVer uint64
}

func ProbeRemotePairingService(ip string, port uint16, timeout time.Duration) (*PairingServiceInfo, error)
```

实现要点：

1. 帧头写 `"RPPairing"`（9 字节）+ `uint16` 大端长度；
2. 请求 JSON 用紧凑格式，`hostOptions` 必须含 `attemptPairVerify`；
3. 严格读满 9 字节 magic，再读 2 字节长度，再读满整个 body；
4. 每个端口一个新连接（设备发完一条响应即关闭）；
5. 判定 `allowsPairSetup` / `allowsIncomingTunnelConnections`。

### 8.4 与 avahi 的关系

本方案**不依赖 avahi**，纯 TCP/UDP，因此在
`internal/manager/device_manager_avahi.go` 之外独立可用，
也适用于容器内没有 avahi、或设备在 overlay 网络上的场景。

---

## 9. 复现脚本

### 9.1 探测单个端口

```python
import socket, struct, json

MAGIC = b"RPPairing"   # 9 bytes

REQ = json.dumps({
    "message": {"plain": {"_0": {"request": {"_0": {"handshake": {"_0": {
        "hostOptions": {"attemptPairVerify": True},
        "wireProtocolVersion": 19,
    }}}}}}},
    "originatedBy": "host",
    "sequenceNumber": 0,
}, separators=(",", ":")).encode()

FRAME = MAGIC + struct.pack(">H", len(REQ)) + REQ


def probe(ip, port, timeout=3.0):
    s = socket.socket()
    s.settimeout(timeout)
    try:
        s.connect((ip, port))
        s.sendall(FRAME)

        def read_exact(n):
            buf = b""
            while len(buf) < n:
                chunk = s.recv(n - len(buf))
                if not chunk:
                    raise EOFError(f"connection closed at {len(buf)}/{n}")
                buf += chunk
            return buf

        if read_exact(len(MAGIC)) != MAGIC:
            return None
        length = struct.unpack(">H", read_exact(2))[0]
        body = json.loads(read_exact(length))
        hs = body["message"]["plain"]["_0"]["response"]["_1"]["handshake"]["_0"]
        opts = hs.get("deviceOptions", {})
        info = hs.get("peerDeviceInfo", {})
        return {
            "pair_setup": opts.get("allowsPairSetup", False),
            "tunnel": opts.get("allowsIncomingTunnelConnections", False),
            "model": info.get("model"),
            "name": info.get("name"),
            "protocol_version": hs.get("wireProtocolVersion"),
        }
    except Exception:
        return None
    finally:
        s.close()


print(probe("192.168.1.229", 61108))
# {'pair_setup': True, 'tunnel': False, 'model': 'AppleTV11,1',
#  'name': 'AppleTV', 'protocol_version': 22}
```

### 9.2 全段扫描（两阶段）

```python
import socket, struct, json, time, sys
from concurrent.futures import ThreadPoolExecutor

MAGIC = b"RPPairing"
REQ = json.dumps({
    "message": {"plain": {"_0": {"request": {"_0": {"handshake": {"_0": {
        "hostOptions": {"attemptPairVerify": True},
        "wireProtocolVersion": 19,
    }}}}}}},
    "originatedBy": "host", "sequenceNumber": 0,
}, separators=(",", ":")).encode()
FRAME = MAGIC + struct.pack(">H", len(REQ)) + REQ

PORT_FIRST, PORT_LAST = 49152, 65535
DIAL_WORKERS, DIAL_TIMEOUT = 1024, 0.25


def dial(ip, port):
    s = socket.socket()
    s.settimeout(DIAL_TIMEOUT)
    try:
        s.connect((ip, port))
        return port
    except Exception:
        return None
    finally:
        s.close()


def handshake(ip, port, timeout=2.0):
    s = socket.socket()
    s.settimeout(timeout)
    try:
        s.connect((ip, port))
        s.sendall(FRAME)

        def read_exact(n):
            buf = b""
            while len(buf) < n:
                chunk = s.recv(n - len(buf))
                if not chunk:
                    raise EOFError
                buf += chunk
            return buf

        if read_exact(len(MAGIC)) != MAGIC:
            return None
        length = struct.unpack(">H", read_exact(2))[0]
        body = json.loads(read_exact(length))
        hs = body["message"]["plain"]["_0"]["response"]["_1"]["handshake"]["_0"]
        opts = hs.get("deviceOptions", {})
        info = hs.get("peerDeviceInfo", {})
        return {
            "pair_setup": opts.get("allowsPairSetup", False),
            "tunnel": opts.get("allowsIncomingTunnelConnections", False),
            "model": info.get("model"),
            "name": info.get("name"),
        }
    except Exception:
        return None
    finally:
        s.close()


def discover(ip):
    t0 = time.time()
    with ThreadPoolExecutor(max_workers=DIAL_WORKERS) as ex:
        opened = [p for p in ex.map(lambda p: dial(ip, p),
                                    range(PORT_FIRST, PORT_LAST + 1)) if p]
    t1 = time.time()
    with ThreadPoolExecutor(max_workers=16) as ex:
        found = [(p, r) for p, r in
                 ex.map(lambda p: (p, handshake(ip, p)), opened) if r]
    t2 = time.time()
    print(f"open ports ({t1 - t0:.1f}s): {opened}")
    for port, r in sorted(found):
        kind = ("MANUAL-PAIRING" if r["pair_setup"]
                else "TUNNEL" if r["tunnel"] else "other")
        print(f"  {port}  {kind}  model={r['model']} name={r['name']}")
    print(f"total {t2 - t0:.1f}s")
    return found


discover(sys.argv[1] if len(sys.argv) > 1 else "192.168.1.229")
```

实测输出：

```
open ports (4.1s): [49152, 49153, 60540, 61108]
  49152  TUNNEL  model=None name=None
  61108  MANUAL-PAIRING  model=AppleTV11,1 name=AppleTV
total 7.1s
```

### 9.3 mDNS 单播 DNS-SD 查询

```python
import socket, struct


def encode_name(name):
    out = b""
    for label in name.strip(".").split("."):
        out += bytes([len(label)]) + label.encode()
    return out + b"\x00"


def mdns_ptr(ip, service, timeout=2.0):
    # FLAGS=0x0100 (RD), QTYPE=PTR(12), QCLASS=IN|QU(0x8001)
    packet = (struct.pack(">HHHHHH", 0x1234, 0x0100, 1, 0, 0, 0)
              + encode_name(service)
              + struct.pack(">HH", 12, 0x8001))
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    s.settimeout(timeout)
    try:
        s.sendto(packet, (ip, 5353))
        data, _ = s.recvfrom(9000)
        return data
    finally:
        s.close()


data = mdns_ptr("192.168.1.229", "_remotepairing-manual-pairing._tcp.local")
print(len(data), "bytes")
```

解析 SRV 记录即可拿到端口（TTL 10 秒）。也可以直接用仓库已有的
`github.com/miekg/dns`（`go.mod` 已依赖，`vendor/` 中已存在）。

---

## 10. 限制与注意事项

1. **配对模式**：`_remotepairing-manual-pairing._tcp` 只在设备进入配对模式
   时广播。Apple TV 需在
   `设置 → 遥控器与设备 → 遥控器 App 与设备` 页面停留。
2. **iOS 需要 Wi-Fi**：iPhone / iPad 在纯蜂窝网络下不会启动或暴露
   lockdown / developer 服务（见 `internal/manager/reachability.go:17`
   的注释与上游实测）。任何端口扫描都会失败。
3. **端口每次变化**：设备服务重启后重新分配端口，因此
   **每次配对 / 安装前都要重新探测**，不能缓存。
4. **不要缓存 mDNS 结果超过 TTL**（10 秒）。
5. **握手探测是只读的**：不发送 PIN、不写配对记录，可以安全重复执行。
   但设备会记录一次 `attemptPairVerify` 会话。
6. **并发上限**：单机 1024 并发是稳定上限，2048 并发会因本地临时端口
   与文件描述符耗尽导致耗时爆炸（实测 188 s）。
7. **overlay 网络**：跨 Tailscale 时 TCP 连接建立可能接近 1 秒，
   `networkDialWait` 不宜低于 250 ms；同时注意部分设备对关闭端口
   静默丢包而非 RST，超时过短会漏端口。
8. **`sequenceNumber`**：请求可任意设置，设备在
   `forRequestIdentifier` 中原样回显；设备自身响应的
   `sequenceNumber` 恒为 0。
9. **单次连接单次请求**：设备回复后立即关闭连接，
   不要复用 socket。

---

## 11. 与配对流程的关系

完整的手动配对流程（探测端口后）：

```
1. 探测：找到 allowsPairSetup=true 的端口 P
2. plumesign pair --ip <ip> --port <P>
3. 设备显示 6 位 PIN，前端输入，写入子进程 stdin
4. plumesign 保存 <udid>.plist 与 <udid>.json 到 data/pairing_files/
5. 从 <udid>.json（peer record）读取 UDID / model / name
6. 探测隧道端口（allowsIncomingTunnelConnections=true）用于后续安装
```

第 6 步也可以在配对前完成：隧道端口与配对无关，
`allowsIncomingTunnelConnections=true` 的端口在配对前就已存在
（本次实测配对前 `49152` 即返回 `tunnel=true`）。

---

## 附录：实测环境

| 项 | 值 |
| --- | --- |
| 设备 | Apple TV 4K（`AppleTV11,1`，HW `J305AP`） |
| 系统 | tvOS 18.0（`22J357`） |
| 地址 | `192.168.1.229`（同 LAN） |
| manual-pairing 端口 | `61108`（SRV 记录，动态） |
| 隧道端口 | `49152`（SRV 记录） |
| 协议版本 | `wireProtocolVersion=22`，`minimumSupportedWireProtocolVersion=8` |
| 工具 | `plumesign`（PlumeImpactor v2.7.1-patch.3） |
