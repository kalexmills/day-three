package internal

import (
	"image"
	"math"
)

// IRect is an integer rectangle.
type IRect struct {
	X, Y, W, H int
}

// IDim returns the width and height of this rectangle.
func (r IRect) IDim() IDim { return IDim{W: r.W, H: r.H} }

// IVec2 returns the upper-left coordinate of this rectangle.
func (r IRect) IVec2() IVec2 { return IVec2{X: r.X, Y: r.Y} }

func (r IRect) Rectangle() image.Rectangle {
	return image.Rect(r.X, r.Y, r.X+r.W, r.Y+r.H)
}

func (r IRect) Add(pos IVec2) IRect {
	return IRect{X: r.X + pos.X, Y: r.Y + pos.Y, W: r.W, H: r.H}
}

// Rect is a floating-point rectangle.
type Rect struct {
	X, Y, W, H float64
}

// Dim returns the width and height of this rectangle.
func (r Rect) Dim() Dim { return Dim{W: r.W, H: r.H} }

// Vec2 returns the upper-left coordinate of this rectangle.
func (r Rect) Vec2() Vec2 { return Vec2{X: r.X, Y: r.Y} }

// IDim is integer width and height.
type IDim struct{ W, H int }

// Dim is floating-point width and height.
type Dim struct{ W, H float64 }

// IVec2 is integer 2D-coordinates.
type IVec2 struct{ X, Y int }

func (v IVec2) Vec2() Vec2 { return Vec2{X: float64(v.X), Y: float64(v.Y)} }

func (v IVec2) Scale(a int) IVec2 { return IVec2{X: v.X * a, Y: v.Y * a} }

// Vec2 is floating-point 2D-coordinates.
type Vec2 struct{ X, Y float64 }

// Mag returns the magnitude of this vector in Euclidean space.
func (v Vec2) Mag() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y)
}
