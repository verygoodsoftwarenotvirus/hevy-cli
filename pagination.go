package hevy

import (
	"context"
	"iter"
)

// pageFetcher fetches a single page and returns the items, total page count, and any error.
type pageFetcher[T any] func(ctx context.Context, page int) (items []T, pageCount int, err error)

// fetchAllPages returns an iterator that lazily fetches pages on demand.
func fetchAllPages[T any](ctx context.Context, fetch pageFetcher[T]) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		page := 1
		for {
			items, pageCount, err := fetch(ctx, page)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			if page >= pageCount {
				return
			}
			page++
		}
	}
}

// Collect drains an iter.Seq2[T, error] into a slice, returning the first error encountered.
func Collect[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var result []T
	for item, err := range seq {
		if err != nil {
			return result, err
		}
		result = append(result, item)
	}
	return result, nil
}
