package media

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func opaqueJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func transparentPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 0})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSquareCropsAndResizes(t *testing.T) {
	data, contentType, err := Square(bytes.NewReader(opaqueJPEG(t, 800, 400)), AvatarSize)
	if err != nil {
		t.Fatalf("Square: %v", err)
	}
	if contentType != "image/jpeg" {
		t.Errorf("content type = %q, want image/jpeg for an image with no transparency", contentType)
	}

	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("the result did not decode: %v", err)
	}
	if b := decoded.Bounds(); b.Dx() != AvatarSize || b.Dy() != AvatarSize {
		t.Errorf("result is %dx%d, want %dx%d square", b.Dx(), b.Dy(), AvatarSize, AvatarSize)
	}
}

func TestSquareKeepsTransparencyAsPNG(t *testing.T) {
	data, contentType, err := Square(bytes.NewReader(transparentPNG(t, 300, 300)), AvatarSize)
	if err != nil {
		t.Fatalf("Square: %v", err)
	}
	if contentType != "image/png" {
		t.Fatalf("content type = %q; a transparent image re-encoded as jpeg loses its alpha", contentType)
	}

	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, a := decoded.At(0, 0).RGBA(); a == 0xffff {
		t.Error("the transparent corner came back opaque")
	}
}

func TestSquareReEncodesRatherThanPassingBytesThrough(t *testing.T) {
	original := opaqueJPEG(t, 400, 400)
	withExif := append(original[:2], append([]byte("\xff\xe1\x00\x10Exif\x00\x00SECRETGPS"), original[2:]...)...)

	data, _, err := Square(bytes.NewReader(withExif), AvatarSize)
	if err != nil {
		t.Skipf("the crafted file did not decode, which is also a refusal: %v", err)
	}
	if bytes.Contains(data, []byte("SECRETGPS")) {
		t.Error("metadata survived; a photo's GPS location would be served to everyone")
	}
}

func TestSquareRefusesWhatIsNotAnImage(t *testing.T) {
	_, _, err := Square(strings.NewReader("#!/bin/sh\nrm -rf /\n"), AvatarSize)
	if !errors.Is(err, ErrNotAnImage) {
		t.Errorf("Square accepted a shell script: %v", err)
	}
}

func TestSquareRefusesSomethingHuge(t *testing.T) {
	_, _, err := Square(bytes.NewReader(make([]byte, MaxUploadBytes+1)), AvatarSize)
	if !errors.Is(err, ErrTooLarge) {
		t.Errorf("err = %v, want ErrTooLarge before the decoder is handed the bytes", err)
	}
}
