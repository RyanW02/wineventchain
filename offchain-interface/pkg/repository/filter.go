package repository

import (
	"fmt"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/common/pkg/utils"
	"github.com/google/uuid"
	"strconv"
	"strings"
	"time"
)

type (
	Filter struct {
		Property FilterProperty `json:"property"`
		Operator Operator       `json:"operator"`
		Value    string         `json:"value"`
	}

	FilterProperty string
	Operator       string
)

const (
	PropertyEventId      FilterProperty = "event_id"
	PropertyTxHash       FilterProperty = "tx_hash"
	PropertyPrincipal    FilterProperty = "principal"
	PropertyEventType    FilterProperty = "event_type_id"
	PropertyTimestamp    FilterProperty = "timestamp"
	ProviderName         FilterProperty = "provider_name"
	PropertyProviderGuid FilterProperty = "provider_guid"
	PropertyCorrelation  FilterProperty = "correlation"
	PropertyChannel      FilterProperty = "channel"

	OperatorEqual  Operator = "eq"
	OperatorAfter  Operator = "after"
	OperatorBefore Operator = "before"
)

func (f *Filter) Matches(event events.StoredEvent) (bool, error) {
	switch f.Property {
	case PropertyEventId:
		if f.Operator != OperatorEqual {
			return false, ErrInvalidFilter
		}

		return f.Value == event.Metadata.EventId.String(), nil
	case PropertyTxHash:
		if f.Operator != OperatorEqual {
			return false, ErrInvalidFilter
		}

		return f.Value == event.TxHash.String(), nil
	case PropertyPrincipal:
		if f.Operator != OperatorEqual {
			return false, ErrInvalidFilter
		}

		return f.Value == event.Metadata.Principal.String(), nil
	case PropertyEventType:
		if f.Operator != OperatorEqual {
			return false, ErrInvalidFilter
		}

		eventType, err := strconv.Atoi(f.Value)
		if err != nil {
			return false, err
		}

		return eventType == int(event.EventWithData.System.EventId), nil
	case PropertyTimestamp:
		timestamp, err := time.Parse(utils.HtmlDateTimeFormat, f.Value)
		if err != nil {
			return false, fmt.Errorf("%w: invalid timestamp", ErrInvalidFilter)
		}

		switch f.Operator {
		case OperatorAfter:
			return event.Metadata.ReceivedTime.After(timestamp), nil
		case OperatorBefore:
			return event.Metadata.ReceivedTime.Before(timestamp), nil
		default:
			return false, ErrInvalidFilter
		}
	case ProviderName:
		if f.Operator != OperatorEqual {
			return false, ErrInvalidFilter
		}

		return event.EventWithData.System.Provider.Name != nil &&
			strings.ToLower(f.Value) == strings.ToLower(*event.EventWithData.System.Provider.Name), nil
	case PropertyProviderGuid:
		if f.Operator != OperatorEqual {
			return false, ErrInvalidFilter
		}

		uuid, err := uuid.Parse(f.Value)
		if err != nil {
			return false, fmt.Errorf("%w: invalid provider GUID", ErrInvalidFilter)
		}

		return event.EventWithData.System.Provider.Guid != nil &&
			uuid == event.EventWithData.System.Provider.Guid.UUID(), nil
	case PropertyCorrelation:
		if f.Operator != OperatorEqual {
			return false, ErrInvalidFilter
		}

		uuid, err := uuid.Parse(f.Value)
		if err != nil {
			return false, fmt.Errorf("%w: invalid correlation GUID", ErrInvalidFilter)
		}

		return event.EventWithData.System.Correlation.ActivityId != nil &&
			uuid == event.EventWithData.System.Correlation.ActivityId.UUID(), nil
	case PropertyChannel:
		if f.Operator != OperatorEqual {
			return false, ErrInvalidFilter
		}

		return strings.ToLower(f.Value) == strings.ToLower(event.EventWithData.System.Channel), nil
	default:
		return false, ErrInvalidFilter
	}
}
