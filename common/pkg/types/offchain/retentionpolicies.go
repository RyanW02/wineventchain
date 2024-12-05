package offchain

import (
	"fmt"
	"github.com/RyanW02/wineventchain/common/pkg/types"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/pkg/errors"
	"time"
)

type (
	StoredPolicy struct {
		Policy    RetentionPolicy    `json:"policy"`
		Author    identity.Principal `json:"author"`
		AppliedAt time.Time          `json:"applied_at"`
	}

	RetentionPolicy struct {
		Filters []Filter `yaml:"filters" json:"filters"`
	}

	Filter struct {
		Label        string       `yaml:"label" json:"label"`
		Match        Match        `yaml:"match" json:"match"`
		PolicyAction PolicyAction `yaml:"policy" json:"policy"`
	}

	RuleGrouping string

	Match struct {
		Channel      *string         `yaml:"channel,omitempty" json:"channel"`
		EventId      *events.EventId `yaml:"eventId,omitempty" json:"event_id"`
		ProviderGuid *string         `yaml:"provider,omitempty" json:"provider"`
	}

	PolicyAction struct {
		Type            PolicyType               `yaml:"type" json:"type"`
		RuleGroup       RuleGrouping             `yaml:"applyTo" json:"rule_group"`
		RetentionPeriod types.MarshalledDuration `yaml:"retentionPeriod" json:"retention_period"` // Used for timestamp-based policies
		Volume          uint64                   `yaml:"volume" json:"volume"`                    // Used for count-based policies
	}

	PolicyType string
)

const (
	RuleGroupingGlobal    RuleGrouping = "global"
	RuleGroupingPrincipal RuleGrouping = "principal"

	PolicyTypeTimestamp PolicyType = "timestamp"
	PolicyTypeCount     PolicyType = "count"
)

func NewStoredPolicy(policy RetentionPolicy, author identity.Principal, appliedAt time.Time) StoredPolicy {
	return StoredPolicy{
		Policy:    policy,
		Author:    author,
		AppliedAt: appliedAt,
	}
}

func (p RetentionPolicy) Validate() error {
	if len(p.Filters) == 0 {
		return errors.New("no filters defined")
	}

	var hasCountFilter bool
	for _, filter := range p.Filters {
		if filter.PolicyAction.Type == PolicyTypeCount {
			if hasCountFilter {
				return errors.New("only one count filter can be defined")
			}

			hasCountFilter = true

			if filter.PolicyAction.Volume == 0 {
				return errors.New("count filter volume must be greater than 0, or filter removed to achieve the same effect")
			}

			if filter.PolicyAction.RetentionPeriod.Duration() != 0 {
				return errors.New("count filter cannot have retention period")
			}

			if filter.PolicyAction.RuleGroup != RuleGroupingGlobal && filter.PolicyAction.RuleGroup != RuleGroupingPrincipal {
				return fmt.Errorf(
					"invalid rule grouping: %s, must be '%s' or '%s'",
					filter.PolicyAction.RuleGroup,
					RuleGroupingGlobal,
					RuleGroupingPrincipal,
				)
			}
		} else if filter.PolicyAction.Type == PolicyTypeTimestamp {
			if filter.PolicyAction.RetentionPeriod.Duration() == 0 {
				return errors.New("timestamp filter retention period must be greater than 0, or filter removed to achieve the same effect")
			}

			if filter.PolicyAction.RuleGroup != "" && filter.PolicyAction.RuleGroup != RuleGroupingGlobal {
				return errors.New("timestamp filter cannot have rule grouping (applyTo)")
			}

			if filter.PolicyAction.Volume != 0 {
				return errors.New("timestamp filter cannot have volume")
			}
		} else {
			return fmt.Errorf("invalid policy type: %s", filter.PolicyAction.Type)
		}
	}

	return nil
}

func (p StoredPolicy) Equal(other StoredPolicy) bool {
	return p.Policy.Equal(other.Policy) && p.Author == other.Author && p.AppliedAt.Equal(other.AppliedAt)
}

func (p RetentionPolicy) Equal(other RetentionPolicy) bool {
	if len(p.Filters) != len(other.Filters) {
		return false
	}

	for i, filter := range p.Filters {
		if !filter.Equal(other.Filters[i]) {
			return false
		}
	}

	return true
}

func (f Filter) Equal(other Filter) bool {
	return f.Label == other.Label && f.Match.Equal(other.Match) && f.PolicyAction == other.PolicyAction
}

func (m Match) Equal(other Match) bool {
	return isEqual(m.Channel, other.Channel) && isEqual(m.EventId, other.EventId) && isEqual(m.ProviderGuid, other.ProviderGuid)
}

func isEqual[T comparable](p1, p2 *T) bool {
	if p1 == nil && p2 == nil {
		return true
	}

	if p1 == nil || p2 == nil {
		return false
	}

	return *p1 == *p2
}
