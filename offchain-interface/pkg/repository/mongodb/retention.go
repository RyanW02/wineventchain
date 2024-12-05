package mongodb

import (
	"fmt"
	types "github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strings"
	"time"
)

const (
	KeyEventId   = "metadata.event_id"
	KeyTimestamp = "metadata.received_time"
	KeyPrincipal = "metadata.principal"

	KeyChannel     = "event.event.system.channel"
	KeyEventTypeId = "event.event.system.event_id"
	KeyProvider    = "event.event.system.provider.guid"
)

var (
	ErrInvalidPolicyType    = errors.New("invalid policy type in context")
	ErrMultipleCountFilters = errors.New("multiple count-based filters not allowed, as they would overlap")
	ErrInvalidRuleGrouping  = errors.New("invalid rule grouping")
)

// Re-usable aggregation stages
var (
	stageExtractIds = bson.M{
		"$group": bson.M{
			"_id": nil,
			"events": bson.M{
				"$push": "$" + KeyEventId,
			},
		},
	}

	stageProjectEventsOnly = bson.M{
		"$project": bson.M{
			"_id":    0,
			"events": 1,
		},
	}
)

func buildAggregate(policy types.RetentionPolicy) (bson.A, error) {
	if err := policy.Validate(); err != nil {
		return nil, errors.Wrap(err, "policy validation failed")
	}

	var aggregate bson.A

	// Apply global filters
	var matcher bson.A
	for _, filter := range policy.Filters {
		if !isGlobal(filter) {
			continue
		}

		// Infallible, since only timestamp policies are supported for global filters
		if filter.PolicyAction.Type != types.PolicyTypeTimestamp {
			return nil, errors.Wrapf(ErrInvalidPolicyType, "policy type: %s", filter.PolicyAction.Type)
		}

		var orClause bson.A
		andClause := bson.A{
			bson.M{KeyTimestamp: bson.M{
				"$lt": primitive.NewDateTimeFromTime(time.Now().Add(-filter.PolicyAction.RetentionPeriod.Duration()))},
			},
		}

		if filter.Match.Channel != nil {
			orClause = append(orClause, bson.M{
				KeyChannel: bson.M{"$ne": *filter.Match.Channel},
			})

			andClause = append(andClause, bson.M{KeyChannel: *filter.Match.Channel})
		}

		if filter.Match.EventId != nil {
			orClause = append(orClause, bson.M{
				KeyEventTypeId: bson.M{"$ne": *filter.Match.EventId},
			})

			andClause = append(andClause, bson.M{KeyEventTypeId: *filter.Match.EventId})
		}

		if filter.Match.ProviderGuid != nil {
			guid := fmt.Sprintf("{%s}", strings.ToLower(*filter.Match.ProviderGuid))

			orClause = append(orClause, bson.M{
				KeyProvider: bson.M{"$ne": guid},
			})

			andClause = append(andClause, bson.M{KeyProvider: guid})
		}

		matcher = append(matcher, bson.M{
			"$or": append(orClause, bson.M{
				"$and": andClause,
			}),
		})
	}

	if len(matcher) > 0 {
		aggregate = append(aggregate, bson.M{
			"$match": bson.M{
				"$and": matcher,
			},
		})
	}

	// Projection, group, etc. only needed for count-based policies
	countFilter := getCountFilter(policy)
	if countFilter == nil {
		aggregate = append(aggregate, stageExtractIds, stageProjectEventsOnly)
		return aggregate, nil
	}

	// Apply sort
	aggregate = append(aggregate, bson.M{
		"$sort": bson.M{
			KeyTimestamp: -1,
		},
	})

	// Global volume limit
	if countFilter.PolicyAction.RuleGroup == types.RuleGroupingGlobal || countFilter.PolicyAction.RuleGroup == "" {
		aggregate = append(aggregate,
			bson.M{
				"$skip": countFilter.PolicyAction.Volume,
			},
			// Return as single array, instead of array of objects with string property
			stageExtractIds,
			stageProjectEventsOnly,
		)
	} else if countFilter.PolicyAction.RuleGroup == types.RuleGroupingPrincipal { // Group by, keep last N events per principal
		aggregate = append(aggregate,
			bson.M{
				"$group": bson.M{
					"_id": "$" + KeyPrincipal,
					"events": bson.M{
						"$push": bson.M{
							"event_id":  "$" + KeyEventId,
							"principal": "$" + KeyPrincipal,
						},
					},
				},
			},
			bson.M{
				"$project": bson.M{
					"principal": "$_id",
					"events": bson.M{
						"$slice": bson.A{
							bson.M{
								"$map": bson.M{
									"input": "$events",
									"as":    "event",
									"in": bson.M{
										"event_id": "$$event.event_id",
									},
								},
							},
							countFilter.PolicyAction.Volume,
							bson.M{
								"$size": "$events",
							},
						},
					},
				},
			},
			bson.M{
				"$unwind": "$events",
			},
			bson.M{
				"$replaceRoot": bson.M{
					"newRoot": "$events",
				},
			},
			// Return as single array, instead of array of objects with string property
			bson.M{
				"$group": bson.M{
					"_id": nil,
					"events": bson.M{
						"$push": "$event_id",
					},
				},
			},
			stageProjectEventsOnly,
		)
	} else {
		return nil, errors.Wrapf(ErrInvalidRuleGrouping, "rule grouping: %s", countFilter.PolicyAction.RuleGroup)
	}

	return aggregate, nil
}

func isMatchEmpty(m types.Match) bool {
	return m.Channel == nil && m.EventId == nil && m.ProviderGuid == nil
}

func isGlobal(f types.Filter) bool {
	return f.PolicyAction.Type == types.PolicyTypeTimestamp &&
		(f.PolicyAction.RuleGroup == types.RuleGroupingGlobal || f.PolicyAction.RuleGroup == "")
}

func getCountFilter(p types.RetentionPolicy) *types.Filter {
	for _, filter := range p.Filters {
		if filter.PolicyAction.Type == types.PolicyTypeCount {
			return &filter
		}
	}

	return nil
}
