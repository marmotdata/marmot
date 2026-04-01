package imageproc

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"

	"golang.org/x/image/webp"
)

const (
	maxDimension = 10000
	jpegQuality  = 90
)

var (
	ErrDimensionsTooLarge = errors.New("image dimensions exceed maximum of 10000px")
	ErrUnsupportedFormat  = errors.New("unsupported image format")
	ErrDecodeFailed       = errors.New("failed to decode image")
	ErrEncodeFailed       = errors.New("failed to encode image")
)

type SanitizeResult struct {
	Data        []byte
	ContentType string
}

// SanitizeImage re-encodes an image to strip any non-image payloads and
// validates that its dimensions do not exceed the maximum allowed size.
func SanitizeImage(data []byte, declaredContentType string) (*SanitizeResult, error) {
	detected := http.DetectContentType(data)

	switch detected {
	case "image/jpeg":
		return sanitizeJPEG(data)
	case "image/png":
		return sanitizePNG(data)
	case "image/gif":
		return sanitizeGIF(data)
	case "image/webp":
		return sanitizeWebP(data)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, detected)
	}
}

func checkDimensions(img image.Image) error {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w > maxDimension || h > maxDimension {
		return fmt.Errorf("%w: %dx%d", ErrDimensionsTooLarge, w, h)
	}
	return nil
}

func sanitizeJPEG(data []byte) (*SanitizeResult, error) {
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecodeFailed, err)
	}

	if err := checkDimensions(img); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncodeFailed, err)
	}

	return &SanitizeResult{Data: buf.Bytes(), ContentType: "image/jpeg"}, nil
}

func sanitizePNG(data []byte) (*SanitizeResult, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecodeFailed, err)
	}

	if err := checkDimensions(img); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncodeFailed, err)
	}

	return &SanitizeResult{Data: buf.Bytes(), ContentType: "image/png"}, nil
}

func sanitizeGIF(data []byte) (*SanitizeResult, error) {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecodeFailed, err)
	}

	// Check dimensions of each frame
	for _, frame := range g.Image {
		if err := checkDimensions(frame); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncodeFailed, err)
	}

	return &SanitizeResult{Data: buf.Bytes(), ContentType: "image/gif"}, nil
}

func sanitizeWebP(data []byte) (*SanitizeResult, error) {
	img, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecodeFailed, err)
	}

	if err := checkDimensions(img); err != nil {
		return nil, err
	}

	// No Go WebP encoder exists; re-encode as PNG (lossless for pixel data)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncodeFailed, err)
	}

	return &SanitizeResult{Data: buf.Bytes(), ContentType: "image/png"}, nil
}
