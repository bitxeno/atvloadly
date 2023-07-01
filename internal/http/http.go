package http

import (
	"fmt"

	"github.com/go-resty/resty/v2"
)

const (
	HEADER_USER_AGENT = "User-Agent"
	HTTP_USER_AGENT   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.159 Safari/537.36 Edg/92.0.902.84"
)

func NewClient() *resty.Client {
	return resty.New().SetHeader(HEADER_USER_AGENT, HTTP_USER_AGENT)
}

func Get(url string) string {
	resp, err := resty.New().SetHeader(HEADER_USER_AGENT, HTTP_USER_AGENT).R().Get(url)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	return string(resp.Body())
}

func Post(url string, postBody interface{}) string {
	resp, err := resty.New().R().SetHeader(HEADER_USER_AGENT, HTTP_USER_AGENT).SetBody(postBody).Post(url)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	return string(resp.Body())
}
