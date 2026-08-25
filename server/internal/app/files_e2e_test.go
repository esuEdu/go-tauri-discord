//go:build e2e

package app_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func aPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (h *harness) putImage(path string, body []byte, contentType string) (int, map[string]string) {
	h.t.Helper()

	req, err := http.NewRequest(http.MethodPut, h.server.URL+path, bytes.NewReader(body))
	if err != nil {
		h.t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}

	res, err := h.server.Client().Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()

	out := map[string]string{}
	raw, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(raw, &out)
	return res.StatusCode, out
}

func TestAnAvatarCanBeSetAndIsServedBack(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	status, body := h.putImage("/api/v1/users/@me/avatar", aPNG(t, 400, 200), "image/png")
	if status != http.StatusOK {
		t.Fatalf("upload returned %d, want 200", status)
	}

	key := body["avatar_key"]
	if key == "" {
		t.Fatal("upload returned no avatar key")
	}

	res, err := h.server.Client().Get(h.server.URL + "/api/v1/files/" + key)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("fetching the avatar returned %d", res.StatusCode)
	}

	served, _ := io.ReadAll(res.Body)
	decoded, _, err := image.Decode(bytes.NewReader(served))
	if err != nil {
		t.Fatalf("what came back was not an image: %v", err)
	}
	if b := decoded.Bounds(); b.Dx() != b.Dy() {
		t.Errorf("served image is %dx%d, want a square", b.Dx(), b.Dy())
	}

	var me struct {
		AvatarKey *string `json:"avatar_key"`
	}
	h.mustDo(http.MethodGet, "/api/v1/users/@me", http.StatusOK, nil, &me)
	if me.AvatarKey == nil || *me.AvatarKey != key {
		t.Errorf("the account still reports avatar %v, want %q", me.AvatarKey, key)
	}
}

func TestReplacingAnAvatarDoesNotLeaveTheOldOneServable(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	_, first := h.putImage("/api/v1/users/@me/avatar", aPNG(t, 300, 300), "image/png")
	_, second := h.putImage("/api/v1/users/@me/avatar", aPNG(t, 260, 260), "image/png")

	if first["avatar_key"] == second["avatar_key"] {
		t.Fatal("the replacement reused the same key, so caches would keep the old picture")
	}

	res, err := h.server.Client().Get(h.server.URL + "/api/v1/files/" + first["avatar_key"])
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("the replaced avatar is still served (%d); every upload would leak an object",
			res.StatusCode)
	}
}

func TestUploadRefusesWhatIsNotAnImage(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	status, _ := h.putImage("/api/v1/users/@me/avatar",
		[]byte("<?php system($_GET['c']); ?>"), "image/png")
	if status != http.StatusBadRequest {
		t.Errorf("a php file claiming to be a png returned %d, want 400", status)
	}
}

func TestFilesCannotBeReadOutsideTheStore(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	for _, key := range []string{"../../../etc/passwd", "..%2f..%2fetc%2fpasswd"} {
		res, err := h.server.Client().Get(h.server.URL + "/api/v1/files/" + key)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode == http.StatusOK {
			t.Errorf("%q was served (%d bytes)", key, len(body))
		}
	}
}

func TestOnlySomebodyWithManageGuildSetsTheIcon(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Iconic")
	member := owner.inviteMember(guild.ID)

	status, _ := member.putImage("/api/v1/guilds/"+guild.ID.String()+"/icon",
		aPNG(t, 200, 200), "image/png")
	if status != http.StatusForbidden {
		t.Errorf("an ordinary member setting the icon returned %d, want 403", status)
	}

	status, body := owner.putImage("/api/v1/guilds/"+guild.ID.String()+"/icon",
		aPNG(t, 200, 200), "image/png")
	if status != http.StatusOK {
		t.Fatalf("the owner setting the icon returned %d", status)
	}
	if body["icon_key"] == "" {
		t.Fatal("the owner's upload returned no icon key")
	}

	var mine []struct {
		ID      uuid.UUID `json:"id"`
		IconKey *string   `json:"icon_key"`
	}
	owner.mustDo(http.MethodGet, "/api/v1/guilds", http.StatusOK, nil, &mine)
	for _, g := range mine {
		if g.ID == guild.ID && (g.IconKey == nil || *g.IconKey != body["icon_key"]) {
			t.Errorf("the guild lists icon %v, want %q", g.IconKey, body["icon_key"])
		}
	}
}
