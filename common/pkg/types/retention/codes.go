package retention

const (
	Codespace string = "retention_policy"

	CodeOk                 uint32 = 0
	CodeUnknownRequestType uint32 = iota + 4000
	CodeUnsupportedRequest
	CodeUnauthorized
	CodePolicyAlreadySet
	CodePolicyNotSet
	CodeInvalidPolicy
)
