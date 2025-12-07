package requests

import (
	"bytes"
	"encoding/json"
	_ "errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps http.Client allowing custom UA.
type Client struct {
	httpClient *http.Client
	UserAgent  string
}

// Data models
type BasicSignInfo struct {
	CourseID int `json:"courseId"`
	SignID   int `json:"signId"`
}

type ActiveSign struct {
	CourseID int    `json:"courseId"`
	SignID   int    `json:"signId"`
	IsGPS    int    `json:"isGPS"`
	IsQR     int    `json:"isQR"`
	Name     string `json:"name"`
	Code     string `json:"code"`
}

type SignInQuery struct {
	CourseID int      `json:"courseId"`
	SignID   int      `json:"signId"`
	Lon      *float64 `json:"lon,omitempty"`
	Lat      *float64 `json:"lat,omitempty"`
}

type StudentField struct {
	ItemName  string      `json:"item_name"`
	ItemValue interface{} `json:"item_value"` // 可能是string, int, null
}

func New(userAgent string) *Client {
	// 提高超时时间，缓解偶发的首包慢/网络抖动
	return &Client{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		UserAgent:  userAgent,
	}
}

func (cli *Client) doJSON(method, url, openid string, reqt_body any, resp_body any, referrer string) error {
	var reader io.Reader
	//序列化请求体
	if reqt_body != nil {
		json_reqt_body, err := json.Marshal(reqt_body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(json_reqt_body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return err
	}
	//设置请求头
	req.Header.Set("User-Agent", cli.UserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,en-US;q=0.7,en;q=0.3")
	if openid != "" {
		req.Header.Set("openId", openid)
	}
	if referrer != "" {
		req.Header.Set("Referrer", referrer)
	}
	//发送请求
	resp, err := cli.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	//检查响应码
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	//获取响应体
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(resp_body); err != nil {
		return err
	}
	return nil
}

func (c *Client) ActiveSigns(openID string) ([]ActiveSign, error) { //获取活跃签到
	var out []ActiveSign
	err := c.doJSON("GET", "https://v18.teachermate.cn/wechat-api/v1/class-attendance/student/active_signs", openID, nil, &out, fmt.Sprintf("https://v18.teachermate.cn/wechat-pro-ssr/student/sign?openid=%s", openID))
	return out, err
}

func (c *Client) SignIn(openID string, q SignInQuery) (map[string]interface{}, error) { //post定位签到的请求
	var out map[string]interface{}
	err := c.doJSON("POST", "https://v18.teachermate.cn/wechat-api/v1/class-attendance/student-sign-in", openID, q, &out, fmt.Sprintf("https://v18.teachermate.cn/wechat-pro-ssr/student/sign?openid=%s", openID))
	return out, err
}

func (c *Client) GetStudentName(openID string) (string, error) {
	var data [][]StudentField // 对应返回的二维数组结构

	err := c.doJSON("GET",
		"https://v18.teachermate.cn/wechat-api/v2/students",
		openID,
		nil,
		&data,
		fmt.Sprintf("https://v18.teachermate.cn/wechat-pro/student/edit?openid=%s", openID),
	)
	if err != nil {
		return "", err
	}

	// 遍历所有组的所有字段
	for _, group := range data {
		for _, field := range group {
			if field.ItemName == "name" {
				// 确保值是字符串类型
				if name, ok := field.ItemValue.(string); ok && name != "" {
					return name, nil
				}
			}
		}
	}

	return "", fmt.Errorf("can't find name")
}
