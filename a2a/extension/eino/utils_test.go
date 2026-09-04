/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package eino

import (
	"context"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/a2a/models"
)

func strPtr(s string) *string { return &s }

func TestMessage2Parts(t *testing.T) {
	t.Run("nil message", func(t *testing.T) {
		if got := message2Parts(nil); got != nil {
			t.Errorf("nil msg: got %+v, want nil", got)
		}
	})
	t.Run("plain content", func(t *testing.T) {
		got := message2Parts(&schema.Message{Content: "hello"})
		if len(got) != 1 || got[0].Kind != models.PartKindText || got[0].Text == nil || *got[0].Text != "hello" {
			t.Errorf("text content: got %+v", got)
		}
	})
	t.Run("multi content image", func(t *testing.T) {
		msg := &schema.Message{MultiContent: []schema.ChatMessagePart{
			{Type: schema.ChatMessagePartTypeText, Text: "intro"},
			{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: "https://x/img.png", MIMEType: "image/png"}},
		}}
		got := message2Parts(msg)
		if len(got) != 2 {
			t.Fatalf("len: got %d, want 2 (parts=%+v)", len(got), got)
		}
		if got[0].Kind != models.PartKindText || got[0].Text == nil || *got[0].Text != "intro" {
			t.Errorf("text part: got %+v", got[0])
		}
		if got[1].Kind != models.PartKindFile || got[1].File == nil || got[1].File.URI == nil || *got[1].File.URI != "https://x/img.png" {
			t.Errorf("file part: got %+v", got[1])
		}
	})
	t.Run("empty message returns nil", func(t *testing.T) {
		if got := message2Parts(&schema.Message{}); got != nil {
			t.Errorf("empty: got %+v, want nil", got)
		}
	})
}

func TestMessages2Parts(t *testing.T) {
	got, err := messages2Parts(context.Background(), []*schema.Message{
		{Content: "a"},
		{Content: "b"},
		{}, // empty message contributes nothing
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len: got %d, want 2 (parts=%+v)", len(got), got)
	}
	if *got[0].Text != "a" || *got[1].Text != "b" {
		t.Errorf("texts: got [%q, %q]", *got[0].Text, *got[1].Text)
	}
}

func TestToFileParts(t *testing.T) {
	cases := []struct {
		name    string
		uri     string
		wantURI bool
	}{
		{"http URI", "http://x/y", true},
		{"https URI", "https://x/y", true},
		{"ftp URI", "ftp://x/y", true},
		{"raw bytes", "iVBORw0KGgo=", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := toFileParts("image/png", c.uri)
			if p.Kind != models.PartKindFile {
				t.Fatalf("kind: got %v, want File", p.Kind)
			}
			if c.wantURI {
				if p.File.URI == nil || *p.File.URI != c.uri || p.File.Bytes != nil {
					t.Errorf("expected URI set for %q, got %+v", c.uri, p.File)
				}
			} else {
				if p.File.Bytes == nil || *p.File.Bytes != c.uri || p.File.URI != nil {
					t.Errorf("expected Bytes set for %q, got %+v", c.uri, p.File)
				}
			}
			if p.File.MimeType != "image/png" {
				t.Errorf("mime: got %q", p.File.MimeType)
			}
		})
	}
}

func TestParts2Content(t *testing.T) {
	t.Run("all text concatenates into Content", func(t *testing.T) {
		text, mc := parts2Content([]models.Part{
			{Kind: models.PartKindText, Text: strPtr("foo ")},
			{Kind: models.PartKindText, Text: strPtr("bar")},
		})
		if text != "foo bar" {
			t.Errorf("content: got %q, want %q", text, "foo bar")
		}
		if mc != nil {
			t.Errorf("multi content should be nil, got %+v", mc)
		}
	})
	t.Run("mixed content uses MultiContent", func(t *testing.T) {
		text, mc := parts2Content([]models.Part{
			{Kind: models.PartKindText, Text: strPtr("see image:")},
			{Kind: models.PartKindFile, File: &models.FileContent{URI: strPtr("https://x/img.png"), MimeType: "image/png"}},
		})
		if text != "" {
			t.Errorf("content should be empty for mixed, got %q", text)
		}
		if len(mc) != 2 {
			t.Fatalf("multi content len: got %d, want 2", len(mc))
		}
		if mc[0].Type != schema.ChatMessagePartTypeText || mc[0].Text != "see image:" {
			t.Errorf("text mc[0]: got %+v", mc[0])
		}
		if mc[1].Type != schema.ChatMessagePartTypeImageURL || mc[1].ImageURL == nil || mc[1].ImageURL.URL != "https://x/img.png" {
			t.Errorf("image mc[1]: got %+v", mc[1])
		}
	})
	t.Run("file mime dispatch", func(t *testing.T) {
		_, mc := parts2Content([]models.Part{
			{Kind: models.PartKindFile, File: &models.FileContent{URI: strPtr("u1"), MimeType: "audio/mp3"}},
			{Kind: models.PartKindFile, File: &models.FileContent{URI: strPtr("u2"), MimeType: "video/mp4"}},
			{Kind: models.PartKindFile, File: &models.FileContent{URI: strPtr("u3"), MimeType: "application/pdf"}},
		})
		if len(mc) != 3 {
			t.Fatalf("len: got %d", len(mc))
		}
		if mc[0].Type != schema.ChatMessagePartTypeAudioURL || mc[0].AudioURL == nil {
			t.Errorf("audio: got %+v", mc[0])
		}
		if mc[1].Type != schema.ChatMessagePartTypeVideoURL || mc[1].VideoURL == nil {
			t.Errorf("video: got %+v", mc[1])
		}
		if mc[2].Type != schema.ChatMessagePartTypeFileURL || mc[2].FileURL == nil {
			t.Errorf("generic file: got %+v", mc[2])
		}
	})
}

func TestToADKMessage(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := toADKMessage(nil); got != nil {
			t.Errorf("nil: got %+v, want nil", got)
		}
	})
	t.Run("role mapping and IDs", func(t *testing.T) {
		taskID := "t-1"
		ctxID := "c-1"
		msg := &models.Message{
			Role:      models.RoleAgent,
			MessageID: "m-1",
			TaskID:    &taskID,
			ContextID: &ctxID,
			Parts:     []models.Part{{Kind: models.PartKindText, Text: strPtr("hi")}},
			Metadata:  map[string]any{"k": "v"},
		}
		got := toADKMessage(msg)
		if got.Role != schema.Assistant {
			t.Errorf("role: got %v, want %v", got.Role, schema.Assistant)
		}
		if got.Content != "hi" {
			t.Errorf("content: got %q", got.Content)
		}
		if id, ok := GetMessageID(got); !ok || id != "m-1" {
			t.Errorf("messageID extra: got id=%q ok=%v", id, ok)
		}
		if id, ok := GetTaskID(got); !ok || id != "t-1" {
			t.Errorf("taskID extra: got id=%q ok=%v", id, ok)
		}
		if id, ok := GetContextID(got); !ok || id != "c-1" {
			t.Errorf("contextID extra: got id=%q ok=%v", id, ok)
		}
		if got.Extra["k"] != "v" {
			t.Errorf("metadata copied to Extra: got %+v", got.Extra)
		}
	})
	t.Run("user role mapping", func(t *testing.T) {
		got := toADKMessage(&models.Message{Role: models.RoleUser, Parts: []models.Part{{Kind: models.PartKindText, Text: strPtr("q")}}})
		if got.Role != schema.User {
			t.Errorf("role: got %v, want %v", got.Role, schema.User)
		}
	})
}

func TestToADKMessages(t *testing.T) {
	got := toADKMessages([]*models.Message{
		{Role: models.RoleAgent, Parts: []models.Part{{Kind: models.PartKindText, Text: strPtr("a")}}},
		{Role: models.RoleUser, Parts: []models.Part{{Kind: models.PartKindText, Text: strPtr("b")}}},
	})
	if len(got) != 2 {
		t.Fatalf("len: got %d", len(got))
	}
	if got[0].Role != schema.Assistant || got[0].Content != "a" {
		t.Errorf("[0]: %+v", got[0])
	}
	if got[1].Role != schema.User || got[1].Content != "b" {
		t.Errorf("[1]: %+v", got[1])
	}
}

func TestArtifact2ADKMessage(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := artifact2ADKMessage(nil); got != nil {
			t.Errorf("nil: got %+v, want nil", got)
		}
	})
	t.Run("populates Role, content, and ID", func(t *testing.T) {
		got := artifact2ADKMessage(&models.Artifact{
			ArtifactID: "art-1",
			Parts:      []models.Part{{Kind: models.PartKindText, Text: strPtr("piece")}},
			Metadata:   map[string]any{"src": "tool"},
		})
		if got.Role != schema.Assistant {
			t.Errorf("role: got %v, want Assistant", got.Role)
		}
		if got.Content != "piece" {
			t.Errorf("content: got %q", got.Content)
		}
		if id, ok := GetArtifactID(got); !ok || id != "art-1" {
			t.Errorf("artifactID: got id=%q ok=%v", id, ok)
		}
		if got.Extra["src"] != "tool" {
			t.Errorf("metadata copied: got %+v", got.Extra)
		}
	})
	t.Run("nil metadata does not panic", func(t *testing.T) {
		got := artifact2ADKMessage(&models.Artifact{
			ArtifactID: "art-2",
			Parts:      []models.Part{{Kind: models.PartKindText, Text: strPtr("x")}},
		})
		if got == nil {
			t.Fatal("got nil")
		}
		// Extra may have only the artifact ID
		want := map[string]any{extraKeyOfArtifactID: "art-2"}
		if !reflect.DeepEqual(got.Extra, want) {
			t.Errorf("extra: got %+v, want %+v", got.Extra, want)
		}
	})
}
