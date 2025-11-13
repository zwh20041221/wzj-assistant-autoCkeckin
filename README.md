# WZJ Assistant Auto Check-in (Go)

一个用于配合「智慧教室/Teachermate」学生端的命令行签到助手：
- 轮询课程的活跃签到；
- WebSocket 预连接 Bayeux 通道，自动订阅二维码频道；
- 控制台渲染签到二维码；
- 支持定位（GPS）签到与普通签到；
- 全部日志带时间戳，支持 debug 详略切换。

> 仅供学习交流，请遵守相关平台与课程规则，勿用于任何违反协议与法规的用途。

---

## 功能概览
- 预连接 WS（Bayeux/Faye）：握手、/meta/connect、心跳、自动重握手
- 二维码签到：订阅 `/attendance/{courseId}/{signId}/qr`，控制台打印二维码（type=1）
- 结果回推：解析服务端推送的学生结果（type=3），打印姓名/学号/排名
- 定位签到：按配置 `lat/lon` 提交坐标签到；未配置时尝试无坐标
- 普通签到：直接提交，不带经纬度
- 日志：时间戳前缀；`debug=1` 打印 RAW 帧、心跳 ACK、逐条消息等细节

## 架构与目录
```
wzj-assistant-autoCkeckin/
├─ main.go                         # 入口：读取配置、获取 openid、预连接、轮询/分支处理
├─ config.json                     # 运行配置（见下）
├─ internal/
│  ├─ input/                       # 读取用户输入（openid 或包含 openid 的 URL）
│  ├─ requests/                    # Teachermate HTTP API 封装（ActiveSigns / SignIn 等）
│  ├─ qr/                          # 终端二维码渲染（mdp/qrterminal）
│  └─ qrws/                        # WS 客户端（Bayeux）：握手/连接/心跳/订阅/消息处理
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
  "lat": 0,
  "lon": 0,
  "ua": "Your UA string here",
  "debug": 0
}
```
- `polling_interval`：轮询活跃签到的间隔（毫秒）
- `start_delay`：保留字段（毫秒）
- `lat`/`lon`：用于定位签到的坐标（为 0 则尝试无坐标签到）
- `ua`：HTTP 请求的 User-Agent（建议填写微信/浏览器 UA）
- `debug`：1=开启详细日志（RAW 帧/心跳/逐条消息），0=仅关键日志

## 运行指南
```bash
# 方式一：直接运行
go run main.go

# 方式二：运行已编译的可执行文件
./wzj-assistant-autoCkeckin
```
启动后，按提示在终端输入 openid 或完整 URL（包含 `?openid=`）。

典型交互流程：
1. 程序验证 openid，并打印学生姓名；
2. 启动 WS 预连接，打印握手/连接日志；
3. 轮询活跃签到：
   - 若 `IsQR==1`：订阅二维码频道，控制台渲染二维码；接收 `type=3` 学生结果后打印；
   - 若 `IsGPS==1`：按配置经纬度发起定位签到；
   - 否则：发起普通签到；
4. 所有日志带时间戳；`debug=1` 下会附加更多细节（RAW/心跳/消息计数等）。

> 使用建议：
> - 可以先运行程序，待 WS `connect ok` 后再由老师发起签到；若签到已在进行中再运行，程序会检测到活动并自动订阅（连接未就绪时会延迟订阅，连接成功后立即自动订阅）。

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
