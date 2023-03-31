package internal

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"log"
	"math"
)

// Below are some constant mechanic knobs for tuning the overall 'feel' of the game.

const Friction = float64(0.5)    // Friction multiplies X velocity while the player is becoming idle.
const Gravity = float64(40)      // Gravity in cells per second^2
const PlayerJumpForce = 8        // PlayerJumpForce is the upward force applied by the player's initial jump.
const PlayerLadderJumpForce = 4  // PlayerJumpForce is the upward force applied by the player's initial jump when the player is on a ladder.
const PlayerLeapCoeff = 1.25     // PlayerLeapCoeff is a multiplier to max X velocity when jumping or leaping.
const PlayerTerminalVelocity = 7 // PlayerTerminalVelocity is the players max Y velocity when falling.
const PlayerMaxWalkSpeed = 2     // PlayerMaxWalkSpeed is how quickly the player moves when walking.
const PlayerWalkAccel = 1        // PlayerWalkAccel is the acceleration the player uses in the X-direction when walking.
const PlayerFallAccel = 0.5      // PlayerFallAccel is the acceleration the player uses in the Y-direction when falling.
const PlayerMaxRunSpeed = 5      // PlayerMaxRunSpeed is how quickly the player moves when running.
const PlayerMaxLadderSpeed = 2   // PlayerMaxLadderSpeed is how quickly the player moves up and down ladders.
const PlayerClimbAccel = 0.5     // PlayerClimbAccel is the acceleration the player uses when climbing.
const PlayerOneWayLiftForce = 3  // PlayerOneWayLiftForce is the force on the player when they are being lifted through one-way platforms.

// PlayerInput is a bit vector identifying which buttons are currently being pressed.
type PlayerInput uint32

const (
	InputNone        PlayerInput = 0               // InputNone is equal to the input vector when nothing is being pressed.
	InputWalkedRight PlayerInput = 1 << (iota - 1) // InputWalkedRight is set when the walk right button is held.
	InputWalkedLeft                                // InputWalkedLeft is set when the walk left button is held.
	InputClimbedUp                                 // InputClimbedUp is set when the climb (up) button is held.
	InputClimbedDown                               // InputClimbedDown is set when the climb (up) button is held.
	InputRunning                                   // InputRunning is set when the run button is held.
	InputJumped                                    // InputJumped is set when the jump button is held.

	InputWalked  PlayerInput = InputWalkedRight | InputWalkedLeft // InputWalked is an input mask which doesn't distinguish between the direction walked.
	InputClimbed PlayerInput = InputClimbedUp | InputClimbedDown  // InputClimbed is an input mask which doesn't distinguish between climbing up or down.
)

// TODO: this refactor
//// Running returns true if the run button is held
//func (i PlayerInput) Running() bool {
//	return i&InputRunning > 0
//}
//
//func (i PlayerInput) ClimbingUp() bool {
//	return i&InputClimbedUp > 0
//}
//
//func (i PlayerInput) ClimbingDown()

// PlayerState is a state machine denoting player states at any given point of time.
type PlayerState byte

const (
	PlayerStateIdle = iota
	PlayerStateWalking
	PlayerStateJumping
	PlayerStateFalling
	PlayerStateRunning
	PlayerStateLeaping
	PlayerStateLadderClimbing
	PlayerStateOneWayClimbing // PlayerStateOneWayClimbing means the player is climbing up through a one-way platform.
)

func (s PlayerState) String() string {
	switch s {
	case PlayerStateIdle:
		return "IDLE"
	case PlayerStateWalking:
		return "WALK"
	case PlayerStateJumping:
		return "JUMP"
	case PlayerStateFalling:
		return "FALL"
	case PlayerStateRunning:
		return "RUN"
	case PlayerStateLeaping: // a 'leap' is a jump with a running start.
		return "LEAP"
	case PlayerStateLadderClimbing:
		return "LADDER"
	case PlayerStateOneWayClimbing:
		return "ONEWAY_CLIMB"
	default:
		return "?!?!"
	}
}

type Player struct {
	*Actor
	state PlayerState // state is the player's current state.
	Pos   IVec2       // pos is position in world coordinates.
	Vel   Vec2        // vel is velocity in world coordinates.

	keys []ebiten.Key

	fallResetY    int         // y position past which fallClipmask is reset.
	fallClipmask  CollideMask // fallClipmask is the clipmask set for this fall state. Reset after Y position has dropped
	colliding     CollideMask
	maxFallXSpeed float64 // maxFallXSpeed is the maximum fall speed allowed given how the player started to fall.

	sprite *PlayerSprite
}

func NewPlayer(scene *PlatformerScene) (*Player, error) {
	sprite, err := LoadPlayerAnims()
	if err != nil {
		return nil, err
	}
	result := &Player{
		Actor:  &Actor{scene: scene},
		sprite: sprite,
	}
	result.sprite.Update()
	return result, nil
}

// Update updates the player this frame.
func (p *Player) Update() {
	p.sprite.Update()
	input := p.handleInput()
	nextState := p.state

	switch p.state {
	case PlayerStateIdle:
		nextState = p.updateIdle(input)
	case PlayerStateWalking:
		nextState = p.updateWalking(input)
	case PlayerStateFalling:
		nextState = p.updateFalling(input)
	case PlayerStateJumping:
		nextState = p.updateJumping(input)
	case PlayerStateRunning:
		nextState = p.updateRunning(input)
	case PlayerStateLeaping:
		nextState = p.updateLeaping(input)
	case PlayerStateLadderClimbing:
		nextState = p.updateLadderClimbing(input)
	case PlayerStateOneWayClimbing:
		nextState = p.updateOneWayClimbing(input)
	default:
		panic("default!")
	}

	if nextState != p.state {
		log.Printf("debug: player state changed from %s to %s", p.state, nextState)
	}
	p.state = nextState
}

// SetPos sets the players position without performing any collision testing. It should only be used on loading.
func (p *Player) SetPos(pos IVec2) {
	p.Pos = pos
}

// MoveX moves this player by X, updating its hitbox, velocity, and position as needed.
func (p *Player) MoveX(hitbox IRect) CollideMask {
	dx, collidesWith := p.Actor.MoveX(hitbox, p.Vel.X, p.clipsX)
	p.Pos.X += dx
	if collidesWith.Colliding(p.clipsX) {
		p.Vel.X = 0
	}
	return collidesWith
}

// MoveY moves this player by Y, updating its hitbox, velocity, and position as needed.
func (p *Player) MoveY() CollideMask {
	dy, collidesWith := p.Actor.MoveY(p.Hitbox(), p.Vel.Y, p.clipsY)
	p.Pos.Y += dy
	if collidesWith.Colliding(p.clipsY) {
		p.Vel.Y = 0
	}
	return collidesWith
}

// cellUnderFoot provides the collideMask for the point directly under the player.
func (p *Player) cellUnderFoot() (Vec2, CollideMask) {
	hb := p.Hitbox()
	x, y := float64(hb.X)+float64(hb.W)/2, float64(hb.Y+hb.H)
	return p.CellAt(Vec2{x, y})
}

func (p *Player) startIdling() PlayerState {
	p.sprite.SetAnim(PlayerAnimIdle, p.Vel.X < 0)
	return PlayerStateIdle
}

func (p *Player) updateIdle(input PlayerInput) PlayerState {
	p.Vel.X = orZero(Friction * p.Vel.X)
	p.Vel.Y = orZero(Friction * p.Vel.Y)

	if !p.onSolidGround() {
		return PlayerStateFalling
	}
	if input&InputClimbedUp > 0 {
		if p.startLadderClimbing(input) == PlayerStateLadderClimbing {
			return PlayerStateLadderClimbing
		}
	}
	_, underfoot := p.cellUnderFoot()
	if input&InputClimbedDown > 0 && underfoot&CollideLadderTop > 0 {
		if p.startLadderClimbing(input) == PlayerStateLadderClimbing {
			return PlayerStateLadderClimbing
		}
	}
	if input&InputWalked > 0 {
		return p.walkingOrRunning(input)
	}
	if input&InputJumped > 0 {
		return p.startJumping(input)
	}
	return PlayerStateIdle
}

// onSolidGround returns true iff the player is on solid ground.
func (p *Player) onSolidGround() bool {
	collides := p.Actor.Collides(p.Hitbox().Add(IVec2{0, 1}))
	p.colliding = collides
	return collides&CollidedSolid > 0 || collides&CollidedOneWay == CollidedOneWay
}

func (p *Player) clipsX(mask CollideMask) bool {
	if p.Vel.Y < 0 {
		return CollidedOneWay&mask > 0
	}
	return false
}

func (p *Player) clipsY(mask CollideMask) bool {
	if p.state == PlayerStateFalling {
		return mask == p.fallClipmask
	}
	if p.state == PlayerStateLadderClimbing {
		return mask == CollideLadderTop
	}
	if p.Vel.Y < 0 || p.state == PlayerStateOneWayClimbing {
		return CollidedOneWay&mask > 0
	}
	return false
}

// walkingOrRunning starts or continues walking or running depending on whether the run key is held.
func (p *Player) walkingOrRunning(input PlayerInput) PlayerState {
	p.sprite.SetFacing(p.Vel.X < 0)
	if input&InputRunning > 0 {
		if p.state != PlayerStateRunning {
			p.sprite.SetAnim(PlayerAnimRun, p.Vel.X < 0)
		}
		return PlayerStateRunning
	} else {
		if p.state != PlayerStateWalking {
			p.sprite.SetAnim(PlayerAnimWalk, p.Vel.X < 0)
		}
		return PlayerStateWalking
	}
}

// updateIdle performs an update and returns the next player state.
func (p *Player) updateWalking(input PlayerInput) PlayerState {
	return p.updateRunOrWalk(input, PlayerMaxWalkSpeed, false)
}

// updateIdle performs an update and returns the next player state.
func (p *Player) updateRunning(input PlayerInput) PlayerState {
	return p.updateRunOrWalk(input, PlayerMaxRunSpeed, true)
}

// updateRunOrWalk handles the update frame when running or walking.
func (p *Player) updateRunOrWalk(input PlayerInput, maxSpeed float64, canLeap bool) PlayerState {
	p.handleXVelUpdate(input, PlayerWalkAccel, maxSpeed, true)

	_ = p.MoveY()
	_ = p.MoveX(p.Hitbox()) // TODO: play bump sound / animation?

	if !p.onSolidGround() {
		return p.startFalling(maxSpeed)
	}
	if input&InputClimbedUp > 0 {
		if p.startLadderClimbing(input) == PlayerStateLadderClimbing {
			return PlayerStateLadderClimbing
		}
	}
	if input&InputJumped > 0 {
		if canLeap {
			return p.startJumpingOrLeaping(input)
		} else {
			return p.startJumping(input)
		}
	}
	if input&InputWalked == 0 {
		return p.startIdling()
	}

	return p.walkingOrRunning(input)
}

// handleXMotion handles updating the X velocity based on the current input, using the provided acceleration and max
// speed.
func (p *Player) handleXVelUpdate(input PlayerInput, accel, maxSpeed float64, useFriction bool) {
	if input&InputWalked == InputWalked {
		if useFriction { // dampen the player's movement if both bottoms are pressed
			p.Vel.X = orZero(Friction * p.Vel.X)
		} else {
			if p.Vel.X > 1e2 {
				p.Vel.X = orZero(p.Vel.X - accel)
			} else if p.Vel.X < -1e2 {
				p.Vel.X = orZero(p.Vel.X + accel)
			}
		}
	}
	if input&InputWalkedRight > 0 {
		p.Vel.X = min(p.Vel.X+accel, maxSpeed)
	}
	if input&InputWalkedLeft > 0 {
		p.Vel.X = max(p.Vel.X-accel, -maxSpeed)
	}
}

// startFalling transitions to the fall state. When this transitions occurs the prior state must provide a maxFallXSpeed
// based on the prior state.
func (p *Player) startFalling(maxFallXSpeed float64) PlayerState {
	p.sprite.SetAnim(PlayerAnimJump, p.Vel.X < 0) // TODO: pick the last and middle frames of the animation
	// test to see if we're colliding with a one-way platform, if so, increment y-velocity and don't change state.
	collides := p.Collides(p.Hitbox())
	if collides&CollidedOneWay > 0 && collides.Colliding(p.clipsY) { // if jumping up through a
		fmt.Println("attempted to fall; not allowed")
		p.Vel.Y -= PlayerOneWayLiftForce
		p.Vel.X = 0
		return PlayerStateOneWayClimbing
	}

	p.maxFallXSpeed = maxFallXSpeed
	return PlayerStateFalling
}

func (p *Player) updateFalling(input PlayerInput) (result PlayerState) {
	defer func() {
		if result != PlayerStateFalling {
			p.fallClipmask = 0
		}
	}()
	p.handleXVelUpdate(input, PlayerFallAccel, p.maxFallXSpeed, false)
	p.Vel.Y = min(p.Vel.Y+Gravity/TPS, PlayerTerminalVelocity)

	collidesY := p.MoveY()
	_ = p.MoveX(p.Hitbox())

	if p.fallClipmask != 0 && p.Pos.Y > p.fallResetY {
		p.fallClipmask = 0
	}

	if collidesY.Colliding(p.clipsY) {
		if input&InputWalked > 0 {
			return p.walkingOrRunning(input)
		} else {
			return PlayerStateIdle
		}
	}

	if input&InputClimbedUp > 0 {
		if p.startLadderClimbing(input) == PlayerStateLadderClimbing {
			return PlayerStateLadderClimbing
		}
	}
	return PlayerStateFalling
}

// startJumping starts jumping, disabling the ability to leap by unsetting the run key.
func (p *Player) startJumping(input PlayerInput) PlayerState {
	return p.startJumpingOrLeaping(input & (^InputRunning)) // no running allowed
}

// startJumpingOrLeaping starts jumping or leaping depending on whether the run bit is set in the input.
func (p *Player) startJumpingOrLeaping(input PlayerInput) PlayerState {
	p.sprite.SetAnim(PlayerAnimJump, p.Vel.X < 0)

	if input&InputClimbedDown > 0 { // if the player is jumping down off a one-way platform
		_, underfoot := p.cellUnderFoot()
		if underfoot&CollidedOneWay > 0 {
			p.Vel.Y = -PlayerLadderJumpForce
			p.fallClipmask = underfoot
			p.fallResetY = p.Hitbox().Rectangle().Max.Y
			return PlayerStateFalling
		}
	}

	if p.Vel.X < -1e-2 || 1e-2 < p.Vel.X {
		p.Vel.X = p.Vel.X * PlayerLeapCoeff
	}
	p.Vel.Y = -PlayerJumpForce
	if p.state&CollideLadder > 0 {
		p.Vel.Y = -PlayerLadderJumpForce
	}
	p.Pos.Y -= 1 // pick the player off the ground to prevent collisions with the ground from immediately ending the jump.

	if input&InputRunning > 0 {
		return PlayerStateLeaping
	}
	return PlayerStateJumping
}

// updateIdle performs an update and returns the next player state.
func (p *Player) updateJumping(_ PlayerInput) PlayerState {
	return p.updateLeapingOrJumping(PlayerMaxWalkSpeed)
}

// updateLeaping performs an update and returns the next player state.
func (p *Player) updateLeaping(_ PlayerInput) PlayerState {
	return p.updateLeapingOrJumping(PlayerMaxRunSpeed)
}

func (p *Player) updateLeapingOrJumping(maxFallXSpeed float64) PlayerState {
	p.Vel.Y = orZero(p.Vel.Y + Gravity/TPS)

	collidesY := p.MoveY()
	_ = p.MoveX(p.Hitbox())

	if collidesY.Colliding(p.clipsY) {
		p.Vel.Y = 0
		return p.startFalling(maxFallXSpeed)
	}

	if p.Vel.Y > -0.25 {
		return p.startFalling(math.Abs(p.Vel.X))
	}
	return p.state // don't change the current state; either leaping or jumping
}

// startLadderClimbing performs a quick collision check to see if a ladder is underfoot, and starts climbing if so. The
// caller should check the return value to ensure a ladder was found before proceeding.
func (p *Player) startLadderClimbing(input PlayerInput) PlayerState {
	// TODO: we don't have animations for this.
	// test point under foot
	coords, cell := p.cellUnderFoot()
	if cell&CollideLadder == 0 {
		return p.state // don't change state unless we're under a ladder.
	}
	if input&InputClimbedUp > 0 && cell&CollideLadderTop == CollideLadderTop { // don't climb up at tops
		return p.state
	}
	if input&InputClimbedDown > 0 && cell&CollideLadderBot == CollideLadderBot { // don't climb down at bottoms
		return p.state
	}
	p.Pos.X = int(coords.X) // center the player on the ladder (TODO: probably a bit too quickly..)
	p.Vel.Y = 0             // player catches themselves and stops all movement.
	p.Vel.X = 0
	return PlayerStateLadderClimbing
}

func (p *Player) updateLadderClimbing(input PlayerInput) PlayerState {
	// ignore X movement until you jump off
	if input&InputClimbed == InputClimbed {
		if p.Vel.Y > 1e2 { // start dampening the player's movement.
			p.Vel.Y = orZero(p.Vel.Y - PlayerClimbAccel)
		} else if p.Vel.X < -1e2 {
			p.Vel.Y = orZero(p.Vel.Y + PlayerClimbAccel)
		}
	} else if input&InputClimbedDown > 0 {
		p.Vel.Y = min(p.Vel.Y+PlayerClimbAccel, PlayerMaxLadderSpeed)
	} else if input&InputClimbedUp > 0 {
		p.Vel.Y = max(p.Vel.Y-PlayerClimbAccel, -PlayerMaxLadderSpeed)
	} else {
		p.Vel.Y = 0
	}

	collidesY := p.MoveY()
	if collidesY&CollidedSolid > 0 {
		p.Vel.Y = 0
	}

	_, underfoot := p.cellUnderFoot()
	if underfoot&CollideLadder == 0 {
		return p.startFalling(PlayerMaxWalkSpeed)
	} else if underfoot == CollideLadderBot && p.onSolidGround() {
		return p.startFalling(PlayerMaxWalkSpeed)
	}

	jumpedDown := InputJumped | InputClimbedDown
	if input&jumpedDown == jumpedDown { // jumped while climb down button pressed: no vertical lift
		return p.startFalling(PlayerMaxWalkSpeed)
	} else if input&InputJumped > 0 {
		if input&InputWalked > 0 { // move in x-direction before jumping, if a direction is pressed.
			p.handleXVelUpdate(input, PlayerWalkAccel, PlayerMaxWalkSpeed, true)
			if p.Vel.Mag() > 1.5 { // ladder stickiness constant
				return p.startFalling(PlayerMaxWalkSpeed)
			}
		}
		return p.startJumping(input)
	}
	return PlayerStateLadderClimbing
}

func (p *Player) updateOneWayClimbing(input PlayerInput) PlayerState {
	p.Vel.Y -= PlayerOneWayLiftForce
	p.Vel.X = 0
	collidesY := p.MoveY()
	_ = p.MoveX(p.Hitbox())
	if !collidesY.Colliding(p.clipsY) {
		return PlayerStateIdle
	}

	return PlayerStateOneWayClimbing
}

// Hitbox retrieves the bounds of the current image.
func (p *Player) Hitbox() (result IRect) {
	result.X, result.Y = p.Pos.X, p.Pos.Y
	result.W, result.H = p.sprite.Bounds().Dx(), p.sprite.Bounds().Dy()
	return result
}

// handleInput handles all player input and returns PlayerInput flags which are used to handle state changes.
func (p *Player) handleInput() PlayerInput {
	var inputFlags PlayerInput

	p.keys = inpututil.AppendPressedKeys(p.keys[:0]) // TODO: virtualize input from multiple sources.
	for _, key := range p.keys {
		switch key {
		case ebiten.KeyA:
			inputFlags = inputFlags | InputWalkedLeft
		case ebiten.KeyD:
			inputFlags = inputFlags | InputWalkedRight
		case ebiten.KeyW:
			inputFlags = inputFlags | InputClimbedUp
		case ebiten.KeyS:
			inputFlags = inputFlags | InputClimbedDown
		case ebiten.KeySpace:
			inputFlags = inputFlags | InputJumped
		case ebiten.KeyShift:
			inputFlags = inputFlags | InputRunning
		}
	}
	return inputFlags
}
