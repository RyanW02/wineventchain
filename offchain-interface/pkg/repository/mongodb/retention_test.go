package mongodb

import (
	types2 "github.com/RyanW02/wineventchain/common/pkg/types"
	types "github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
	"time"
)

func TestBuildPolicyGlobalVolume(t *testing.T) {
	policy := types.RetentionPolicy{
		Filters: []types.Filter{
			{
				PolicyAction: types.PolicyAction{
					Type:      types.PolicyTypeCount,
					RuleGroup: types.RuleGroupingGlobal,
					Volume:    1000,
				},
			},
		},
	}

	aggregate, err := buildAggregate(policy)
	require.NoError(t, err)

	require.Equal(t, bson.A{
		bson.M{
			"$sort": bson.M{
				"metadata.received_time": -1,
			},
		},
		bson.M{
			"$skip": uint64(1000),
		},
		bson.M{
			"$group": bson.M{
				"_id": nil,
				"events": bson.M{
					"$push": "$metadata.event_id",
				},
			},
		},
		bson.M{
			"$project": bson.M{
				"_id":    0,
				"events": 1,
			},
		},
	}, aggregate)
}

func TestBuildPolicyPrincipalVolume(t *testing.T) {
	policy := types.RetentionPolicy{
		Filters: []types.Filter{
			{
				PolicyAction: types.PolicyAction{
					Type:      types.PolicyTypeCount,
					RuleGroup: types.RuleGroupingPrincipal,
					Volume:    1000,
				},
			},
		},
	}

	aggregate, err := buildAggregate(policy)
	require.NoError(t, err)

	require.Equal(t, bson.A{
		bson.M{
			"$sort": bson.M{
				"metadata.received_time": -1,
			},
		},
		bson.M{
			"$group": bson.M{
				"_id": "$metadata.principal",
				"events": bson.M{
					"$push": bson.M{
						"event_id":  "$metadata.event_id",
						"principal": "$metadata.principal",
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
						uint64(1000),
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
		bson.M{
			"$group": bson.M{
				"_id": nil,
				"events": bson.M{
					"$push": "$event_id",
				},
			},
		},
		bson.M{
			"$project": bson.M{
				"_id":    0,
				"events": 1,
			},
		},
	}, aggregate)
}

func TestBuildPolicyGlobalTimestamp(t *testing.T) {
	policy := types.RetentionPolicy{
		Filters: []types.Filter{
			{
				PolicyAction: types.PolicyAction{
					Type:            types.PolicyTypeTimestamp,
					RuleGroup:       types.RuleGroupingGlobal,
					RetentionPeriod: types2.MarshalledDuration(time.Hour * 24 * 3),
				},
			},
		},
	}

	aggregate, err := buildAggregate(policy)
	require.NoError(t, err)

	// Custom checks due to timestamps
	expected := bson.A{
		bson.M{
			"$match": bson.M{
				"$or": bson.A{
					bson.M{
						"$and": bson.A{
							bson.M{
								"metadata.received_time": bson.M{
									"$lt": time.Now().Add(-time.Hour * 24 * 3),
								},
							},
						},
					},
				},
			},
		},
		bson.M{
			"$group": bson.M{
				"_id": nil,
				"events": bson.M{
					"$push": "$metadata.event_id",
				},
			},
		},
		bson.M{
			"$project": bson.M{
				"_id":    0,
				"events": 1,
			},
		},
	}

	require.Len(t, aggregate, len(expected))
	require.Equal(t, expected[1], aggregate[1])
	require.Equal(t, expected[2], aggregate[2])

	match := aggregate[0].(bson.M)
	require.Len(t, match, 1)

	inner := match["$match"].(bson.M)
	require.Len(t, inner, 1)

	inner2 := inner["$and"].(bson.A)[0].(bson.M)["$or"].(bson.A)
	require.Len(t, inner2, 1)

	inner3 := inner2[0].(bson.M)["$and"].(bson.A)
	require.Len(t, inner3, 1)

	inner4 := inner3[0].(bson.M)["metadata.received_time"].(bson.M)
	require.Len(t, inner3, 1)

	lowerBound := time.Now().Add(-time.Hour * 24 * 3)
	upperBound := time.Now().Add(-time.Hour * 24 * 3).Add(-time.Second * 2)

	lt := inner4["$lt"].(primitive.DateTime)
	require.True(t, lt.Time().Before(lowerBound))
	require.True(t, lt.Time().After(upperBound))
}
