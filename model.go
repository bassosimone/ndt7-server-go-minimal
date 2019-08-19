package main

// ReceiverInfo contains a receiver measurement.
type ReceiverInfo struct {
	// ElapsedSeconds contains the number of elapsed seconds.
	ElapsedSeconds float64

	// NumBytes contains the number of received bytes.
	NumBytes       int64
}
