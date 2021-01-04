package mail

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/smtp"
	"strings"
)

// Send sends an email message
func Send(addr string, auth smtp.Auth, msg *Message) error {

	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	bnd := writer.Boundary()

	buf.WriteString(fmt.Sprintf("From: %s\r\n", msg.sender))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.recipients, ",")))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.subject))
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%q\r\n", bnd))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	buf.WriteString("\r\n")

	buf.WriteString(fmt.Sprintf("--%s\r\n", bnd))
	buf.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	buf.WriteString("Content-Disposition: inline\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(fmt.Sprintf("%s\r\n", msg.body))

	for _, att := range msg.attachments {
		buf.WriteString(fmt.Sprintf("--%s\r\n", bnd))
		buf.WriteString(fmt.Sprintf("Content-Type: %s\r\n", att["mime"]))
		buf.WriteString("Content-Transfer-Encoding: base64\r\n")
		buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", att["name"]))
		buf.WriteString("\r\n")
		buf.Write(att["body"].([]byte))
		buf.WriteString("\r\n")
	}

	buf.WriteString(fmt.Sprintf("\r\n--%s--\r\n", bnd))

	// fmt.Printf("%s", buf)

	if err := smtp.SendMail(addr, auth, msg.sender, msg.recipients, buf.Bytes()); err != nil {
		return err
	}

	return nil
}
