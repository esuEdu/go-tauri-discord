package media

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"

	xdraw "golang.org/x/image/draw"

	_ "image/gif"

	_ "golang.org/x/image/webp"
)

const (
	MaxUploadBytes = 5 << 20
	AvatarSize     = 256
	IconSize       = 256
)

var ErrNotAnImage = errors.New("media: that file is not an image we can read")

var ErrTooLarge = errors.New("media: that file is larger than the limit")

func Square(r io.Reader, side int) ([]byte, string, error) {
	limited := io.LimitReader(r, MaxUploadBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", err
	}
	if len(raw) > MaxUploadBytes {
		return nil, "", ErrTooLarge
	}

	source, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", ErrNotAnImage
	}

	square := xdraw.CatmullRom
	cropped := centreSquare(source)
	out := image.NewRGBA(image.Rect(0, 0, side, side))
	square.Scale(out, out.Bounds(), cropped, cropped.Bounds(), draw.Src, nil)

	if transparent(out) {
		var buf bytes.Buffer
		if err := png.Encode(&buf, out); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/png", nil
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 85}); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/jpeg", nil
}

func centreSquare(src image.Image) image.Image {
	bounds := src.Bounds()
	side := min(bounds.Dx(), bounds.Dy())

	left := bounds.Min.X + (bounds.Dx()-side)/2
	top := bounds.Min.Y + (bounds.Dy()-side)/2
	rect := image.Rect(left, top, left+side, top+side)

	if sub, ok := src.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok {
		return sub.SubImage(rect)
	}

	out := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(out, out.Bounds(), src, rect.Min, draw.Src)
	return out
}

func transparent(img *image.RGBA) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a < 0xffff {
				return true
			}
		}
	}
	return false
}
