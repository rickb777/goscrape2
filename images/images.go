package images

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log/slog"
	"net/url"

	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	"github.com/rickb777/goscrape2/logger"
)

type ImageQuality int

func (q ImageQuality) CheckImageForRecode(url *url.URL, data []byte) []byte {
	kind, err := filetype.Match(data)
	if err != nil || kind == types.Unknown {
		return data
	}

	if kind.MIME.Type == matchers.TypeJpeg.MIME.Type && kind.MIME.Subtype == matchers.TypeJpeg.MIME.Subtype {
		return q.recodeJPEG(url, data)
	}

	if kind.MIME.Type == matchers.TypePng.MIME.Type && kind.MIME.Subtype == matchers.TypePng.MIME.Subtype {
		return q.recodePNG(url, data)
	}

	return data
}

// encodeJPEG encodes a new JPG based on the given quality setting.
func (q ImageQuality) encodeJPEG(img image.Image) []byte {
	o := &jpeg.Options{
		Quality: int(q),
	}

	outBuf := &bytes.Buffer{}
	if err := jpeg.Encode(outBuf, img, o); err != nil {
		return nil
	}
	return outBuf.Bytes()
}

// recodeJPEG recodes the image and returns it if it is smaller than before.
func (q ImageQuality) recodeJPEG(url fmt.Stringer, data []byte) []byte {
	inBuf := bytes.NewBuffer(data)
	img, err := jpeg.Decode(inBuf)
	if err != nil {
		return data
	}

	encoded := q.encodeJPEG(img)
	if encoded == nil || len(encoded) > len(data) { // only use the new file if it is smaller
		return data
	}

	logger.Debug("Recoded JPEG",
		slog.String("url", url.String()),
		slog.Int("size_original", len(data)),
		slog.Int("size_recoded", len(encoded)))
	return encoded
}

// recodePNG recodes the image and returns it if it is smaller than before.
func (q ImageQuality) recodePNG(url fmt.Stringer, data []byte) []byte {
	inBuf := bytes.NewBuffer(data)
	img, err := png.Decode(inBuf)
	if err != nil {
		return data
	}

	encoded := q.encodeJPEG(img)
	if encoded == nil || len(encoded) > len(data) { // only use the new file if it is smaller
		return data
	}

	logger.Debug("Recoded PNG",
		slog.String("url", url.String()),
		slog.Int("size_original", len(data)),
		slog.Int("size_recoded", len(encoded)))
	return encoded
}
