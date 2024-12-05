package identity

type Principal string

func (p Principal) String() string {
	return string(p)
}

func (p Principal) Bytes() []byte {
	return []byte(p.String())
}
