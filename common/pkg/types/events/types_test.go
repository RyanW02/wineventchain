package events

import (
	"encoding/xml"
	"github.com/RyanW02/wineventchain/common/pkg/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var (
	TestEvent = EventWithData{
		Event: Event{
			System: System{
				Provider: Provider{
					Name:            utils.Ptr("Test-Provider"),
					Guid:            NewGuid(uuid.MustParse("69884110-5b41-41cc-93c7-02ce8e8882f6")),
					EventSourceName: utils.Ptr("Name2"),
				},
				EventId: 5379,
				TimeCreated: TimeCreated{
					SystemTime: time.Unix(0, 0).UTC(),
				},
				EventRecordId: 12743445,
				Correlation: Correlation{
					ActivityId: NewGuid(uuid.MustParse("d4c3abe3-54f8-4467-9b26-b8da06ae52b1")),
				},
				Execution: Execution{
					ProcessId: utils.Ptr(1234),
					ThreadId:  utils.Ptr(12005),
				},
				Channel:  "Security",
				Computer: "laptop",
			},
		},
		EventData: EventData{
			Data{
				Name:  utils.Ptr("SubjectUserName"),
				Value: utils.Ptr("user"),
			},
			Data{
				Name:  utils.Ptr("SubjectDomainName"),
				Value: utils.Ptr("laptop"),
			},
		},
	}

	TestXml = `
<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">
  <System>
    <Provider Name="Test-Provider" Guid="{69884110-5b41-41cc-93c7-02ce8e8882f6}" EventSourceName="Name2" /> 
    <EventID>5379</EventID> 
    <Version>0</Version> 
    <Level>0</Level> 
    <Task>12743445</Task> 
    <Opcode>0</Opcode> 
    <Keywords>0x8020000000000000</Keywords> 
    <TimeCreated SystemTime="1970-01-01T00:00:00.00000000Z" /> 
    <EventRecordID>12743445</EventRecordID> 
    <Correlation ActivityID="{d4c3abe3-54f8-4467-9b26-b8da06ae52b1}" /> 
    <Execution ProcessID="1234" ThreadID="12005" /> 
    <Channel>Security</Channel> 
    <Computer>laptop</Computer> 
    <Security />
  </System>
  <EventData>
    <Data Name="SubjectUserName">user</Data> 
    <Data Name="SubjectDomainName">laptop</Data> 
  </EventData>
</Event>`
)

func TestUnmarshalXml(t *testing.T) {
	var event EventWithData
	require.NoError(t, xml.Unmarshal([]byte(TestXml), &event), "failed to unmarshal xml")

	require.Equal(t, TestEvent, event, "unmarshalled event does not match expected event")
}
