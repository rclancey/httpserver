package auth

import (
	H "github.com/rclancey/httpserver/v2"
)

func (cfg *TemplateConfig) Init(serverRoot string) error {
	if cfg.Text != "" {
		fn, err := H.MakeRootAbs(serverRoot, cfg.Text)
		if err != nil {
			return err
		}
		cfg.Text = fn
	}
	if cfg.HTML != "" {
		fn, err := H.MakeRootAbs(serverRoot, cfg.HTML)
		if err != nil {
			return err
		}
		cfg.HTML = fn
	}
	if cfg.SMS != "" {
		fn, err := H.MakeRootAbs(serverRoot, cfg.SMS)
		if err != nil {
			return err
		}
		cfg.SMS = fn
	}
	return nil
}

func (cfg *AuthConfig) Init(serverRoot string) error {
	err := cfg.ResetTemplate.Init(serverRoot)
	if err != nil {
		return err
	}
	err = cfg.TwoFactorTemplate.Init(serverRoot)
	if err != nil {
		return err
	}
	return nil
}
