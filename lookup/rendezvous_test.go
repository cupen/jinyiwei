package lookup

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newList(count int) []string {
	var list = []string{}
	for i := 0; i < count; i++ {
		n1, n2, n3, n4 := i%16, i%32, i%64, i%128
		list = append(list, fmt.Sprintf("%d.%d.%d.%d:%d", n1, n2, n3, n4, i))
	}
	return list
}

func TestLookup(t *testing.T) {
	assert := assert.New(t)
	obj := NewRendezvous(newList(3))
	s := obj.Get("123")
	assert.NotNil(s)
}

func BenchmarkLookup(b *testing.B) {
	runtime.GOMAXPROCS(1)
	for _, size := range []int{1, 2, 3, 10, 100, 1000} {
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
			l := newList(size)
			if len(l) != size {
				b.FailNow()
			}
			obj := NewRendezvous(l)
			expected := obj.Get("123")
			runtime.GC()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				s := obj.Get("123")
				if s != expected {
					b.FailNow()
				}
			}
		})
	}

}
