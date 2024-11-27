package filter

import (
	"errors"
	"log/slog"
	"net/url"
	"regexp"

	"github.com/cornelk/goscrape/logger"
)

type Filter []*regexp.Regexp

func New(regexps []string) ([]*regexp.Regexp, error) {
	var errs []error
	var compiled Filter

	for _, exp := range regexps {
		re, err := regexp.Compile(exp)
		if err == nil {
			compiled = append(compiled, re)
		} else {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return compiled, nil
}

func (filter Filter) Present() bool {
	return len(filter) > 0
}

func (filter Filter) Matches(url *url.URL, intent string) bool {
	for _, re := range filter {
		if re.MatchString(url.Path) {
			logger.Debug(intent,
				slog.String("url", url.String()),
				slog.Any("expression", re))
			return true
		}
	}

	return false
}
