package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/mikkims/ya/internal/storage"
)

func BenchmarkShortener(b *testing.B) {
	ctx := context.Background()
	b.Run("Get", func(b *testing.B) {
		store := storage.NewMemory()
		if err := store.Save(ctx, "id", "https://example.com"); err != nil {
			b.Fatal(err)
		}
		s := NewShortener(store)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			v, ok, err := s.Get(ctx, "id")
			if err != nil || !ok || v == "" {
				b.Fatal("get failed")
			}
		}
	})
	b.Run("Save", func(b *testing.B) {
		s := NewShortener(storage.NewMemory())
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if i%1000 == 0 {
				b.StopTimer()
				s = NewShortener(storage.NewMemory())
				b.StartTimer()
			}
			id, err := s.Save(ctx, fmt.Sprintf("https://example.com/%d", i%1000))
			if err != nil || id == "" {
				b.Fatal(err)
			}
		}
	})
	for _, size := range []int{1, 100, 1000} {
		b.Run(fmt.Sprintf("SaveBatch/%d", size), func(b *testing.B) {
			urls := make([]string, size)
			for i := range urls {
				urls[i] = fmt.Sprintf("https://example.com/%d", i)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				s := NewShortener(storage.NewMemory())
				b.StartTimer()
				ids, err := s.SaveBatch(ctx, urls)
				if err != nil || len(ids) != size {
					b.Fatal(err)
				}
			}
		})
	}
}
