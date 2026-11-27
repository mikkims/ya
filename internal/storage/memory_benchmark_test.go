package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/mikkims/ya/internal/auth"
)

func BenchmarkMemory(b *testing.B) {
	ctx := auth.WithUserID(context.Background(), "user")
	for _, size := range []int{1, 100, 1000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			store := NewMemory()
			for i := 0; i < size; i++ {
				key := fmt.Sprint(i)
				if err := store.Save(ctx, key, "https://example.com/"+key); err != nil {
					b.Fatal(err)
				}
			}
			b.Run("Get", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					v, ok, err := store.Get(ctx, "0")
					if err != nil || !ok || v == "" {
						b.Fatal("get failed")
					}
				}
			})
			b.Run("GetByUser", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					v, err := store.GetByUser(ctx, "user")
					if err != nil || len(v) != size {
						b.Fatal("list failed")
					}
				}
			})
			b.Run("Save", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if i%1000 == 0 {
						b.StopTimer()
						store = NewMemory()
						b.StartTimer()
					}
					key := fmt.Sprint(i % 1000)
					if err := store.Save(ctx, key, "https://example.com/"+key); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
