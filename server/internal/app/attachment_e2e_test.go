//go:build e2e

package app_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func (h *harness) postFiles(channelID uuid.UUID, content string, files map[string][]byte) (int, events.Message) {
	h.t.Helper()

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if content != "" {
		if err := form.WriteField("content", content); err != nil {
			h.t.Fatal(err)
		}
	}
	for name, data := range files {
		part, err := form.CreateFormFile("files", name)
		if err != nil {
			h.t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			h.t.Fatal(err)
		}
	}
	if err := form.Close(); err != nil {
		h.t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPost,
		h.server.URL+"/api/v1/channels/"+channelID.String()+"/messages", &body)
	if err != nil {
		h.t.Fatal(err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+h.token)

	res, err := h.server.Client().Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()

	var msg events.Message
	raw, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(raw, &msg)
	return res.StatusCode, msg
}

func TestAMessageCarriesAFileBackAgain(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Files")
	text, _ := h.textAndVoice(guild.ID)

	status, msg := h.postFiles(text, "here you go", map[string][]byte{
		"notes.txt": []byte("the contents, exactly"),
	})
	if status != http.StatusCreated {
		t.Fatalf("posting with a file returned %d", status)
	}
	if len(msg.Attachments) != 1 {
		t.Fatalf("message carries %d attachments, want 1", len(msg.Attachments))
	}

	got := msg.Attachments[0]
	if got.Filename != "notes.txt" {
		t.Errorf("filename = %q", got.Filename)
	}
	if got.SizeBytes != int64(len("the contents, exactly")) {
		t.Errorf("size = %d, want %d", got.SizeBytes, len("the contents, exactly"))
	}
	if !strings.Contains(got.URL, "sig=") {
		t.Fatalf("url %q carries no signature, so anybody could guess it", got.URL)
	}

	res, err := h.server.Client().Get(h.server.URL + got.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("fetching the attachment returned %d", res.StatusCode)
	}

	served, _ := io.ReadAll(res.Body)
	if string(served) != "the contents, exactly" {
		t.Errorf("served %q; a file must come back byte for byte", served)
	}
	if cd := res.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") {
		t.Errorf("Content-Disposition = %q; a text file served inline can host a page", cd)
	}
	if res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("the attachment was served without nosniff")
	}
}

func TestAnAttachmentIsNotServedWithoutItsSignature(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Private Files")
	text, _ := h.textAndVoice(guild.ID)

	_, msg := h.postFiles(text, "secret", map[string][]byte{"secret.txt": []byte("private")})
	if len(msg.Attachments) != 1 {
		t.Fatal("no attachment came back")
	}

	bare, _, _ := strings.Cut(msg.Attachments[0].URL, "?")
	for _, url := range []string{
		bare,
		bare + "?exp=9999999999&sig=forged",
		bare + "?exp=1&sig=" + strings.Split(msg.Attachments[0].URL, "sig=")[1],
	} {
		res, err := h.server.Client().Get(h.server.URL + url)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode == http.StatusOK {
			t.Errorf("%q was served without a valid signature", url)
		}
	}
}

func TestAFileSurvivesRoundTripUnchanged(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Byte For Byte")
	text, _ := h.textAndVoice(guild.ID)

	original := make([]byte, 4096)
	for i := range original {
		original[i] = byte(i % 251)
	}

	_, msg := h.postFiles(text, "", map[string][]byte{"blob.bin": original})
	if len(msg.Attachments) != 1 {
		t.Fatal("a message with only a file was refused")
	}

	res, err := h.server.Client().Get(h.server.URL + msg.Attachments[0].URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	served, _ := io.ReadAll(res.Body)
	if !bytes.Equal(served, original) {
		t.Error("the file came back changed")
	}
}

func TestDeletingAMessageTakesItsFileWithIt(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Cleanup")
	text, _ := h.textAndVoice(guild.ID)

	_, msg := h.postFiles(text, "temporary", map[string][]byte{"gone.txt": []byte("bye")})
	if len(msg.Attachments) != 1 {
		t.Fatal("no attachment came back")
	}
	url := msg.Attachments[0].URL

	h.mustDo(http.MethodDelete, "/api/v1/messages/"+msg.ID.String(), http.StatusNoContent, nil, nil)

	res, err := h.server.Client().Get(h.server.URL + url)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode == http.StatusOK {
		t.Error("the file outlived the message it belonged to")
	}
}

func TestTooManyFilesAreRefused(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Too Many")
	text, _ := h.textAndVoice(guild.ID)

	files := make(map[string][]byte, 11)
	for i := range 11 {
		files[string(rune('a'+i))+".txt"] = []byte("x")
	}

	status, _ := h.postFiles(text, "lots", files)
	if status != http.StatusBadRequest {
		t.Errorf("eleven files returned %d, want 400", status)
	}
}

func TestAFilenameCannotEscapeItsName(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Nasty Names")
	text, _ := h.textAndVoice(guild.ID)

	_, msg := h.postFiles(text, "odd", map[string][]byte{"../../etc/passwd": []byte("x")})
	if len(msg.Attachments) != 1 {
		t.Fatal("no attachment came back")
	}
	if name := msg.Attachments[0].Filename; strings.Contains(name, "/") || strings.Contains(name, "..") {
		t.Errorf("filename came back as %q, still carrying a path", name)
	}
}
