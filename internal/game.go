package internal

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kalexmills/asebiten"
	"sync"
)

// TPS is the number of ticks per second, read once when the game starts.
var TPS float64
var TPSOnce sync.Once

// Game implements ebiten.Game interface.
type Game struct {
	currScene Scene
}

func NewGame() (*Game, error) {
	data, err := LoadGameData()
	if err != nil {
		return nil, fmt.Errorf("error loading game data: %v", err)
	}
	return &Game{
		currScene: NewPlatformerScene(&data),
	}, nil
}

// Update proceeds the game state.
// Update is called every tick (1/60 [s] by default).
func (g *Game) Update() error {
	asebiten.Update() // call once to update timing data.
	TPSOnce.Do(func() {
		TPS = float64(ebiten.TPS())
	})
	return g.currScene.Update()
}

// Draw draws the game screen.
// Draw is called every frame (typically 1/60[s] for 60Hz display).
func (g *Game) Draw(screen *ebiten.Image) {
	// Write your game's rendering.
	g.currScene.Draw(screen)
}

// Layout takes the outside size (e.g., the window size) and returns the (logical) screen size.
// If you don't have to adjust the screen size with the outside size, just return a fixed size.
func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return g.currScene.Layout(outsideWidth, outsideHeight)
}

// ChangeScene sets the current scene to the provided Scene.
func (g *Game) ChangeScene(s Scene) {
	g.currScene = s
}
