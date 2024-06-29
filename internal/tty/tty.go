package tty

import (
	"fmt"
	"strings"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/gofiber/contrib/websocket"
)

type TTY struct {
	conn    *websocket.Conn
	pl      *PipeLine
	cwd     string
	environ []string
}

func New(conn *websocket.Conn, cmd string) (*TTY, error) {
	pl, err := NewPipeLine(conn, cmd)
	if err != nil {
		return nil, err
	}

	return &TTY{
		conn: conn,
		pl:   pl,
	}, nil
}

func (t *TTY) SetCWD(cwd string) {
	t.cwd = cwd
}

func (t *TTY) SetENV(environ []string) {
	t.environ = environ
}

func (t *TTY) Close() {
	fmt.Println("tty close")
	t.pl.Close()
}

func (t *TTY) Start() {
	if t.cwd != "" {
		if _, err := t.pl.pty.Write([]byte(fmt.Sprintf("cd \"%s\"\n", app.Config.Server.DataDir))); err != nil {
			_ = t.conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
			return
		}
	}
	for _, v := range t.environ {
		if _, err := t.pl.pty.Write([]byte(fmt.Sprintf("export %s\n", v))); err != nil {
			_ = t.conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
			return
		}
	}

	logChan := make(chan string)
	go t.pl.ReadSktAndWritePty(logChan)
	go t.pl.ReadPtyAndWriteSkt(logChan)

	errlog := <-logChan
	fmt.Println(errlog)
	go func() {
		<-logChan
		close(logChan)
	}()
}

func (t *TTY) RunCommand(args []string) error {
	c := fmt.Sprintf("%s\n", strings.Join(args, " "))
	_, err := t.pl.pty.Write([]byte(c))
	return err
}
