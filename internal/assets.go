package internal

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kalexmills/asebiten"
	"github.com/kalexmills/asebiten/aseprite"
	"image"
)

type PlayerAnim uint8

const (
	PlayerAnimIdle PlayerAnim = iota
	PlayerAnimJump
	PlayerAnimLand
	PlayerAnimRun
	PlayerAnimWalk
)

const (
	jumpUpTag   = "Jump up"
	jumpMaxTag  = "Max Height"
	jumpDownTag = "Jump Down"
)

var anims = map[PlayerAnim]string{
	PlayerAnimIdle: "idle.json",
	PlayerAnimJump: "jump.json",
	PlayerAnimLand: "land.json",
	PlayerAnimRun:  "run.json",
	PlayerAnimWalk: "walk.json",
}

func LoadPlayerAnims() (*PlayerSprite, error) {
	var err error
	result := &PlayerSprite{
		anims: make(map[PlayerAnim]*asebiten.Animation, len(anims)),
	}
	for anim, path := range anims {
		result.anims[anim], err = aseprite.LoadAnimation(gameData, "gamedata/sprites/"+path)
		if err != nil {
			return nil, err
		}
	}
	if err := result.loadMasks(); err != nil {
		return nil, err
	}
	return result, nil
}

type PlayerSprite struct {
	curr    *asebiten.Animation
	currKey PlayerAnim
	currTag string

	sheets map[PlayerAnim]aseprite.SpriteSheet
	anims  map[PlayerAnim]*asebiten.Animation

	// collision masks for the sprite.
	masks map[PlayerAnim]map[string][]BitGrid

	facingLeft bool // true if the player is facing left.
}

func (p *PlayerSprite) Update() {
	if p.curr == nil {
		p.curr = p.anims[PlayerAnimIdle]
		p.curr.Resume()
	}
	p.curr.Update()
}

func (p *PlayerSprite) SetAnim(key PlayerAnim, left bool) {
	p.currTag = ""
	if animation, ok := p.anims[key]; ok {
		p.curr = animation
		p.currKey = key
	}
	p.facingLeft = left
	if key != PlayerAnimJump { // jumping uses different animation logic
		p.curr.Restart()
		p.curr.Resume()
	}
}

func (p *PlayerSprite) SetTag(tag string) {
	p.curr.SetTag(tag)
	p.currTag = tag
}

func (p *PlayerSprite) SetFacing(left bool) {
	p.facingLeft = left
}

func (p *PlayerSprite) DrawTo(screen *ebiten.Image, options *ebiten.DrawImageOptions) {
	opts := ebiten.DrawImageOptions{}
	if p.facingLeft {
		opts.GeoM.Scale(-1, 1) // flip horizontal
		opts.GeoM.Translate(float64(p.curr.Bounds().Dx()), 0)
	}
	opts.GeoM.Concat(options.GeoM)
	opts.ColorScale = options.ColorScale
	opts.Blend = options.Blend
	opts.Filter = options.Filter
	p.curr.DrawTo(screen, &opts)
}

func (p *PlayerSprite) Bounds() image.Rectangle {
	return p.curr.Bounds()
}

// Bitmask retrieves the collision mask for the frame of the current animation.
func (p *PlayerSprite) Bitmask() BitGrid {
	fmt.Println(p.currKey, p.currTag, p.curr.FrameIdx())
	return p.masks[p.currKey][p.currTag][p.curr.FrameIdx()]
}

func (p *PlayerSprite) loadMasks() error {
	p.masks = make(map[PlayerAnim]map[string][]BitGrid, len(p.anims))
	for key := range p.anims {
		if err := p.loadMasksForAnim(key); err != nil {
			return err
		}
	}
	return nil
}

// loadMasksForAnim loads up bitmasks for each frame of the animation with the provided key.
func (p *PlayerSprite) loadMasksForAnim(key PlayerAnim) error {
	anim := p.anims[key]
	if anim == nil {
		return fmt.Errorf("unexpected player animation key: %d", key)
	}
	p.masks[key] = make(map[string][]BitGrid, len(anim.FramesByTagName))

	for tag, frames := range anim.FramesByTagName {
		for _, frame := range frames {
			img := frame.Image
			bg := NewBitGrid(img.Bounds().Dx(), img.Bounds().Dy())
			for x := 0; x < img.Bounds().Dx(); x++ {
				for y := 0; y < img.Bounds().Dy(); y++ {
					color := img.At(x, y)
					if _, _, _, a := color.RGBA(); a != 0 {
						bg.Set(x, y)
					}
				}
			}
			p.masks[key][tag] = append(p.masks[key][tag], bg)
		}
	}
	return nil
}
