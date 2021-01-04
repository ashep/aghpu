package mail

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"mime"
	"os"
	"path/filepath"
)

// Message is the message
type Message struct {
	sender      string
	recipients  []string
	subject     string
	body        string
	attachments []map[string]interface{}
}

// NewMessage creates a new message
func NewMessage(from string, to []string, subject, body string) *Message {
	msg := &Message{
		sender:      from,
		recipients:  to,
		subject:     subject,
		body:        body,
		attachments: make([]map[string]interface{}, 0),
	}

	return msg
}

// AddRecipient adds a recipient to the message
func (m *Message) AddRecipient(rcpt string) {
	m.recipients = append(m.recipients, rcpt)
}

// Attach attaches a file to the message
func (m *Message) Attach(fPath string) error {
	fp, err := os.Open(fPath)
	if err != nil {
		return fmt.Errorf("cannot open file %v: %v", fPath, err)
	}

	fBody, err := ioutil.ReadAll(fp)
	if err != nil {
		return fmt.Errorf("cannot read file %v: %v", fPath, err)
	}
	fp.Close()

	b := make([]byte, base64.StdEncoding.EncodedLen(len(fBody)))
	base64.StdEncoding.Encode(b, fBody)

	_, fName := filepath.Split(fPath)

	m.attachments = append(m.attachments, map[string]interface{}{
		"name": fName,
		"mime": mime.TypeByExtension(filepath.Ext(fPath)),
		"body": b,
	})

	return nil
}
