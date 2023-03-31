package internal

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"log"
	"math"
	"strings"
)

// LayerID identifies a specific layer from LDtk Level data by ID.
type LayerID = string

const (
	// CollisionLayerID is the ID for an IntGrid from LDtk.
	CollisionLayerID = "Collisions"
)

// PlatformerScene is set up to use the data output by LDtk.
type PlatformerScene struct {
	*BaseScene
	gdat *GameData

	camera IRect        // camera is the region of the screen being rendered.
	keys   []ebiten.Key // keys is the set of keys currently pressed.

	loaded      bool
	background  *ebiten.Image
	player      *Player
	cellSize    int // width and height of each cell in the collision mask
	intGridData []IntGridData
	cellsWide   int
	debug       bool
	underCursor IntGridData
}

func NewPlatformerScene(gdat *GameData) *PlatformerScene {
	result := &PlatformerScene{
		BaseScene: &BaseScene{},
		gdat:      gdat,
		debug:     true,
	}
	w, h := result.Layout(0, 0) // use base scene's layout options for the screen.
	result.camera = IRect{X: 0, Y: 0, W: w, H: h}
	result.background = ebiten.NewImage(w, h)
	return result
}

// Update calls the update loop every frame.
func (s *PlatformerScene) Update() error {
	if !s.loaded { // TODO: consider doing this async
		timeit("loading level", func() {
			if err := s.LoadLevel(s.gdat.LevelStart); err != nil {
				log.Fatal(err)
			}
		})
	}
	// update under cursor for debug draw
	x, y := ebiten.CursorPosition()
	x -= s.camera.X
	y -= s.camera.Y
	s.underCursor = s.gridData(float64(x), float64(y))

	if s.player != nil {
		s.player.Update()
	}
	s.updateCamera()

	return nil
}

// updateCamera updates the camera.
func (s *PlatformerScene) updateCamera() {
	s.camera.X = s.camera.W/2 - s.player.Pos.X
	s.camera.Y = s.camera.H/2 - s.player.Pos.Y
}

// Draw draws this scene to the provided Image.
func (s *PlatformerScene) Draw(screen *ebiten.Image) {
	// draw background
	opts := ebiten.DrawImageOptions{}
	opts.GeoM.Translate(float64(s.camera.X), float64(s.camera.Y))
	screen.DrawImage(s.background, &opts)

	// draw player sprite
	opts.GeoM.Translate(float64(s.player.Pos.X), float64(s.player.Pos.Y))
	s.player.sprite.DrawTo(screen, &opts)
	//screen.DrawImage(s.player.sprite, &opts)

	// draw player state
	if s.debug {
		s.drawDebug(screen)
	}
}

// drawDebug draws a bunch of platformer-related debug messages to the screen.
func (s *PlatformerScene) drawDebug(screen *ebiten.Image) {
	var lines []string

	// print FPS
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%.0f", ebiten.ActualFPS()), 300, 0)

	// print player state and position
	lines = append(lines, fmt.Sprintf("Player state: %s", s.player.state))
	lines = append(lines, fmt.Sprintf("Pos: (%d, %d); Vel: (%.2f, %.2f)",
		s.player.Pos.X, s.player.Pos.Y, s.player.Vel.X, s.player.Vel.Y))

	ebitenutil.DebugPrint(screen, strings.Join(lines, "\n"))

	// print IntGridData under cursor
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("0x%x", s.underCursor), 0, 220)

	// print Player colliding data
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("0x%x", s.player.colliding), 0, 210)
}

// LoadLevel loads a level by its UID, unloading the currently loaded level and the background. No foreground or
// parallaxing layers are loaded.
func (s *PlatformerScene) LoadLevel(id UID) error {
	log.Printf("loading level ID %d", id)
	s.loaded = true

	level, ok := s.gdat.Levels[id]
	if !ok {
		return fmt.Errorf("no level found with id: %d", id)
	}

	if err := s.loadBackground(level); err != nil {
		return err
	}
	if err := s.loadCells(level); err != nil {
		return err
	}
	if err := s.loadEntities(level); err != nil {
		return err
	}
	s.processLadders()
	s.processOneWay()
	return nil
}

// loadBackground loads the background for the level, returning any fatal errors.
func (s *PlatformerScene) loadBackground(level *Level) error {
	// TODO: probably store the old level's background somewhere in case we end up splattering on it.
	//       the player should be able to see the exact same splatters whenever they come back.

	// paint a (fresh) background.
	s.background = ebiten.NewImage(level.PxDims.W, level.PxDims.H)

	log.Printf("loading level '%s'", level.ID)
	opts := ebiten.DrawImageOptions{} // shared for fewer allocations
	for _, layer := range level.layers {
		if layer.TileSetUID == nil {
			continue
		}
		tileset, ok := s.gdat.Tilesets[*layer.TileSetUID]
		if !ok {
			return fmt.Errorf("no tileset found for UID: %d", layer.TileSetUID)
		}
		for _, tile := range layer.Tiles {
			s.drawTile(tileset, layer, tile, &opts)
		}
	}
	return nil
}

// loadEntities loads all entities associated with the provided Level, returning any fatal errors.
func (s *PlatformerScene) loadEntities(level *Level) error {
	var err error
	for _, entity := range level.Entities {
		switch entity.ID {
		case EtyPlayer:
			if s.player == nil {
				s.player, err = NewPlayer(s)
				if err != nil {
					return err
				}
			}
			s.player.SetPos(entity.PxCoords)
			s.player.startIdling()
		}
	}
	return nil
}

// loadCells loads all cell data associated with the provided Level, returning any fatal errors.
func (s *PlatformerScene) loadCells(level *Level) error {
	collisionGrid, ok := level.layersByID[CollisionLayerID]
	if !ok || len(collisionGrid.Grid) == 0 {
		return fmt.Errorf("could not find layer with ID '%s'", CollisionLayerID)
	}
	s.cellSize = collisionGrid.GridSize
	s.cellsWide = collisionGrid.CellDims.W
	s.intGridData = make([]IntGridData, len(collisionGrid.Grid))
	for i, d := range collisionGrid.Grid {
		s.intGridData[i] = IntGridData(d)
	}
	return nil
}

// processLadders detects ladder tops and bottoms and sets flags appropriately.
func (s *PlatformerScene) processLadders() {
	s.forAllGridData(func(cx int, cy int, dat IntGridData) {
		if dat != IntGridLadder {
			return
		}
		// mark ladder tops
		if !s.gridDataI(cx+1, cy-1).isSolid() && !s.gridDataI(cx-1, cy-1).isSolid() &&
			(s.gridDataI(cx-1, cy).isSolid() || s.gridDataI(cx+1, cy).isSolid()) {
			dat |= IntGridLadderTop
			s.setGridDataI(cx, cy, dat)
		}
		// mark ladder bottoms
		if s.gridDataI(cx, cy+1).isSolid() {
			dat |= IntGridLadderBottom
			s.setGridDataI(cx, cy, dat)
		}
	})
	return
}

// processOneWay processes 'one way' platforms, setting the one-way flag as needed.
func (s *PlatformerScene) processOneWay() {
	s.forAllGridData(func(cx int, cy int, dat IntGridData) {
		if dat != IntGridDirt {
			return
		}
		// dirt that's not surrounded by solids is a one-way platform
		if !s.gridDataI(cx, cy-1).isSolid() && !s.gridDataI(cx, cy+1).isSolid() {
			dat |= IntGridOneWay
			s.setGridDataI(cx, cy, dat)
		}

	})
	return
}

// drawTile draws the provided tile from the provided tileset to the background image. The opts provided is mutated by
// this call and is passed for efficiency.
func (s *PlatformerScene) drawTile(tileset *ebiten.Image, layer *TileLayer, tile Tile, opts *ebiten.DrawImageOptions) {
	opts.GeoM.Reset()
	opts.GeoM = tile.GeoM(layer.GridSize)
	opts.ColorScale.SetA(layer.Opacity)
	s.background.DrawImage(
		tileset.SubImage(tile.Rectangle(layer.GridSize)).(*ebiten.Image), // safe; guaranteed per docs.
		opts,
	)
}

// An Actor represents anything that can move around and collide with objects in the PlatformerScene. Actor handles
// all low-level movement and collision testing within a PlatformerScene.
type Actor struct {
	scene *PlatformerScene
}

// TODO: refactor to remove hitbox and bitgrid from this func?

// MoveX moves this actor's hitbox by the given amount in the X-direction, returning a CollideMask that explains which
// solid collisions occurred, if any. Y-velocity is included in order to test collisions for one-way platforms.
func (a *Actor) MoveX(hitbox IRect, amt float64, clip ClipFunc) (actual int, result CollideMask) {
	return a.scene.MoveX(hitbox, amt, clip)
}

// MoveY moves this actor's hitbox by the given amount in the Y-direction, returning a CollideMask that explains which
// solid collisions occurred, if any.
func (a *Actor) MoveY(hitbox IRect, amt float64, clip ClipFunc) (actual int, result CollideMask) {
	return a.scene.MoveY(hitbox, amt, clip)
}

// CellAt provides the coordinates and contents of the cell containing the provided point.
func (a *Actor) CellAt(point Vec2) (Vec2, CollideMask) {
	return a.scene.at(point)
}

// Collides performs a collision test for this actor, returning the collidemask found.
func (a *Actor) Collides(hitbox IRect) CollideMask {
	return a.scene.Collides(hitbox, func(mask CollideMask) bool {
		return false
	})
}

type IntGridData uint32

const (
	IntGridNothing IntGridData = iota
	IntGridDirt
	IntGridLadder
	IntGridStone
	IntGridLadderTop    = IntGridLadder | (1 << 31)
	IntGridLadderBottom = IntGridLadder | (1 << 30)
	IntGridOneWay       = 1 << 31 // OneWay solids are cells you cannot hit your head on.
)

func (d IntGridData) isLadder() bool {
	return d&(0x3fffffff) == IntGridLadder // unset 2 rightmost bits then compare
}

func (d IntGridData) isSolid() bool {
	return d == IntGridStone || d == IntGridDirt
}

func (d IntGridData) isOneWay() bool {
	return d&IntGridOneWay == IntGridOneWay
}

func (d IntGridData) CollideMask() CollideMask {
	newd := d & (0x3fffffff) // unset flags.
	if newd == 0 {
		return CollideNone
	}
	return CollideMask(1<<(newd-1)) | CollideMask(d&(0xc0000000)) // reset flags
}

// CollideMask is a bitmask according to the following diagram.
type CollideMask uint32

const (
	CollideNone = 0
	CollideDirt = 1 << (iota - 1)
	CollideLadder
	CollideStone
	CollidedSolid                = CollideDirt | CollideStone // solids are solid underfoot
	CollideLadderTop CollideMask = CollideLadder | (1 << 31)
	CollideLadderBot CollideMask = CollideLadder | (1 << 30)
	CollidedOneWay   CollideMask = 1 << 31
)

type ClipFunc func(CollideMask) bool

// Colliding returns false if the provided ClipFunc clips through the provided mask, otherwise
// returns whether the provided collide mask is considered a solid object.
func (m CollideMask) Colliding(clip ClipFunc) bool {
	if clip(m) {
		return false
	}
	return m&CollidedSolid > 0 || (m&CollidedOneWay) == CollidedOneWay
}

// MoveX attempts to move a sprite with the provided hitbox by the provided amount in the X-direction, which may be
// positive or negative. Returns the actual amount moved without colliding with a solid object and any items currently
// collided with. MoveX only moves the provided box by integer amounts. Callers are responsible for managing the state
// of their own floating point "remainder" and including it in the amount passed on each frame.
func (s *PlatformerScene) MoveX(hitbox IRect, amt float64, clip ClipFunc) (actual int, result CollideMask) {
	return s.move(hitbox, amt, IVec2{X: 1, Y: 0}, clip)
}

// MoveY is like MoveX, except it moves in the Y-direction. See MoveX for documentation.
func (s *PlatformerScene) MoveY(hitbox IRect, amt float64, clip ClipFunc) (actual int, result CollideMask) {
	return s.move(hitbox, amt, IVec2{X: 0, Y: 1}, clip)
}

// move moves the provided hitbox by the requested amount along the provided axis. The provided velocity is used to
// ensure that one-way platforms are handled appropriately.
func (s *PlatformerScene) move(hitbox IRect, amount float64, axis IVec2, clip ClipFunc) (actual int, result CollideMask) {
	move := int(math.Round(amount))
	if move == 0 {
		return 0, s.AllOverlapping(hitbox) // TODO: must be pixel-perfect
	}
	actualMoved := 0
	sign := int(math.Copysign(1, amount))
	for move != 0 {
		displacement := axis.Scale(sign)
		collideMask := s.Collides(hitbox.Add(displacement), clip)
		if !collideMask.Colliding(clip) {
			hitbox = hitbox.Add(displacement)
			move -= sign
			actualMoved += sign
		} else {
			return actualMoved, collideMask
		}
	}
	return actualMoved, 0 // no collision
}

// cellOver returns the contents and coordinates of the unique cell closest to the bottom of the provided hitbox.
func (s *PlatformerScene) at(pt Vec2) (Vec2, CollideMask) {
	cx, cy := s.screenToCell(pt.X, pt.Y)
	return Vec2{X: float64(cx * s.cellSize), Y: float64(cy * s.cellSize)}, s.gridData(pt.X, pt.Y).CollideMask()
}

func (s *PlatformerScene) Collides(hitbox IRect, clip ClipFunc) CollideMask {
	return s.BoxCollides(hitbox, clip)
}

// Collides performs collision detection for the provided hitbox, travelling at the provided velocity. Velocity is used
// to handle one-way platforms.
func (s *PlatformerScene) BoxCollides(hitbox IRect, clip ClipFunc) (result CollideMask) {
	const eps = 1e-3
	x1, y1, x2, y2 := float64(hitbox.X)+eps, float64(hitbox.Y)+eps, float64(hitbox.X+hitbox.W)-eps, float64(hitbox.Y+hitbox.H)-eps

	collides := func(x, y float64) bool { // tests collisions, ignoring one-way platforms
		dat := s.gridData(x, y)
		if clip(dat.CollideMask()) || dat.isOneWay() { // no one-way platform collisions are possible except
			return false
		}
		result = result | dat.CollideMask()
		return false
	}
	collidesBot := func(x, y float64) bool { // tests collisions, one-way platforms are only solid when not travelling upwards.
		dat := s.gridData(x, y)
		if clip(dat.CollideMask()) {
			return false
		}
		result = result | dat.CollideMask()
		return false
	}

	collidesBot(x2, y2) // bottom-right corner
	collidesBot(x1, y2) // bottom-left corner
	collides(x2, y1)    // top-right corner
	collides(x1, y1)    // top-left corner

	forAllVLine(x2, y1, y2, collides)    // right edge
	forAllVLine(x1, y1, y2, collides)    // left edge
	forAllHLine(x1, x2, y1, collides)    // top edge
	forAllHLine(x1, x2, y2, collidesBot) // bottom edge

	for x := x1 + 0.5; x < x2; x += 0.5 {
		for y := y1 + 0.5; y < y2; y += 0.5 {
			collides(x, y)
		}
	}
	return result
}

// AllOverlapping retrieves all cells which the provided hitbox overlaps.
func (s *PlatformerScene) AllOverlapping(hitbox IRect) (result CollideMask) {
	// TODO: use a sprite mask also instead of a hitbox!
	const eps = 1e-3
	x1, y1, x2, y2 := float64(hitbox.X)+eps, float64(hitbox.Y)+eps, float64(hitbox.X+hitbox.W)-eps, float64(hitbox.Y+hitbox.H)-eps
	forAllGrid(x1, y1, x2, y2, func(x, y float64) (halt bool) {
		result = result | s.gridData(x, y).CollideMask()
		return false
	})
	return result
}

// gridData retrieves grid data using screen coordinates (x,y)
func (s *PlatformerScene) gridData(x, y float64) IntGridData {
	cx, cy := s.screenToCell(x, y) // convert to cell space.
	return s.gridDataI(cx, cy)
}

// gridDataI retrieves grid data using cell coordinates (cx, cy).
func (s *PlatformerScene) gridDataI(cx, cy int) IntGridData {
	idx := cx + cy*s.cellsWide
	if idx < 0 || idx >= len(s.intGridData) {
		return 0
	}
	return s.intGridData[idx]
}

// setGridDataI sets grid data. If the cell provided is outside of the range of the currently loaded level, this
// func is a no-op.
func (s *PlatformerScene) setGridDataI(cx, cy int, dat IntGridData) {
	idx := cx + cy*s.cellsWide
	if idx < 0 || idx >= len(s.intGridData) {
		return
	}
	s.intGridData[idx] = dat
}

// screenToCell rounds the provided screen coordinates (x, y) to cell coordinates (cx, cy)
func (s *PlatformerScene) screenToCell(x, y float64) (int, int) {
	return int(x / float64(s.cellSize)), int(y / float64(s.cellSize))
}

// forAllGridData loops over the grid data, calling f at each cell.
func (s *PlatformerScene) forAllGridData(f func(cx int, cy int, dat IntGridData)) {
	w := s.cellsWide
	total := len(s.intGridData)
	for x := 0; x < w; x++ {
		for y := 0; y < total/w; y++ {
			f(x, y, s.intGridData[x+y*w])
		}
	}
}

// forAllHLine visits all points in an half-integer grid which overlap the provided horizontal line, excluding the endpoints.
// If f ever returns true, this func returns immediately and stops testing.
func forAllHLine(x1, x2, y float64, f func(x, y float64) (halt bool)) {
	if f(x1, y) {
		return
	}
	for x := x1 + 0.5; x < x2; x += 0.5 {
		if f(x, y) {
			return
		}
	}
	if f(x2, y) {
		return
	}
}

// forAllHLine visits all points in a half-integer grid which overlap the provided vertical line. If f ever returns true,
// this func returns immediately and stops testing.
func forAllVLine(x, y1, y2 float64, f func(x, y float64) (halt bool)) {
	if y2 > y1 {
		y1, y2 = y2, y1
	}
	if f(x, y1) {
		return
	}
	for y := y1 + 0.5; y < y2; y += 0.5 {
		if f(x, y) {
			return
		}
	}
	if f(x, y2) {
		return
	}
}

// forAllGrid visits all points in a half-integer grid which overlap a rectangle with (x1,y1) as upper-left corner and
// (x2,y2) as the lower-right corner. If f ever returns true, this func returns immediately without testing any further
// points.
func forAllGrid(x1, y1, x2, y2 float64, f func(x, y float64) (halt bool)) {
	// check corners
	if f(x2, y2) {
		return
	}
	if f(x1, y2) {
		return
	}
	if f(x2, y1) {
		return
	}
	if f(x1, y1) {
		return
	}

	forAllVLine(x2, y1, y2, f) // right edge
	forAllVLine(x1, y1, y2, f) // left edge
	forAllHLine(x1, x2, y1, f) // top edge
	forAllHLine(x1, x2, y2, f) // bottom edge

	for x := x1 + 0.5; x < x2; x += 0.5 {
		for y := y1 + 0.5; y < y2; y += 0.5 {
			if f(x, y) {
				return
			}
		}
	}
}
