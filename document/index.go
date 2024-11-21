package document

import (
	"github.com/cornelk/goscrape/htmlindex"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/work"
	"log/slog"
)

func (d *HTMLDocument) FindReferences() (work.Refs, error) {
	var result work.Refs
	for tag := range htmlindex.Nodes {
		references, err := d.index.URLs(tag)
		if err != nil {
			logger.Error("Getting node URLs failed",
				slog.String("url", d.u.String()),
				slog.String("node", tag.String()),
				slog.Any("error", err))
		}

		for _, ur := range references {
			ur.Fragment = ""
			result = append(result, ur)
		}
	}

	return result, nil
}
