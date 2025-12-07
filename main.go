package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/zwh20041221/wzj-assistant-autoCkeckin/internal/autoqr"
	"github.com/zwh20041221/wzj-assistant-autoCkeckin/internal/config"
	"github.com/zwh20041221/wzj-assistant-autoCkeckin/internal/input"
	"github.com/zwh20041221/wzj-assistant-autoCkeckin/internal/qrws"
	"github.com/zwh20041221/wzj-assistant-autoCkeckin/internal/requests"
)

// timestamped logging helpers for main
func ts() string                      { return time.Now().Format("2006-01-02 15:04:05.000") }
func logln(args ...any)               { fmt.Println(append([]any{"[" + ts() + "]"}, args...)...) }
func logf(format string, args ...any) { fmt.Printf("[%s] ", ts()); fmt.Printf(format, args...) }

func main() {
	// 读取配置
	cfg, err := config.Load()
	if err != nil {
		logln("your config.json is error:", err)
		os.Exit(1)
	}
	// 配置调试级别：1 打印详细日志，否则仅必要日志
	qrws.SetDebug(cfg.Debug == 1)

	// 选择签到地点
	fmt.Println("请选择签到地点:")
	fmt.Println("1. 西十二楼")
	fmt.Println("2. 南一楼")
	fmt.Println("3. 使用默认配置 (config.json 中的 lat/lon)")
	var locChoice int
	fmt.Print("请输入序号 (1-3): ")
	fmt.Scanln(&locChoice)

	switch locChoice {
	case 1:
		if cfg.Lat_W12 != 0 && cfg.Lon_W12 != 0 {
			cfg.Lat = cfg.Lat_W12
			cfg.Lon = cfg.Lon_W12
			logf("已选择西十二楼: lat=%.6f, lon=%.6f\n", cfg.Lat, cfg.Lon)
		} else {
			logln("警告: 未配置西十二楼坐标 (lat_w12, lon_w12)，将使用默认配置")
		}
	case 2:
		if cfg.Lat_S1 != 0 && cfg.Lon_S1 != 0 {
			cfg.Lat = cfg.Lat_S1
			cfg.Lon = cfg.Lon_S1
			logf("已选择南一楼: lat=%.6f, lon=%.6f\n", cfg.Lat, cfg.Lon)
		} else {
			logln("警告: 未配置南一楼坐标 (lat_s1, lon_s1)，将使用默认配置")
		}
	default:
		logln("使用默认配置坐标")
	}

	// 提取 openid
	var openid string
	for {
		id, err := input.GetOpenid()
		if err != nil {
			logln(err)
			continue
		}
		openid = id
		break
	}

	cli := requests.New(cfg.Ua)
	stuName, err := cli.GetStudentName(openid)
	if err != nil {
		logln("your openid is invalid", err)
		os.Exit(1)
	}
	logln(stuName)

	// 启动预连接（仅握手与保活，不订阅）
	warm := qrws.New()
	if err := warm.Start(); err == nil {
		logln("[Preconnect] QR 通道握手已发起")
	} else {
		logln("[警告] 预连接失败:", err)
	}

	// 轮询并处理签到
	var lastAutoQRSignID int
	for {
		active, err := cli.ActiveSigns(openid)
		if err != nil {
			logln(err)
			os.Exit(1)
		}
		if len(active) == 0 {
			time.Sleep(time.Duration(cfg.Polling_interval) * time.Millisecond)
			logln("no active sign")
			continue
		}

		a := active[0]
		logln(a)

		// 延迟策略优化：根据签到类型使用不同的延迟配置
		if a.IsQR == 1 {
			if cfg.Start_delay_qr > 0 {
				logf("检测到二维码签到，等待 %d 毫秒...\n", cfg.Start_delay_qr)
				time.Sleep(time.Duration(cfg.Start_delay_qr) * time.Millisecond)
			}
		} else if a.IsGPS == 1 {
			if cfg.Start_delay_gps > 0 {
				logf("检测到定位签到，等待 %d 毫秒...\n", cfg.Start_delay_gps)
				time.Sleep(time.Duration(cfg.Start_delay_gps) * time.Millisecond)
			}
		} else {
			if cfg.Start_delay > 0 {
				logf("检测到普通签到，等待 %d 毫秒...\n", cfg.Start_delay)
				time.Sleep(time.Duration(cfg.Start_delay) * time.Millisecond)
			}
		}

		// 分支处理：二维码 vs 定位 vs 普通
		if a.IsQR == 1 {
			// 订阅 QR 通道以接收二维码与结果
			warm.Attach(a.CourseID, a.SignID)
			// 这里的 Attach 可能只是登记了待订阅（若 connect 尚未完成），因此提示更中性
			logf("[QR] 已登记订阅目标 /attendance/%d/%d/qr，等待连接/二维码...\n", a.CourseID, a.SignID)

			// 模式控制：manual 仅等待扫码；autohotkey 自动反复截图识别
			if cfg.AutoQRMode == "autohotkey" {
				if lastAutoQRSignID != a.SignID {
					lastAutoQRSignID = a.SignID
					courseID := a.CourseID
					signID := a.SignID
					// 控制扫描协程的结束（仅在 QR 更新时触发扫描）
					autoDone := make(chan struct{})
					go func() {
						logf("[AutoQR] 等待二维码链接以触发 PC 微信截图识别 courseId=%d signId=%d\n", courseID, signID)
						var lastQR string
						var lastPNG string
						// 内部函数：对当前 QR 执行一次截图识别
						doScan := func() {
							if lastQR == "" {
								return
							}
							// 初次或 QR 更新时生成新 PNG
							if lastPNG == "" {
								p, err := autoqr.GenerateQRPng(lastQR, 320)
								if err != nil {
									logf("[AutoQR] 生成二维码 PNG 失败: %v\n", err)
									return
								}
								lastPNG = p
								logf("[AutoQR] 已生成二维码图片: %s\n", lastPNG)
							}
							if err := autoqr.LaunchWeChatScreenshot(lastPNG, cfg.AutoQRX, cfg.AutoQRY, cfg.AutoQRSize, cfg.AutoQRRecognizeX, cfg.AutoQRRecognizeY); err != nil {
								logf("[AutoQR] 调用 AutoHotkey 失败: %v\n", err)
								logln("[AutoQR] 提示: 请安装 AutoHotkey(v1) 并设置环境变量 AUTOHOTKEY_EXE，或手动按 Alt+A 截图框选生成的二维码图片以识别")
							} else {
								logln("[AutoQR] 已触发 Alt+A 并框选二维码，等待 WeChat 识别与 WS 回推(type=3)")
							}
						}

						// 主循环：仅在接收到新 QR 时进行扫描
						for {
							select {
							case qrURL := <-warm.QrURLCh:
								// 二维码更新：重置 PNG 并立即扫描
								if qrURL != lastQR {
									lastQR = qrURL
									if lastPNG != "" {
										// 尝试删除旧 PNG（忽略错误）
										_ = os.Remove(lastPNG)
										lastPNG = ""
									}
								}
								doScan()
							case <-autoDone:
								if lastPNG != "" {
									_ = os.Remove(lastPNG)
								}
								return
							}
						}
					}()

					// 在等待结果后关闭自动重扫
					defer close(autoDone)
				}
			} else {
				logln("[QR] 手动模式：不会触发 AutoHotkey 自动截图，请使用手机或 PC 微信自行识别二维码")
			}

			// 等待最多 2 分钟
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			select {
			case res := <-warm.ResultCh:
				logf("[Result] 学生: %s(%s) rank=%d id=%d\n", res.Name, res.StudentNumber, res.Rank, res.ID)
			case <-ctx.Done():
				logln("[Result] 等待结果超时，仍保持 WS 保活，可继续观察")
			}
			// 循环继续，可继续监听或再次检查活动
			continue
		}

		if a.IsGPS == 1 {
			// GPS 定位签到（使用配置坐标；未配置则尝试无坐标）
			lon, lat := cfg.Lon, cfg.Lat
			var lonPtr, latPtr *float64
			if lon == 0 && lat == 0 {
				logln("[GPS] 未配置坐标，将尝试无坐标签到")
				lonPtr, latPtr = nil, nil
			} else {
				logf("[GPS] 使用坐标 lon=%.6f lat=%.6f\n", lon, lat)
				lonPtr, latPtr = &lon, &lat
			}
			resp, err := cli.SignIn(openid, requests.SignInQuery{CourseID: a.CourseID, SignID: a.SignID, Lon: lonPtr, Lat: latPtr})
			if err != nil {
				logln("[GPS] 签到失败:", err)
				os.Exit(1)
			}
			
			var errorCode float64
			if val, ok := resp["errorCode"]; ok {
				if v, ok := val.(float64); ok {
					errorCode = v
				}
			}

			if errorCode == 0 {
				logln("[GPS] 签到成功")
				logln("[GPS] 响应内容:", resp)
			} else {
				logf("[GPS] 签到返回错误码: %.0f\n", errorCode)
				logln("[GPS] 响应内容:", resp)
			}
			// GPS/普通签到一次即结束
			break
		}

		// 普通签到（不带经纬度）
		resp, err := cli.SignIn(openid, requests.SignInQuery{CourseID: a.CourseID, SignID: a.SignID})
		if err != nil {
			logln("[Sign] 签到失败:", err)
			os.Exit(1)
		}

		var errorCode float64
		if val, ok := resp["errorCode"]; ok {
			if v, ok := val.(float64); ok {
				errorCode = v
			}
		}

		if errorCode == 0 {
			logln("[Sign] 签到成功")
			logln("[Sign] 响应内容:", resp)
		} else {
			logf("[Sign] 签到返回错误码: %.0f\n", errorCode)
			logln("[Sign] 响应内容:", resp)
		}
		break
	}

	fmt.Println("按回车键退出...")
	fmt.Scanln()
}
