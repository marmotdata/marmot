package imageproc

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

func createTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{G: 255, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func createTestGIF(t *testing.T, w, h, frames int) []byte {
	t.Helper()
	g := &gif.GIF{}
	for range frames {
		palette := color.Palette{color.Black, color.White}
		img := image.NewPaletted(image.Rect(0, 0, w, h), palette)
		g.Image = append(g.Image, img)
		g.Delay = append(g.Delay, 10)
	}
	var buf bytes.Buffer
	require.NoError(t, gif.EncodeAll(&buf, g))
	return buf.Bytes()
}

func TestSanitizeJPEG(t *testing.T) {
	data := createTestJPEG(t, 100, 80)
	result, err := SanitizeImage(data, "image/jpeg")
	require.NoError(t, err)
	assert.Equal(t, "image/jpeg", result.ContentType)
	assert.NotEmpty(t, result.Data)

	// Verify round-trip produces a valid JPEG
	img, err := jpeg.Decode(bytes.NewReader(result.Data))
	require.NoError(t, err)
	assert.Equal(t, 100, img.Bounds().Dx())
	assert.Equal(t, 80, img.Bounds().Dy())
}

func TestSanitizePNG(t *testing.T) {
	data := createTestPNG(t, 50, 60)
	result, err := SanitizeImage(data, "image/png")
	require.NoError(t, err)
	assert.Equal(t, "image/png", result.ContentType)

	img, err := png.Decode(bytes.NewReader(result.Data))
	require.NoError(t, err)
	assert.Equal(t, 50, img.Bounds().Dx())
	assert.Equal(t, 60, img.Bounds().Dy())
}

func TestSanitizeGIF(t *testing.T) {
	data := createTestGIF(t, 30, 30, 1)
	result, err := SanitizeImage(data, "image/gif")
	require.NoError(t, err)
	assert.Equal(t, "image/gif", result.ContentType)

	g, err := gif.DecodeAll(bytes.NewReader(result.Data))
	require.NoError(t, err)
	require.Len(t, g.Image, 1)
	assert.Equal(t, 30, g.Image[0].Bounds().Dx())
}

func TestSanitizeGIFAnimated(t *testing.T) {
	data := createTestGIF(t, 20, 20, 3)
	result, err := SanitizeImage(data, "image/gif")
	require.NoError(t, err)
	assert.Equal(t, "image/gif", result.ContentType)

	g, err := gif.DecodeAll(bytes.NewReader(result.Data))
	require.NoError(t, err)
	assert.Len(t, g.Image, 3)
}

func TestDimensionsTooLarge(t *testing.T) {
	// Create a PNG with dimensions exceeding the limit
	// We use a small buffer with a crafted PNG header to avoid allocating huge memory
	data := createTestPNG(t, 10001, 1)
	_, err := SanitizeImage(data, "image/png")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDimensionsTooLarge)
}

func TestDimensionsTooLargeHeight(t *testing.T) {
	data := createTestPNG(t, 1, 10001)
	_, err := SanitizeImage(data, "image/png")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDimensionsTooLarge)
}

func TestDimensionsAtLimit(t *testing.T) {
	data := createTestPNG(t, 10000, 10000)
	result, err := SanitizeImage(data, "image/png")
	require.NoError(t, err)
	assert.Equal(t, "image/png", result.ContentType)
}

func TestCorruptData(t *testing.T) {
	// Data that starts with JPEG magic bytes but is corrupt
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}
	_, err := SanitizeImage(data, "image/jpeg")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDecodeFailed)
}

func TestUnsupportedFormat(t *testing.T) {
	data := []byte("this is not an image at all")
	_, err := SanitizeImage(data, "image/png")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsupportedFormat)
}

func TestPolyglotStripping(t *testing.T) {
	// Create a valid PNG and append a JavaScript payload
	data := createTestPNG(t, 10, 10)
	polyglot := make([]byte, len(data)+len("<script>alert('xss')</script>"))
	copy(polyglot, data)
	copy(polyglot[len(data):], "<script>alert('xss')</script>")

	result, err := SanitizeImage(polyglot, "image/png")
	require.NoError(t, err)
	assert.Equal(t, "image/png", result.ContentType)

	// Re-encoded output should not contain the injected payload
	assert.NotContains(t, string(result.Data), "script")
	assert.NotEqual(t, polyglot, result.Data)
}
