package events

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"testing"
)

var testUuidRaw = "6bf6eb1d-e3cc-4841-85e7-aafc80b80663"

func TestMarshalJSON(t *testing.T) {
	guid := Guid(uuid.MustParse(testUuidRaw))

	marshalled, err := json.Marshal(guid)
	require.NoError(t, err)

	require.Equal(t, `"{6bf6eb1d-e3cc-4841-85e7-aafc80b80663}"`, string(marshalled))
}

func TestUnmarshalJSON(t *testing.T) {
	var guid Guid
	err := json.Unmarshal([]byte(`"{6bf6eb1d-e3cc-4841-85e7-aafc80b80663}"`), &guid)
	require.NoError(t, err)

	require.Equal(t, testUuidRaw, guid.String())
}

func TestMarshalXML(t *testing.T) {
	guid := Guid(uuid.MustParse(testUuidRaw))

	marshalled, err := xml.Marshal(guid)
	require.NoError(t, err)

	require.Equal(t, `<Guid>{6bf6eb1d-e3cc-4841-85e7-aafc80b80663}</Guid>`, string(marshalled))
}

func TestUnmarshalXML(t *testing.T) {
	var guid Guid
	err := xml.Unmarshal([]byte(`<Guid>{6bf6eb1d-e3cc-4841-85e7-aafc80b80663}</Guid>`), &guid)
	require.NoError(t, err)

	require.Equal(t, testUuidRaw, guid.String())
}

func TestMarshalXMLNil(t *testing.T) {
	type testStruct struct {
		Guid *Guid `xml:"Guid"`
	}

	marshalled, err := xml.Marshal(testStruct{})
	require.NoError(t, err)

	require.Equal(t, `<testStruct></testStruct>`, string(marshalled))
}

func TestMarshalXMLAttr(t *testing.T) {
	type testStruct struct {
		Guid *Guid `xml:"Guid,attr"`
	}

	marshalled, err := xml.Marshal(testStruct{Guid: NewGuid(uuid.MustParse(testUuidRaw))})
	require.NoError(t, err)

	require.Equal(t, fmt.Sprintf(`<testStruct Guid="{%s}"></testStruct>`, testUuidRaw), string(marshalled))
}

func TestUnmarshalXMLAttr(t *testing.T) {
	type testStruct struct {
		Guid *Guid `xml:"Guid,attr"`
	}

	var test testStruct
	err := xml.Unmarshal([]byte(fmt.Sprintf(`<testStruct Guid="{%s}"></testStruct>`, testUuidRaw)), &test)
	require.NoError(t, err)

	require.Equal(t, testUuidRaw, test.Guid.String())
}

func TestUnmarshalXMLNilAttr(t *testing.T) {
	type testStruct struct {
		Guid *Guid `xml:"Guid,attr"`
	}

	var test testStruct
	err := xml.Unmarshal([]byte(`<testStruct></testStruct>`), &test)
	require.NoError(t, err)

	require.Nil(t, test.Guid)
}

func TestMarshalUnmarshalBSONGuid(t *testing.T) {
	type testStruct struct {
		Guid Guid `bson:"guid"`
	}

	guid := Guid(uuid.MustParse(testUuidRaw))
	w := testStruct{Guid: guid}

	marshalled, err := bson.Marshal(w)
	require.NoError(t, err)

	var w2 testStruct
	err = bson.Unmarshal(marshalled, &w2)
	require.NoError(t, err)

	require.Equal(t, w, w2)
}
