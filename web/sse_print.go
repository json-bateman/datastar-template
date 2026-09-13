// sse_print.go
package web

import (
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

func ssePrintMessage(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r, datastar.WithCompression(datastar.WithBrotli()))

	ticker := time.NewTicker(time.Millisecond * 300)
	defer ticker.Stop()

	s := "Aloha Travelers"
	t := 0

	for {
		if err := sse.PatchElementTempl(Home(s[:t])); err != nil {
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if len(s) <= t {
				return
			}
			t++
		}
	}
}
