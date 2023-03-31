package internal_test

import (
	"github.com/niftysoft/2d-platformer/internal"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBitGrid(t *testing.T) {
	grid := internal.NewBitGrid(10, 10)

	testPoint := func(x, y int) {
		assert.False(t, grid.Get(x, y))
		grid.Set(x, y)
		assert.True(t, grid.Get(x, y))
		grid.Unset(x, y)
		assert.False(t, grid.Get(x, y))
	}

	for x := 0; x < 10; x++ {
		for y := 0; y < 10; y++ {
			testPoint(x, y)
		}
	}

	// test points out of bounds; should be false
	for x := 20; x < 25; x++ {
		for y := 20; y < 25; y++ {
			assert.False(t, grid.Get(x, y))
		}
	}

	offset := internal.IVec2{X: 10, Y: 10} // works with offset.
	grid = grid.Add(offset)
	for x := 10; x < 20; x++ {
		for y := 10; y < 20; y++ {
			testPoint(x, y)
		}
	}
}
