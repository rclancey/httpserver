package auth

import (
	"io/ioutil"
	htmltpl "html/template"
	"os"
	texttpl "text/template"

	H "github.com/rclancey/httpserver"
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

func (cfg *TemplateConfig) GetTemplates(serverRoot string) (text, html, sms Template, err error) {
	var fn string
	if cfg.Text != "" {
		fn, err = H.MakeRootAbs(serverRoot, cfg.Text)
		if err != nil {
			return
		}
		text, err = makeTextTemplateFromFile("text", fn)
		if err != nil {
			return
		}
	}
	if cfg.HTML != "" {
		fn, err = H.MakeRootAbs(serverRoot, cfg.HTML)
		if err != nil {
			return
		}
		html, err = makeHtmlTemplateFromFile("html", fn)
		if err != nil {
			return
		}
	}
	if cfg.SMS != "" {
		fn, err = H.MakeRootAbs(serverRoot, cfg.SMS)
		if err != nil {
			return
		}
		sms, err = makeTextTemplateFromFile("sms", fn)
		if err != nil {
			return
		}
	}
	return
}

