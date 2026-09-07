package fixtures

import "time"

func leaks() {
	// ruleid: go-ticker-without-stop
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		_ = 1
	}
}

func deferredStop() {
	// ok: go-ticker-without-stop
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		_ = 1
	}
}

func explicitStop() {
	// ok: go-ticker-without-stop
	ticker := time.NewTicker(time.Second)
	<-ticker.C
	ticker.Stop()
}
