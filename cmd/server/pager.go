package main

import (
	"net/http"
	"net/url"
	"strconv"
)

const (
	defaultLimit = 200
	maxLimit     = 1000
)

// Pager provides current page information.
type Pager struct {
	Total   int
	Current int
	Prev    int
	Next    int
	Head    int
	Tail    int
	Queries url.Values
}

func page(r *http.Request) int {
	page := 1
	pageParam, ok := r.URL.Query()["page"]
	if !ok {
		pageParam = []string{strconv.Itoa(page)}
	}
	if len(pageParam) > 0 {
		var err error
		if page, err = strconv.Atoi(pageParam[0]); err != nil {
			return 1
		}
		if page < 1 {
			return 1
		}
	}
	return page
}

func limit(r *http.Request) int {
	limit := defaultLimit
	limitParam, ok := r.URL.Query()["rows"]
	if !ok {
		limitParam = []string{strconv.Itoa(limit)}
	}
	if len(limitParam) > 0 {
		var err error
		if limit, err = strconv.Atoi(limitParam[0]); err != nil {
			return defaultLimit
		}
		if limit < 1 {
			return defaultLimit
		}
		if limit > maxLimit {
			return maxLimit
		}
	}
	return limit
}

func newPager(total, subtotal, page, limit int, queries url.Values) Pager {
	p := Pager{total, page, page - 1, page + 1, 0, 0, queries}
	if total > 0 {
		p.Head = (page-1)*limit + 1
		p.Tail = p.Head + subtotal - 1
	}
	return p
}
