package proof

import (
	"github.com/cosmos/iavl"
)

func GetProofHeight(tree *iavl.MutableTree) int64 {
	latest := tree.Version()
	if tree.VersionExists(latest - 1) {
		return latest - 1
	} else {
		return latest
	}
}
