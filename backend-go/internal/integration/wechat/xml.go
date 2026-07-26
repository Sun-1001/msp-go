package wechat

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	maxUserNameBytes     = 256
	maxMessageTypeBytes  = 32
	maxEventTypeBytes    = 64
	maxMessageIDBytes    = 32
	maxEventKeyBytes     = 2048
	maxTicketBytes       = 2048
	maxPassiveReplyBytes = 2048
)

// IncomingMessage represents the common fields sent by an Official Account.
// Message IDs remain strings so their exact unsigned representation is retained.
type IncomingMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgID        string   `xml:"MsgId"`
	MsgDataID    string   `xml:"MsgDataId"`
	Idx          string   `xml:"Idx"`
	Event        string   `xml:"Event"`
	EventKey     string   `xml:"EventKey"`
	Ticket       string   `xml:"Ticket"`
	Encrypt      string   `xml:"Encrypt"`

	PicURL       string  `xml:"PicUrl"`
	MediaID      string  `xml:"MediaId"`
	Format       string  `xml:"Format"`
	Recognition  string  `xml:"Recognition"`
	ThumbMediaID string  `xml:"ThumbMediaId"`
	LocationX    float64 `xml:"Location_X"`
	LocationY    float64 `xml:"Location_Y"`
	Scale        int     `xml:"Scale"`
	Label        string  `xml:"Label"`
	Title        string  `xml:"Title"`
	Description  string  `xml:"Description"`
	URL          string  `xml:"Url"`
	Latitude     float64 `xml:"Latitude"`
	Longitude    float64 `xml:"Longitude"`
	Precision    float64 `xml:"Precision"`
	MenuID       string  `xml:"MenuId"`
}

// EncryptedEnvelope contains the authenticated ciphertext wrapper used by
// compatibility and safe callback modes.
type EncryptedEnvelope struct {
	XMLName    xml.Name `xml:"xml"`
	ToUserName string   `xml:"ToUserName"`
	Encrypt    string   `xml:"Encrypt"`
}

type cdataValue struct {
	Value string `xml:",cdata"`
}

type textReplyXML struct {
	XMLName      xml.Name   `xml:"xml"`
	ToUserName   cdataValue `xml:"ToUserName"`
	FromUserName cdataValue `xml:"FromUserName"`
	CreateTime   int64      `xml:"CreateTime"`
	MsgType      cdataValue `xml:"MsgType"`
	Content      cdataValue `xml:"Content"`
}

type encryptedReplyXML struct {
	XMLName   xml.Name   `xml:"xml"`
	Encrypt   cdataValue `xml:"Encrypt"`
	Signature cdataValue `xml:"MsgSignature"`
	Timestamp int64      `xml:"TimeStamp"`
	Nonce     cdataValue `xml:"Nonce"`
}

// ParseIncomingXML strictly parses one plaintext or decrypted WeChat message.
// Callers must apply an HTTP body limit before passing payload to this function.
func ParseIncomingXML(payload []byte) (IncomingMessage, error) {
	var message IncomingMessage
	if err := decodeSingleXML(payload, &message); err != nil {
		return IncomingMessage{}, err
	}
	message.MsgType = strings.ToLower(strings.TrimSpace(message.MsgType))
	message.Event = strings.ToLower(strings.TrimSpace(message.Event))
	if err := validateIncomingMessage(message); err != nil {
		return IncomingMessage{}, err
	}
	return message, nil
}

// ParseEncryptedEnvelopeXML strictly parses one AES callback wrapper.
func ParseEncryptedEnvelopeXML(payload []byte) (EncryptedEnvelope, error) {
	var envelope EncryptedEnvelope
	if err := decodeSingleXML(payload, &envelope); err != nil {
		return EncryptedEnvelope{}, err
	}
	if !validBoundedText(envelope.ToUserName, 1, maxUserNameBytes) ||
		envelope.Encrypt == "" || len(envelope.Encrypt) > maxEncryptedTextBytes {
		return EncryptedEnvelope{}, ErrInvalidMessage
	}
	return envelope, nil
}

// BuildTextReply builds a plaintext passive text reply with recipient and
// sender automatically reversed from the incoming message.
func BuildTextReply(incoming IncomingMessage, content string, timestamp int64) ([]byte, error) {
	if !validBoundedText(incoming.FromUserName, 1, maxUserNameBytes) ||
		!validBoundedText(incoming.ToUserName, 1, maxUserNameBytes) ||
		timestamp <= 0 || !validBoundedText(content, 1, maxPassiveReplyBytes) {
		return nil, ErrInvalidMessage
	}
	reply := textReplyXML{
		ToUserName:   cdataValue{Value: incoming.FromUserName},
		FromUserName: cdataValue{Value: incoming.ToUserName},
		CreateTime:   timestamp,
		MsgType:      cdataValue{Value: "text"},
		Content:      cdataValue{Value: content},
	}
	return marshalXML(reply)
}

// BuildEncryptedReply encrypts a passive reply and wraps it in the XML format
// required by WeChat compatibility and safe modes.
func (p *Protocol) BuildEncryptedReply(plaintext []byte, timestamp int64, nonce string) ([]byte, error) {
	if timestamp <= 0 || nonce == "" || len(nonce) > maxNonceBytes || !utf8.ValidString(nonce) {
		return nil, ErrInvalidMessage
	}
	encrypted, err := p.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	timestampText := fmt.Sprintf("%d", timestamp)
	signatureParts := []string{p.token, timestampText, nonce, encrypted}
	digest := calculateSHA1Signature(signatureParts...)
	reply := encryptedReplyXML{
		Encrypt:   cdataValue{Value: encrypted},
		Signature: cdataValue{Value: fmt.Sprintf("%x", digest)},
		Timestamp: timestamp,
		Nonce:     cdataValue{Value: nonce},
	}
	return marshalXML(reply)
}

func validateIncomingMessage(message IncomingMessage) error {
	if !validBoundedText(message.ToUserName, 1, maxUserNameBytes) ||
		!validBoundedText(message.FromUserName, 1, maxUserNameBytes) ||
		message.CreateTime <= 0 ||
		!validBoundedText(message.MsgType, 1, maxMessageTypeBytes) {
		return ErrInvalidMessage
	}
	if len(message.Content) > maxPlaintextBytes || !utf8.ValidString(message.Content) {
		return ErrInvalidMessage
	}
	if message.MsgID != "" && (!decimalText(message.MsgID) || len(message.MsgID) > maxMessageIDBytes) {
		return ErrInvalidMessage
	}
	if message.MsgDataID != "" && (!decimalText(message.MsgDataID) || len(message.MsgDataID) > maxMessageIDBytes) {
		return ErrInvalidMessage
	}
	if message.Idx != "" && (!decimalText(message.Idx) || len(message.Idx) > maxMessageIDBytes) {
		return ErrInvalidMessage
	}
	if message.MsgType == "event" && !validBoundedText(message.Event, 1, maxEventTypeBytes) {
		return ErrInvalidMessage
	}
	if len(message.EventKey) > maxEventKeyBytes || len(message.Ticket) > maxTicketBytes {
		return ErrInvalidMessage
	}
	return nil
}

func decodeSingleXML(payload []byte, target any) error {
	if len(payload) == 0 || !utf8.Valid(payload) {
		return ErrInvalidXML
	}
	if err := validateXMLDocument(payload); err != nil {
		return err
	}
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	decoder.Strict = true
	if err := decoder.Decode(target); err != nil {
		return ErrInvalidXML
	}
	return nil
}

func validateXMLDocument(payload []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	decoder.Strict = true
	depth := 0
	rootCount := 0
	seenNonWhitespace := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ErrInvalidXML
		}
		switch value := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				rootCount++
				if rootCount != 1 || value.Name.Space != "" || value.Name.Local != "xml" {
					return ErrInvalidXML
				}
			}
			depth++
			seenNonWhitespace = true
		case xml.EndElement:
			depth--
			if depth < 0 {
				return ErrInvalidXML
			}
		case xml.Directive:
			return ErrInvalidXML
		case xml.ProcInst:
			if seenNonWhitespace || value.Target != "xml" {
				return ErrInvalidXML
			}
			seenNonWhitespace = true
		case xml.CharData:
			if depth == 0 && len(bytes.TrimSpace(value)) != 0 {
				return ErrInvalidXML
			}
		case xml.Comment:
			if depth == 0 && rootCount > 0 {
				seenNonWhitespace = true
			}
		}
	}
	if depth != 0 || rootCount != 1 {
		return ErrInvalidXML
	}
	return nil
}

func marshalXML(value any) ([]byte, error) {
	payload, err := xml.Marshal(value)
	if err != nil {
		return nil, ErrInvalidMessage
	}
	return payload, nil
}

func validBoundedText(value string, minBytes, maxBytes int) bool {
	length := len(value)
	return length >= minBytes && length <= maxBytes && utf8.ValidString(value)
}

func decimalText(value string) bool {
	if value == "" {
		return false
	}
	for _, digit := range []byte(value) {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
