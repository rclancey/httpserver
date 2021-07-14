package auth

import (
	"bytes"

	"github.com/pkg/errors"
)

func (a *Authenticator) sendMessage(user User, subject string, data interface{}, sms, text, html Template) error {
	eu, euok := user.(EmailUser)
	pu, puok := user.(PhoneUser)
	if euok && a.EmailClient != nil && text != nil {
		textBuf := &bytes.Buffer{}
		err := text.Execute(textBuf, data)
		if err != nil {
			return errors.Wrap(err, "error generating email text")
		}
		textContent := string(textBuf.Bytes())
		var htmlContent *string
		if html != nil {
			htmlBuf := &bytes.Buffer{}
			err := html.Execute(htmlBuf, data)
			if err != nil {
				return errors.Wrap(err, "error generating email html")
			}
			htmlStr := string(htmlBuf.Bytes())
			htmlContent = &htmlStr
		}
		err = a.EmailClient.Send(a.EmailSender, eu.GetEmailAddress(), subject, textContent, htmlContent)
		if err != nil {
			return errors.Wrap(err, "error sending reset email")
		}
		return nil
	}
	if puok && a.SMSClient != nil && sms != nil {
		textBuf := &bytes.Buffer{}
		err := sms.Execute(textBuf, data)
		if err != nil {
			return errors.Wrap(err, "error generating sms text")
		}
		textContent := string(textBuf.Bytes())
		err = a.SMSClient.Send(pu.GetPhoneNumber(), textContent)
		if err != nil {
			return errors.Wrap(err, "error sending sms")
		}
		return nil
	}
	return errors.New("no way to send message")
}
