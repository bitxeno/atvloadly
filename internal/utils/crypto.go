package utils

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
)

func Md5(str string) string {
	data := []byte(str)
	has := md5.Sum(data)
	return fmt.Sprintf("%x", has)
}

func Base64(str string) string {
	b := []byte(str)
	return base64.StdEncoding.EncodeToString(b)
}
