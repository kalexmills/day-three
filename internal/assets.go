package internal

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kalexmills/asebiten"
	"image"
)

type PlayerAnim uint8

const (
	PlayerAnimIdle PlayerAnim = iota
	PlayerAnimJump
	PlayerAnimRun
	PlayerAnimWalk
)

const (
	jumpUpTag   = "up"
	jumpMaxTag  = "max"
	jumpDownTag = "down"
)

var anims = map[PlayerAnim]string{
	PlayerAnimIdle: "idle.json",
	PlayerAnimJump: "jump.json",
	PlayerAnimRun:  "run.json",
	PlayerAnimWalk: "run.json",
}

func LoadPlayerAnims() (*PlayerSprite, error) {
	var err error
	result := &PlayerSprite{
		anims: make(map[PlayerAnim]*asebiten.Animation, len(anims)),
	}
	for anim, path := range anims {
		result.anims[anim], err = asebiten.LoadAnimation(gameData, "gamedata/sprites/"+path)
		if err != nil {
			return nil, err
		}
	}
	if err := result.loadHitboxes(); err != nil {
		return nil, err
	}
	return result, nil
}

type PlayerSprite struct {
	curr    *asebiten.Animation
	currKey PlayerAnim
	currTag string

	sheets map[PlayerAnim]asebiten.SpriteSheet
	anims  map[PlayerAnim]*asebiten.Animation

	// collision hitboxes for the sprite.
	hitboxes map[PlayerAnim]image.Rectangle

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

const hitboxSliceID = "Hitbox"

func (p *PlayerSprite) Hitbox() image.Rectangle {

	rect := p.hitboxes[p.currKey]
	if p.facingLeft {
		dx := rect.Dx()
		rect.Min.X = 2*dx - rect.Min.X // flip horizontally
		rect.Max.X = 2*dx - rect.Max.X
	}
	return rect
}

func (p *PlayerSprite) loadHitboxes() error {
	p.hitboxes = make(map[PlayerAnim]image.Rectangle)
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

	for _, slice := range anim.Source.Meta.Slices {
		if slice.Name == hitboxSliceID {
			p.hitboxes[key] = slice.Keys[0].Bounds.ImageRect()
			return nil
		}
	}
	return fmt.Errorf("no 'Hitbox' slice was found for anim %v", key)
}
