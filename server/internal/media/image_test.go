package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
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

func pngClaiming(t *testing.T, w, h uint32) []byte {
	t.Helper()

	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header[0:4], w)
	binary.BigEndian.PutUint32(header[4:8], h)
	header[8] = 8
	header[9] = 6

	chunk := append([]byte("IHDR"), header...)

	var buf bytes.Buffer
	buf.Write([]byte("\x89PNG\r\n\x1a\n"))
	binary.Write(&buf, binary.BigEndian, uint32(len(header)))
	buf.Write(chunk)
	binary.Write(&buf, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return buf.Bytes()
}

func TestSquareRefusesAPixelBombBeforeDecoding(t *testing.T) {
	bomb := pngClaiming(t, 20000, 20000)
	if len(bomb) > 1024 {
		t.Fatalf("the crafted header is %d bytes; it must stay far under MaxUploadBytes to prove the byte cap is not what stops it", len(bomb))
	}

	_, _, err := Square(bytes.NewReader(bomb), AvatarSize)
	if !errors.Is(err, ErrTooManyPixels) {
		t.Fatalf("err = %v, want ErrTooManyPixels; 20000x20000 RGBA is 1.6 GB of heap for a 256px avatar", err)
	}
}

func TestSquareStillAcceptsAnOrdinaryPhoto(t *testing.T) {
	if _, _, err := Square(bytes.NewReader(opaqueJPEG(t, 4000, 3000)), AvatarSize); err != nil {
		t.Errorf("Square refused a 12 megapixel photo, which is an ordinary phone camera upload: %v", err)
	}
}
