package main

const (
	// SpecInitialBinaryMessageSizeExponent is the power-of-2 exponent of the
	// initial binary message size.
	SpecInitialBinaryMessageSizeExponent = 13

	// SpecInitialBinaryMessageSize is the initial binary message size
	SpecInitialBinaryMessageSize = 1 << SpecInitialBinaryMessageSizeExponent

	// SpecMaxBinaryMessageSizeExponent is the power-of-2 exponent of the
	// maximum binary message size.
	SpecMaxBinaryMessageSizeExponent = 24

	// SpecMaxTextMessageSize is the maximum textual message size
	SpecMaxTextMessageSize = 1 << 10

	// SpecMaxBinaryMessageSize is the maximum binary message size
	SpecMaxBinaryMessageSize = 1 << SpecMaxBinaryMessageSizeExponent

	// SpecWebSocketProtocol is the websocket protocol name used by ndt7
	SpecWebSocketProtocol = "net.measurementlab.ndt.v7"
)
