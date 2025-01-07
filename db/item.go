package db

import (
	"github.com/rickb777/acceptable/header"
	"strconv"
	"strings"
	"time"
)

// Item is a record in the database.
type Item struct {
	Code     int
	Location string // redirection
	Content  header.ContentType
	ETags    string
	Expires  time.Time
}

func (i Item) EmptyContentType() bool {
	return (i.Content.Type == "" && i.Content.Subtype == "") ||
		(i.Content.Type == "*" && i.Content.Subtype == "*")
}

func (i Item) Empty() bool {
	return i.Code == 0 && i.Location == "" && i.EmptyContentType() && i.ETags == "" && i.Expires.IsZero()
}

func dashIfBlank(s string) string {
	if s == "" {
		s = "-"
	}
	return s
}

func (i Item) Strings() []string {
	ct := "-"
	if i.Content.Type != "" {
		ct = i.Content.String()
	}

	expires := "-"
	if !i.Expires.IsZero() {
		expires = i.Expires.Format(time.RFC3339)
	}

	return []string{
		strconv.Itoa(i.Code),
		dashIfBlank(i.Location),
		ct,
		expires,
		dashIfBlank(i.ETags),
	}
}

func (i Item) String() string {
	return strings.Join(i.Strings(), "\t")
}

func parseItem(line string) (string, Item) {
	parts := strings.Split(line, "\t")

	if len(parts) != 6 {
		return "", Item{}
	}

	key := parts[0]
	v1, _ := strconv.Atoi(parts[1])
	v2 := parts[2]
	v3 := parts[3]
	v4 := parts[4]
	v5 := parts[5]

	var ct header.ContentType
	if v3 != "-" {
		ct = header.ParseContentType(v3)
	}

	var expires time.Time
	if v4 != "-" {
		// time.Parse conveniently returns the zero value on error
		expires, _ = time.Parse(time.RFC3339, v4)
	}

	return key, Item{
		Code:     v1,
		Location: strNotDash(v2),
		Content:  ct,
		Expires:  expires,
		ETags:    strNotDash(v5),
	}

}
