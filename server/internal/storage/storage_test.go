package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidKeyRejectsAnythingThatEscapes(t *testing.T) {
	for _, key := range []string{
		"",
		"/etc/passwd",
		"../../etc/passwd",
		"avatars/../../../etc/passwd",
		"avatars/..",
		".ssh/id_rsa",
		"avatars/a\x00.png",
		"avatars/a b.png",
		"avatars/a;rm -rf.png",
		strings.Repeat("a", 201),
	} {
		if ValidKey(key) {
			t.Errorf("ValidKey(%q) = true; this key reaches the filesystem from a URL", key)
		}
	}
}

func TestValidKeyAcceptsWhatWeMint(t *testing.T) {
	key, err := NewKey("avatars", "image/png")
	if err != nil {
		t.Fatalf("NewKey: %v", err)
	}
	if !ValidKey(key) {
		t.Errorf("a freshly minted key %q was rejected by ValidKey", key)
	}
	if !strings.HasSuffix(key, ".png") {
		t.Errorf("key %q lost the extension the content type implies", key)
	}
	if ContentTypeOf(key) != "image/png" {
		t.Errorf("ContentTypeOf(%q) = %q", key, ContentTypeOf(key))
	}
}

func TestNewKeyRefusesAContentTypeWeDoNotServe(t *testing.T) {
	if _, err := NewKey("avatars", "application/x-msdownload"); err == nil {
		t.Error("an executable content type was given a key")
	}
}

func TestDiskRoundTrip(t *testing.T) {
	store, err := NewDisk(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	key, err := NewKey("avatars", "image/png")
	if err != nil {
		t.Fatal(err)
	}

	const payload = "not really a png"
	if err := store.Put(ctx, key, "image/png", strings.NewReader(payload), int64(len(payload))); err != nil {
		t.Fatalf("put: %v", err)
	}

	r, err := store.Open(ctx, key)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	got, _ := io.ReadAll(r)
	r.Close()
	if string(got) != payload {
		t.Errorf("read back %q, want %q", got, payload)
	}

	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Open(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete, Open returned %v, want ErrNotFound", err)
	}
}

func TestDiskDeleteOfSomethingGoneIsNotAnError(t *testing.T) {
	store, err := NewDisk(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), "avatars/never-existed.png"); err != nil {
		t.Errorf("deleting a missing object failed: %v", err)
	}
}

func TestDiskRefusesToReadOutsideItsRoot(t *testing.T) {
	root := t.TempDir()
	secret := filepath.Join(filepath.Dir(root), "secret.txt")
	if err := os.WriteFile(secret, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := NewDisk(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(context.Background(), "../secret.txt"); !errors.Is(err, ErrBadKey) {
		t.Errorf("Open escaped the root and returned %v", err)
	}
}

func TestDiskPutIsAllOrNothing(t *testing.T) {
	root := t.TempDir()
	store, err := NewDisk(root)
	if err != nil {
		t.Fatal(err)
	}

	key, _ := NewKey("avatars", "image/png")
	failing := io.MultiReader(strings.NewReader("half"), errorReader{})
	if err := store.Put(context.Background(), key, "image/png", failing, 0); err == nil {
		t.Fatal("a failed upload reported success")
	}

	if _, err := store.Open(context.Background(), key); !errors.Is(err, ErrNotFound) {
		t.Error("a half-written object was left behind under its real name")
	}

	entries, _ := os.ReadDir(filepath.Join(root, "avatars"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".upload-") {
			t.Errorf("temporary file %q was left behind", e.Name())
		}
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("the upload died") }
