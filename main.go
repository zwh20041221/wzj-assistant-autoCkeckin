package main
import (
	"github.com/zwh20041221/wzj-assistant-autoCkeckin/internal/config"
	"github.com/zwh20041221/wzj-assistant-autoCkeckin/internal/input"
	"github.com/zwh20041221/wzj-assistant-autoCkeckin/internal/requests"
	"fmt"
	"os"
)

func main() {
	//读取配置文件
	cfg,err:=config.Load()
	if(err != nil){
		fmt.Println("your config.json is error:",err)
		os.Exit(1)
	}
	//提取openid
    var openid string
	for {
		id,err:=input.GetOpenid()
		if err != nil {
			fmt.Println(err)
			continue
		} else {
			openid=id
			fmt.Println(cfg,openid)
			break
		}
	}
	cli := requests.New(cfg.Ua)
	stu_data,err:=cli.GetStudentName(openid)
	if err != nil {
		fmt.Println("your openid is invalid",err)
		os.Exit(1)		
	}
	fmt.Println(stu_data)

}