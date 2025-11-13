package input

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)
var openidParamRe = regexp.MustCompile(`(?i)(?:[?&]openid=)([^&]+)`)

func GetOpenid() (string,error) {
	fmt.Println("input url or openid: ")
	text,err:=bufio.NewReader(os.Stdin).ReadString('\n')
	if(err != nil &&err != io.EOF) {
		return "",err
	}
	text = strings.TrimSpace(text)
	if text == ""{
		return "",fmt.Errorf("your input is nil")
	}
	if len(text) == 32 {
		return text,nil
	}
	if m:=openidParamRe.FindStringSubmatch(text);len(m) == 2 {
		return m[1],nil
	} else {
		return "",fmt.Errorf("can't get openid from your input")
	} 

}