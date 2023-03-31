package internal

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kalexmills/asebiten"
	"github.com/kalexmills/asebiten/aseprite"
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
	idleAnim = "idle.json"
	jumpAnim = "jump.json"
	landAnim = "land.json"
	runAnim  = "run.json"
	walkAnim = "walk.json"
)

func LoadPlayerAnims() (*PlayerSprite, error) {
	var err error
	result := &PlayerSprite{}
	result.idle, err = aseprite.LoadAnimation(gameData, "gamedata/sprites/"+idleAnim)
	if err != nil {
		return nil, err
	}
	result.jump, err = aseprite.LoadAnimation(gameData, "gamedata/sprites/"+jumpAnim)
	if err != nil {
		return nil, err
	}
	result.land, err = aseprite.LoadAnimation(gameData, "gamedata/sprites/"+landAnim)
	if err != nil {
		return nil, err
	}
	result.run, err = aseprite.LoadAnimation(gameData, "gamedata/sprites/"+runAnim)
	if err != nil {
		return nil, err
	}
	result.walk, err = aseprite.LoadAnimation(gameData, "gamedata/sprites/"+walkAnim)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type PlayerSprite struct {
	curr *asebiten.Animation

	idle *asebiten.Animation
	jump *asebiten.Animation
	land *asebiten.Animation
	run  *asebiten.Animation
	walk *asebiten.Animation

	facingLeft bool // true if the player is facing left.
}

func (p *PlayerSprite) Update() {
	if p.curr == nil {
		p.curr = p.idle
	}
	p.curr.Update()
}

func (p *PlayerSprite) SetAnim(anim PlayerAnim) {
	switch anim {
	case PlayerAnimIdle:
		p.curr = p.idle
	case PlayerAnimJump:
		p.curr = p.jump
	case PlayerAnimLand:
		p.curr = p.land
	case PlayerAnimWalk:
		p.curr = p.walk
	case PlayerAnimRun:
		p.curr = p.run
	}
	p.curr.Restart()
}

func (p *PlayerSprite) DrawTo(screen *ebiten.Image, options *ebiten.DrawImageOptions) {
	opts := &ebiten.DrawImageOptions{}
	if p.facingLeft {
		opts.GeoM.Scale(-1, 1) // flip horizontal
		//opts.GeoM.Translate(p.curr.)
	}
}
