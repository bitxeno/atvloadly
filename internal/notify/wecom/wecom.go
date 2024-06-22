package wecom

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/silenceper/wechat/v2/cache"
	"github.com/silenceper/wechat/v2/work"
	"github.com/silenceper/wechat/v2/work/config"
	"github.com/silenceper/wechat/v2/work/message"
)

// wecomMessageClient abstracts message.Client for writing unit tests
//
//go:generate mockery --name=wecomMessageClient --output=. --case=underscore --inpackage
type wecomMessageClient interface {
	SendText(request message.SendTextRequest) (*message.SendResponse, error)
}

// Config is the Service configuration.
type Config struct {
	CorpID     string
	CorpSecret string
	AgentID    string

	Token          string
	RasPrivateKey  string
	EncodingAESKey string
	Cache          cache.Cache
}

// Service encapsulates the WeCom client along with internal state for storing users.
type Service struct {
	config        *Config
	messageClient wecomMessageClient
	toUser        []string
	toParty       []string
	toTag         []string
}

// New returns a new instance of a WeCom notification service.
func New(cfg *Config) *Service {
	wcCfg := &config.Config{
		CorpID:         cfg.CorpID,
		CorpSecret:     cfg.CorpSecret,
		AgentID:        cfg.AgentID,
		Cache:          cfg.Cache,
		RasPrivateKey:  cfg.RasPrivateKey,
		Token:          cfg.Token,
		EncodingAESKey: cfg.EncodingAESKey,
	}
	wc := work.NewWork(wcCfg)

	return &Service{
		config:        cfg,
		messageClient: wc.GetMessage(),
		toUser:        []string{},
		toParty:       []string{},
		toTag:         []string{},
	}
}

// AddReceivers takes user ids and adds them to the internal users list. The Send method will send
// a given message to all those users.
func (s *Service) AddReceivers(toUser ...string) {
	s.toUser = append(s.toUser, toUser...)
}

func (s *Service) AddPartyReceivers(toParty ...string) {
	s.toParty = append(s.toParty, toParty...)
}

func (s *Service) AddTagReceivers(toTag ...string) {
	s.toTag = append(s.toTag, toTag...)
}

// Send takes a message subject and a message content and sends them to all previously set users.
func (s *Service) Send(ctx context.Context, subject, content string) error {
	toUser := strings.Join(s.toUser, "|")
	toParty := strings.Join(s.toParty, "|")
	toTag := strings.Join(s.toTag, "|")

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		text := fmt.Sprintf("%s\n\n%s", subject, content)
		_, err := s.messageClient.SendText(message.SendTextRequest{
			SendRequestCommon: &message.SendRequestCommon{ToUser: toUser, ToParty: toParty, ToTag: toTag, AgentID: s.config.AgentID},
			Text:              message.TextField{Content: text},
		})
		if err != nil {
			return errors.Wrapf(err, "failed to send message to WeCom user '%s' and party '%s' and tag '%s'", toUser, toParty, toTag)
		}
	}

	return nil
}
