package eino

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/a2a/models"
)

func messages2Parts(_ context.Context, messages []adk.Message) ([]models.Part, error) {
	var ret []models.Part
	for _, m := range messages {
		parts := message2Parts(m)
		if len(parts) > 0 {
			ret = append(ret, parts...)
		}
	}
	return ret, nil
}

func message2Parts(m adk.Message) []models.Part {
	if m == nil {
		return nil
	}
	// ignore reasoning_content
	// ignore tool_call
	if len(m.Content) > 0 {
		text := m.Content
		return []models.Part{{Kind: models.PartKindText, Text: &text}}
	}

	if len(m.MultiContent) > 0 {
		ret := make([]models.Part, 0, len(m.MultiContent))
		for _, content := range m.MultiContent {
			switch content.Type {
			case schema.ChatMessagePartTypeText:
				text := content.Text
				ret = append(ret, models.Part{Kind: models.PartKindText, Text: &text})
			case schema.ChatMessagePartTypeImageURL:
				if content.ImageURL != nil {
					// todo: how to specify bytes or uri?
					ret = append(ret, toFileParts(content.ImageURL.MIMEType, content.ImageURL.URL))
				}
			case schema.ChatMessagePartTypeAudioURL:
				if content.AudioURL != nil {
					// todo: how to specify bytes or uri?
					ret = append(ret, toFileParts(content.AudioURL.MIMEType, content.AudioURL.URL))
				}
			case schema.ChatMessagePartTypeVideoURL:
				if content.VideoURL != nil {
					// todo: how to specify bytes or uri?
					ret = append(ret, toFileParts(content.VideoURL.MIMEType, content.VideoURL.URL))
				}
			case schema.ChatMessagePartTypeFileURL:
				if content.FileURL != nil {
					// todo: how to specify bytes or uri?
					ret = append(ret, toFileParts(content.FileURL.MIMEType, content.FileURL.URL))
				}
			default:
			}
		}

		return ret
	}

	return nil
}

func toFileParts(mimeType, uri string) models.Part {
	p := models.Part{Kind: models.PartKindFile, File: &models.FileContent{MimeType: mimeType}}
	if strings.HasPrefix(uri, "http") ||
		strings.HasPrefix(uri, "ftp") {
		p.File.URI = &uri
	} else {
		p.File.Bytes = &uri
	}
	return p
}

func toADKMessages(ms []*models.Message) []adk.Message {
	ret := make([]adk.Message, 0, len(ms))
	for _, m := range ms {
		ret = append(ret, toADKMessage(m))
	}
	return ret
}

func toADKMessage(m *models.Message) adk.Message {
	if m == nil {
		return nil
	}
	ret := &schema.Message{}
	switch m.Role {
	case models.RoleAgent:
		ret.Role = schema.Assistant
	case models.RoleUser:
		ret.Role = schema.User
	}

	ret.Content, ret.MultiContent = parts2Content(m.Parts)

	SetMessageID(ret, m.MessageID)
	if m.ContextID != nil {
		SetContextID(ret, *m.ContextID)
	}
	if m.TaskID != nil {
		SetTaskID(ret, *m.TaskID)
	}
	for k, v := range m.Metadata {
		ret.Extra[k] = v
	}
	return ret
}

func artifact2ADKMessage(a *models.Artifact) *schema.Message {
	if a == nil {
		return nil
	}

	ret := &schema.Message{
		Role: schema.Assistant,
	}

	ret.Content, ret.MultiContent = parts2Content(a.Parts)

	SetArtifactID(ret, a.ArtifactID)
	for k, v := range a.Metadata {
		ret.Extra[k] = v
	}
	return ret
}

func parts2Content(parts []models.Part) (string, []schema.ChatMessagePart) {
	mc := make([]schema.ChatMessagePart, 0, len(parts))
	allText := true
	for _, part := range parts {
		switch part.Kind {
		case models.PartKindText:
			if part.Text != nil {
				mc = append(mc, schema.ChatMessagePart{
					Type: schema.ChatMessagePartTypeText,
					Text: *part.Text,
				})
			}
		case models.PartKindFile:
			allText = false
			if part.File != nil {
				var url string
				if part.File.URI != nil {
					url = *part.File.URI
				}
				if part.File.Bytes != nil {
					url = *part.File.Bytes
				}

				if strings.HasPrefix(part.File.MimeType, "image/") {
					mc = append(mc, schema.ChatMessagePart{
						Type: schema.ChatMessagePartTypeImageURL,
						ImageURL: &schema.ChatMessageImageURL{
							URL:      url,
							MIMEType: part.File.MimeType,
						},
					})

				} else if strings.HasPrefix(part.File.MimeType, "audio/") {
					mc = append(mc, schema.ChatMessagePart{
						Type: schema.ChatMessagePartTypeAudioURL,
						AudioURL: &schema.ChatMessageAudioURL{
							URL:      url,
							MIMEType: part.File.MimeType,
						},
					})

				} else if strings.HasPrefix(part.File.MimeType, "video/") {
					mc = append(mc, schema.ChatMessagePart{
						Type: schema.ChatMessagePartTypeVideoURL,
						VideoURL: &schema.ChatMessageVideoURL{
							URL:      url,
							MIMEType: part.File.MimeType,
						},
					})
				} else {
					mc = append(mc, schema.ChatMessagePart{
						Type: schema.ChatMessagePartTypeFileURL,
						FileURL: &schema.ChatMessageFileURL{
							URL:      url,
							MIMEType: part.File.MimeType,
						},
					})
				}
			}
		default:
			// todo: how to handle PartKindData
		}
	}
	if allText {
		content := ""
		for _, c := range mc {
			content += c.Text
		}
		return content, nil
	}
	return "", mc
}
