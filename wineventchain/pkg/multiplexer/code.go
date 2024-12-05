package multiplexer

const Codespace string = "multiplex"

const (
	CodeOk uint32 = iota
	CodeUnknownError
	CodeEncodingError
	CodeUnknownApp
	CodeInvalidValidatorTx
)
