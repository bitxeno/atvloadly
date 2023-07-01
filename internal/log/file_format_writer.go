package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
)

type fileFormatWriter struct {
	l io.Writer
}

func newFileFormatWriter(out io.Writer) fileFormatWriter {
	return fileFormatWriter{l: out}
}

func (w fileFormatWriter) Write(p []byte) (n int, err error) {
	var evt map[string]interface{}

	d := json.NewDecoder(bytes.NewReader(p))
	d.UseNumber()
	err = d.Decode(&evt)
	if err != nil {
		return n, fmt.Errorf("cannot decode event: %s", err)
	}

	var msgInfo interface{} = ""
	if evt[zerolog.MessageFieldName] != nil {
		msgInfo = evt[zerolog.MessageFieldName]
	}

	var errInfo interface{} = ""
	if evt[zerolog.ErrorFieldName] != nil {
		errInfo = evt[zerolog.ErrorFieldName]
	}
	level := fmt.Sprintf("[%s]", strings.ToUpper(evt[zerolog.LevelFieldName].(string)))
	newformat := fmt.Sprintf("%-24s %-8s %-16s> %s%s\n", evt[zerolog.TimestampFieldName], level, w.formatCaller(evt[zerolog.CallerFieldName]), msgInfo, errInfo)
	_, err = w.l.Write([]byte(newformat))

	return len(p), err
}

func (w fileFormatWriter) formatCaller(i interface{}) string {
	var c string
	if cc, ok := i.(string); ok {
		c = cc
	}
	if len(c) > 0 {
		if cwd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(cwd, c); err == nil {
				c = rel
			}
		}
	}
	return c
}
