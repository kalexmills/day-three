package internal

import "fmt"

// BitGrid manages a 2D grid of booleans which can move around in 2D space.
type BitGrid struct {
	width int
	bytes []byte
	// position of this BitGrid in its ambient 2D space. Used for virtually 'moving' this BitGrid during collision
	// detection without actually modifying heap memory.
	offset IVec2
}

func NewBitGrid(width, height int) BitGrid {
	return BitGrid{
		bytes: make([]byte, width*height),
		width: width,
	}
}

func (g *BitGrid) Dims() IDim {
	return IDim{W: g.width, H: len(g.bytes) / g.width}
}

func (g *BitGrid) Set(x, y int) {
	idx := g.idx(x-g.offset.X, y-g.offset.Y)
	if idx < 0 || idx > len(g.bytes) { // nothing to set
		return
	}
	g.bytes[idx/8] |= 1 << (idx % 8)
}

func (g *BitGrid) Unset(x, y int) {
	idx := g.idx(x-g.offset.X, y-g.offset.Y)
	if idx < 0 || idx > len(g.bytes) { // nothing to unset
		return
	}
	g.bytes[idx/8] -= 1 << (idx % 8)
}

// Get returns true iff the bit at (x,y) is set.
func (g *BitGrid) Get(x, y int) bool {
	idx := g.idx(x-g.offset.X, y-g.offset.Y)
	if idx < 0 || idx > len(g.bytes) { // nothing to unset
		return false // everything outside this BitGrid is by definition false.
	}
	return g.isSet(idx)
}

// Add returns a shallow copy of this BitGrid, pointing to the same underlying memory but with an updated offset.
func (g *BitGrid) Add(v IVec2) BitGrid {
	result := *g // flyweight
	result.offset.X += v.X
	result.offset.Y += v.Y
	return result
}

func (g *BitGrid) isSet(idx int) bool {
	return g.bytes[idx/8]&(1<<(idx%8)) > 0
}

// ForEach calls f for each point in this bitgrid. The coordinates passed to f are offset by the BitGrid's current
// position in space. If f ever returns true, no further calls will be made.
func (g *BitGrid) ForEach(f func(x, y int, set bool) (halt bool)) {
	dims := g.Dims()
	fmt.Println("calling ForEach with offset:", g.offset)
	for x := 0; x < dims.W; x++ {
		for y := 0; y < dims.H; y++ {
			idx := y*g.width + x
			if f(x+g.offset.X, y+g.offset.Y, g.bytes[idx/8]&(1<<(idx%8)) > 0) {
				return
			}
		}
	}
}

func (g *BitGrid) idx(x, y int) int {
	return y*g.width + x
}
