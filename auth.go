package httpserver

import (
	"encoding/hex"
	"math/rand"
	"time"

	"github.com/pkg/errors"
)

type SocialLoginConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

func (cfg *SocialLoginConfig) GetClientID() string {
	return cfg.ClientID
}

func (cfg *SocialLoginConfig) GetClientSecret() string {
	return cfg.ClientSecret
}

type AuthConfig struct {
	PasswordFile string `json:"htpasswd" arg:"--htpasswd"`
	AuthKey      string `json:"key"      arg:"key"`
	TTL          int    `json:"ttl"      arg:"ttl"`
	SocialLogin  map[string]*SocialLoginConfig `json:"social"`
	keyBytes     []byte
}

func (cfg *AuthConfig) Init(serverRoot string) error {
	fn, err := makeRootAbs(serverRoot, cfg.PasswordFile)
	if err != nil {
		return errors.Wrap(err, "can't get abs path for " + cfg.PasswordFile)
	}
	cfg.PasswordFile = fn
	return nil
}

func (cfg *AuthConfig) GetAuthKey() []byte {
	if cfg.keyBytes == nil || len(cfg.keyBytes) != 16 {
		kb, _ := hex.DecodeString(cfg.AuthKey)
		if kb == nil {
			kb = []byte{}
		}
		if len(kb) < 16 {
			pad := make([]byte, 16 - len(kb))
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			rng.Read(pad)
			kb = append(kb, pad...)
		}
		cfg.keyBytes = kb[:16]
	}
	return cfg.keyBytes
}

func (cfg *AuthConfig) GetTTL() time.Duration {
	return time.Duration(cfg.TTL) * time.Second
}
