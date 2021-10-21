package auth

import (
	"io/ioutil"
	htmltpl "html/template"
	"os"
	texttpl "text/template"
)

func readFileText(fn string) (string, error) {
	f, err := os.Open(fn)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func makeTextTemplateFromFile(name, fn string) (Template, error) {
	text, err := readFileText(fn)
	if err != nil {
		return nil, err
	}
	return texttpl.New(name).Parse(text)
}

func makeHtmlTemplateFromFile(name, fn string) (Template, error) {
	text, err := readFileText(fn)
	if err != nil {
		return nil, err
	}
	return htmltpl.New(name).Parse(text)
}

func (cfg *TemplateConfig) GetTemplates() (text, html, sms Template, err error) {
	if cfg.Text != "" {
		text, err = makeTextTemplateFromFile("text", cfg.Text)
		if err != nil {
			return
		}
	}
	if cfg.HTML != "" {
		html, err = makeHtmlTemplateFromFile("html", cfg.HTML)
		if err != nil {
			return
		}
	}
	if cfg.SMS != "" {
		sms, err = makeTextTemplateFromFile("sms", cfg.SMS)
		if err != nil {
			return
		}
	}
	return
}
