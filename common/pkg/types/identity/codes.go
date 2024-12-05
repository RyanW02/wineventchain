package identity

const (
	Codespace string = "identity"

	CodeOk                 uint32 = 0
	CodeUnknownRequestType uint32 = iota + 1000
	CodeInvalidSignature
	CodeAlreadySeeded
	CodeUnauthorized
	CodeNotFound
	CodePrincipalAlreadyExists
	CodeTreeUninitialized
	CodeUnknownError
)
