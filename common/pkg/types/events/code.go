package events

const Codespace = "events"

const (
	CodeOk uint32 = iota
	CodeUnknownError
	CodeInvalidQueryPath
	CodeEventNotFound
	CodeTreeUninitialized
)

const (
	EventCreate        = "create"
	AttributeType      = "type"
	AttributeEventId   = "event_id"
	AttributeBlockTime = "block_time"
	AttributePrincipal = "principal"

	AttributeValueCreate = "create"
)
