package internal

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIntGridData_CollideMask(t *testing.T) {
	tests := []struct { // test CollideMask
		in       IntGridData
		expected CollideMask
	}{

		{IntGridLadder, CollideLadder},
		{IntGridLadderTop, CollideLadderTop},
		{IntGridLadderBottom, CollideLadderBot},
		{IntGridDirt, CollideDirt},
		{IntGridStone, CollideStone},
		{IntGridNothing, CollideNone},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("0x%x", tt.in), func(t *testing.T) {
			assert.EqualValues(t, tt.expected, tt.in.CollideMask())
		})
	}
	assert.EqualValues(t, CollidedOneWay, IntGridLadderTop.CollideMask()&CollidedOneWay)
	assert.EqualValues(t, CollidedOneWay|CollideStone, (IntGridStone | IntGridOneWay).CollideMask())

	assert.False(t, CollideLadderTop.Colliding(CollidedOneWay))
}
