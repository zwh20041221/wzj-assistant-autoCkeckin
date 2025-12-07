# WZJ Assistant Auto Check-in (Go)


- 轮询课程的活跃签到；
- WebSocket 预连接 Bayeux 通道，自动订阅二维码频道；
- 控制台渲染签到二维码；
- 支持多地点预设与交互式选择（西十二楼/南一楼/自定义）；
- 支持定位（GPS）签到与普通签到；
- 支持 AutoHotkey 自动识别二维码（驱动 PC 微信 Alt+A 截图识别，二维码更新即刻重扫）-----可能不太好用，鼠标会移动到识别二维码的按钮附近，可能需要手动点一下，这点实现的不太好；
- 全部日志带时间戳，支持 debug 详略切换。

> 仅供学习交流，请遵守相关平台与课程规则，勿用于任何违反协议与法规的用途。

---

## 功能概览
- 预连接 WS（Bayeux/Faye）：握手、/meta/connect、心跳、自动重握手
- 二维码签到：订阅 `/attendance/{courseId}/{signId}/qr`，控制台打印二维码（type=1）
- 结果回推：解析服务端推送的学生结果（type=3），打印姓名/学号/排名
- 交互式选点：启动时可选择预设坐标（西十二楼/南一楼）或使用配置文件默认坐标
- 定位签到：按配置 `lat/lon` 提交坐标签到；未配置时尝试无坐标
- 普通签到：直接提交，不带经纬度
- 日志：时间戳前缀；`debug=1` 打印 RAW 帧、心跳 ACK、逐条消息等细节

## 架构与目录
```
wzj-assistant-autoCkeckin/
├─ main.go                         # 入口：读取配置、交互选点、获取 openid、预连接、轮询/分支处理
├─ config.json                     # 运行配置（见下）
├─ internal/
│  ├─ input/                       # 读取用户输入（openid 或包含 openid 的 URL）
│  ├─ requests/                    # Teachermate HTTP API 封装（ActiveSigns / SignIn 等）
│  ├─ qr/                          # 终端二维码渲染（mdp/qrterminal）
│  ├─ qrws/                        # WS 客户端（Bayeux）：握手/连接/心跳/订阅/消息处理
│  └─ autoqr/                      # AutoHotkey 集成：生成二维码 PNG、驱动微信截图识别
└─ go.mod / go.sum                 # Go 模块依赖
```

关键模块说明：
- `internal/qrws/client.go`
  - `Start()`：建立 WS 连接、发送 `/meta/handshake`，读循环、心跳
  - `Attach(courseID, signID)`：登记/订阅二维码频道；如未 `connect`，则延迟到连接成功后自动订阅
  - `ResultCh`：当收到 type=3 学生结果时写入（非阻塞）
  - Debug 日志开关：`SetDebug(true/false)`
- `internal/requests/requests.go`
  - `ActiveSigns(openID)`：查询当前活跃签到
  - `SignIn(openID, SignInQuery)`：定位/普通签到
  - `GetStudentName(openID)`：读取学生姓名（用于启动确认）
- `internal/input/input.go`
  - `GetOpenid()`：支持直接输入 openid（32位）或粘贴包含 `?openid=` 的 URL

## 环境要求
- Go 1.20+（推荐 1.21/1.22）
- 终端支持 UTF-8 输出
- 可访问 `v18.teachermate.cn` 与 `www.teachermate.com.cn`（网络环境需正常）
- Windows + AutoHotkey v1（仅当启用自动扫码时需要）
  - 建议安装路径：`C:\\Program Files\\AutoHotkey\\`
  - 需使用 v1 引擎（v2 语法不兼容），可与 v2 并存

## 安装与构建
```bash
# 拉取依赖（首次建议执行）
go mod tidy

# 构建可执行文件
go build -o wzj-assistant-autoCkeckin

# 或直接运行
go run main.go
```

## 配置说明（config.json）
示例：
```json
{
  "polling_interval": 4000,
  "start_delay": 1000,
  "lat": 30.514882,
  "lon": 114.413702,
  "lat_w12": 30.508825,
  "lon_w12": 114.407184,
  "lat_s1": 30.509998,
  "lon_s1": 114.413035,
  "ua": "Mozilla/5.0 ...",
  "debug": 1,
  "autoqr_mode": "autohotkey",
  "autoqr_interval_ms": 5000,
  "autoqr_x": 720,
  "autoqr_y": 160,
  "autoqr_size": 320,
  "autoqr_recognize_x": 580,
  "autoqr_recognize_y": 520
}
```
- `polling_interval`：轮询活跃签到的间隔（毫秒）
- `lat`/`lon`：默认定位坐标（当选择“使用默认配置”时生效）
- `lat_w12`/`lon_w12`：西十二楼预设坐标
- `lat_s1`/`lon_s1`：南一楼预设坐标
- `ua`：HTTP 请求的 User-Agent（建议填写微信/浏览器 UA）
- `debug`：1=详细日志（RAW 帧/心跳/逐条消息），0=仅关键日志
- `autoqr_mode`：`manual`（手动扫码）或 `autohotkey`（自动扫码）
- `autoqr_interval_ms`：自动重扫间隔（毫秒，目前主要由二维码更新事件触发）
- `autoqr_x / autoqr_y`：二维码 PNG 显示窗口左上角屏幕坐标（像素）
- `autoqr_size`：二维码 PNG 的边长（像素），用于 Alt+A 框选区域
- `autoqr_recognize_x / autoqr_recognize_y`：识别按钮/图标的绝对屏幕坐标（像素）。若填写 0，则仅执行框选与居中点击，不再额外点击识别图标。

## 运行指南
```bash
# 方式一：直接运行
go run main.go

# 方式二：运行已编译的可执行文件
./wzj-assistant-autoCkeckin
```

典型交互流程：
1. **选择签到地点**：
   - 输入 `1`：使用西十二楼坐标（`lat_w12`, `lon_w12`）
   - 输入 `2`：使用南一楼坐标（`lat_s1`, `lon_s1`）
   - 输入 `3`：使用默认配置（`lat`, `lon`）
2. 程序验证 openid，并打印学生姓名；
3. 启动 WS 预连接，打印握手/连接日志；
4. 轮询活跃签到：
   - 若 `IsQR==1`：订阅二维码频道，控制台渲染二维码；接收 `type=3` 学生结果后打印；
   - 若 `IsGPS==1`：按选定的经纬度发起定位签到；
   - 否则：发起普通签到；
5. 所有日志带时间戳；`debug=1` 下会附加更多细节（RAW/心跳/消息计数等）。

> 使用建议：
> - 可以先运行程序，待 WS `connect ok` 后再由老师发起签到；若签到已在进行中再运行，程序会检测到活动并自动订阅（连接未就绪时会延迟订阅，连接成功后立即自动订阅）。

### Windows + AutoHotkey（自动扫码）
1) 安装 AutoHotkey v1（不是 v2），推荐 64 位 Unicode 版，安装到 `C:\\Program Files\\AutoHotkey\\`；
2) 设置环境变量指向 v1 引擎（避免误用 v2）：
```powershell
setx AUTOHOTKEY_EXE "C:\\Program Files\\AutoHotkey\\AutoHotkeyU64.exe"
```
3) 确认微信已登录，且“截图热键”为 Alt+A；
4) 在 `config.json` 中设置：
  - `autoqr_mode` 为 `autohotkey`
  - 根据屏幕布局调整 `autoqr_x/autoqr_y/autoqr_size`（二维码窗口位置与大小）
  - 如需自动点击识别按钮，配置 `autoqr_recognize_x/autoqr_recognize_y` 的绝对坐标
5) 运行程序并等待二维码频道下发，即可自动框选与识别。程序将：
  - 首次下发二维码立即扫描；
  - 新二维码下发时，自动更新图片并立刻重扫；
  - 收到服务端 `type=3` 结果后停止并打印结果。

## 常见问题（FAQ）
- 看不到二维码？
  - 检查日志中是否已出现 `subscribe ack` 与 `/attendance/.../qr` 的推送；
  - 若日志显示“connect 尚未完成，延迟订阅”，说明订阅会在 `connect ok` 后自动发起；
  - 确认网络可访问 `www.teachermate.com.cn`（WS）与 `v18.teachermate.cn`（HTTP）。
- 定位签到失败？
  - 确认 `config.json` 的 `lat/lon` 已正确填写；或留空（0）尝试无坐标；
  - 观察返回 `errorCode`，必要时提高 `debug` 以便排查。
- 日志过多？
  - 将 `debug` 设为 `0`，仅保留关键节点日志。
 - AutoHotkey 未触发？
   - 确认已安装 v1，并设置 `AUTOHOTKEY_EXE` 指向 v1 的 `AutoHotkey.exe/AutoHotkeyU64.exe`；
   - 检查微信是否置顶且已登录，截图热键是否为 Alt+A；
   - 多显示器/高 DPI：已禁用 DPI 缩放（`SetProcessDPIAware` + `Gui, -DPIScale`），坐标以实际像素为准；如仍偏移，请微调 `autoqr_x/autoqr_y/autoqr_recognize_*`；
   - 若识别图标位置有变，可将 `autoqr_recognize_*` 置为 0，仅依靠框选后的居中点击触发展示菜单，再手动确认一次。

## 开发提示
- 关键文件：
  - `internal/qrws/client.go`：Bayeux 协议细节实现（握手、连接、心跳、重握手、订阅、消息分发）
  - `internal/requests/requests.go`：HTTP 接口封装
  - `main.go`：流程编排与分支控制
- 可改进方向：
  - 断线自动重连与订阅恢复策略增强
  - 更丰富的结果输出与文件记录
  - 更灵活的重试/退避策略

---

## 免责声明
本项目仅用于技术学习与交流，请确保你的使用符合所在学校/平台的使用条款与法律法规。作者与贡献者不对由此产生的任何后果负责。
