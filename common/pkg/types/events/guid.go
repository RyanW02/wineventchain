package events

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

// Guid is a globally unique identifier. It is a string of the form {UUID}.
type Guid uuid.UUID

// Enforce compile-time interface conformance
var (
	_ json.Marshaler        = (*Guid)(nil)
	_ json.Unmarshaler      = (*Guid)(nil)
	_ xml.Marshaler         = (*Guid)(nil)
	_ xml.Unmarshaler       = (*Guid)(nil)
	_ bson.ValueMarshaler   = (*Guid)(nil)
	_ bson.ValueUnmarshaler = (*Guid)(nil)
)

func NewGuid(uuid uuid.UUID) *Guid {
	guid := Guid(uuid)
	return &guid
}

func (g Guid) UUID() uuid.UUID {
	return uuid.UUID(g)
}

func (g Guid) String() string {
	return g.UUID().String()
}

func (g Guid) MarshalJSON() ([]byte, error) {
	return []byte(`"{` + g.String() + `}"`), nil
}

func (g *Guid) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	// Check length - include quotes and braces
	if len(data) != 40 {
		return errors.New("invalid GUID length")
	}

	// Remove the quotes and the curly braces and parse the UUID
	u, err := uuid.ParseBytes(data[2:38])
	if err != nil {
		return err
	}

	*g = Guid(u)
	return nil
}

func (g Guid) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	s := fmt.Sprintf("{%s}", g.String())
	return e.EncodeElement(s, start)
}

func (g Guid) MarshalXMLAttr(name xml.Name) (xml.Attr, error) {
	return xml.Attr{Name: name, Value: fmt.Sprintf("{%s}", g.String())}, nil
}

func (g *Guid) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw string
	if err := d.DecodeElement(&raw, &start); err != nil {
		return err
	}

	if raw == "" {
		return nil
	}

	parsed, err := uuid.Parse(raw)
	if err != nil {
		return err
	}

	*g = Guid(parsed)
	return nil
}

func (g *Guid) UnmarshalXMLAttr(attr xml.Attr) error {
	if attr.Value == "" {
		return nil
	}

	parsed, err := uuid.Parse(attr.Value)
	if err != nil {
		return err
	}

	*g = Guid(parsed)
	return nil
}

func (g Guid) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return bson.MarshalValue(fmt.Sprintf("{%s}", g.String()))
}

func (g *Guid) UnmarshalBSONValue(t bsontype.Type, bytes []byte) error {
	if t == bson.TypeNull {
		return nil
	}

	var s string
	if err := bson.UnmarshalValue(t, bytes, &s); err != nil {
		return err
	}

	// Check length - include quotes and braces
	if len(s) != 38 {
		return errors.New("invalid GUID length")
	}

	// Remove the curly braces and parse the UUID
	u, err := uuid.Parse(s[1:37])
	if err != nil {
		return err
	}

	*g = Guid(u)
	return nil
}
