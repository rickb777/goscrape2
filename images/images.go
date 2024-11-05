package images

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/url"

	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/gotokit/log"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
)

type ImageQuality int

func (q ImageQuality) CheckImageForRecode(url *url.URL, data []byte) []byte {
	kind, err := filetype.Match(data)
	if err != nil || kind == types.Unknown {
		return data
	}

	logger.Debug("File type detected",
		log.String("type", kind.MIME.Type),
		log.String("sub_type", kind.MIME.Subtype))

	if kind.MIME.Type == matchers.TypeJpeg.MIME.Type && kind.MIME.Subtype == matchers.TypeJpeg.MIME.Subtype {
		if recoded := q.recodeJPEG(url, data); recoded != nil {
			return recoded
		}
		return data
	}

	if kind.MIME.Type == matchers.TypePng.MIME.Type && kind.MIME.Subtype == matchers.TypePng.MIME.Subtype {
		if recoded := q.recodePNG(url, data); recoded != nil {
			return recoded
		}
		return data
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
		return nil
	}

	encoded := q.encodeJPEG(img)
	if encoded == nil || len(encoded) > len(data) { // only use the new file if it is smaller
		return nil
	}

	logger.Debug("Recoded JPEG",
		log.String("url", url.String()),
		log.Int("size_original", len(data)),
		log.Int("size_recoded", len(encoded)))
	return encoded
}

// recodePNG recodes the image and returns it if it is smaller than before.
func (q ImageQuality) recodePNG(url fmt.Stringer, data []byte) []byte {
	inBuf := bytes.NewBuffer(data)
	img, err := png.Decode(inBuf)
	if err != nil {
		return nil
	}

	encoded := q.encodeJPEG(img)
	if encoded == nil || len(encoded) > len(data) { // only use the new file if it is smaller
		return nil
	}

	logger.Debug("Recoded PNG",
		log.String("url", url.String()),
		log.Int("size_original", len(data)),
		log.Int("size_recoded", len(encoded)))
	return encoded
}