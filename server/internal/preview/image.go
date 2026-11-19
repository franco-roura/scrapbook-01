package preview

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"strings"

	// Decoders for the formats previews come in.
	_ "image/gif"
	_ "image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	maxImageBytes = 5 << 20
	// ThumbWidth is the widest thumbnail kept; cards are never wider.
	ThumbWidth = 640
	// maxPixels refuses decompression bombs before decoding.
	maxPixels = 40_000_000
)

// Thumbnail downloads imageURL and returns it as a JPEG at most ThumbWidth
// wide, so the copy we keep is small and in one format.
func (f *Fetcher) Thumbnail(ctx context.Context, imageURL string) ([]byte, error) {
	res, err := f.get(ctx, imageURL, "image/webp,image/jpeg,image/png,image/*;q=0.8")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("%w (%s)", ErrNotImage, ct)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxImageBytes {
		return nil, fmt.Errorf("preview: image larger than %d bytes", maxImageBytes)
	}
	return Resize(data, ThumbWidth)
}

// Resize decodes a JPEG, PNG, GIF or WebP image and re-encodes it as JPEG,
// scaled down to at most width pixels wide.
func Resize(data []byte, width int) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotImage, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return nil, fmt.Errorf("preview: image is %dx%d", cfg.Width, cfg.Height)
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotImage, err)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > width {
		h = max(1, h*width/w)
		w = width
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	// White first, so transparent PNGs don't turn black in JPEG.
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
