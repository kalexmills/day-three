package internal

import (
	"embed"
	"errors"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/niftysoft/2d-platformer/internal/ldtk"
	"image"
	_ "image/png"
)

//go:embed gamedata
var gameData embed.FS

// UID is an int64 that is used to represent a UID from LDtk.
type UID = int64

// GameData represents all the game data loaded from LDtk, including all loaded tilesets.
type GameData struct {
	json       *ldtk.LdtkJSON        // json is a straightforward representation of the LDtk JSON output.
	Tilesets   map[UID]*ebiten.Image // Tilesets is a list of all images loaded as part of the tileset.
	Levels     map[UID]*Level        // Levels is a list of levels by UID assigned in LDtk.
	LevelsByID map[string]*Level     // LevelsByID references the same level constructs via the name provided in the LDtk editor.

	LevelStart UID // LevelStart is the UID of the level where the playerStart entity is found.
}

// ldtkPath is the path to the LDtk file representing all of this game's level data.
const ldtkPath = "trash-knight-level-1.ldtk"

// LoadGameData loads all gamedata from the expected ldtkPath relative to the gamedata embed folder, including all
// referenced tilesets, which are expected to be found under gamedata/atlas.
func LoadGameData() (result GameData, err error) {
	result.LevelStart = -1

	result.json, err = LoadLdtkJSON(ldtkPath)
	if err != nil {
		return GameData{}, err
	}
	result.Tilesets, err = LoadTilesets(result.json)
	if err != nil {
		return GameData{}, err
	}
	result.Levels, err = LoadLevels(result.json)
	if err != nil {
		return GameData{}, err
	}
	result.LevelsByID = make(map[string]*Level, len(result.Levels))
	for uid, level := range result.Levels {
		result.LevelsByID[level.ID] = level
		for _, entity := range level.Entities {
			if entity.ID == EtyPlayer {
				result.LevelStart = uid
			}
		}
	}
	if result.LevelStart == -1 {
		return GameData{}, errors.New("no player start found")
	}
	return result, nil
}

// LoadLdtkJSON loads a LDtk file from the provided path, relative to the gamedata embed folder.
//
// https://ldtk.io/json/ for details on the spec.
func LoadLdtkJSON(filename string) (*ldtk.LdtkJSON, error) {
	f, err := gameData.Open("gamedata/" + filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	result, err := ldtk.UnmarshalLdtkReader(f)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// LoadTilesets loads all tilesets used in the provided LDTK file as ebiten images; keyed by UID.
func LoadTilesets(json *ldtk.LdtkJSON) (map[UID]*ebiten.Image, error) {
	result := make(map[UID]*ebiten.Image)
	for _, lvl := range json.Levels {
		for _, lay := range lvl.LayerInstances {
			if lay.TilesetDefUid == nil {
				continue
			}
			if _, ok := result[*lay.TilesetDefUid]; ok {
				continue
			}
			img, err := loadImage(*lay.TilesetRelPath)
			if err != nil {
				return nil, err
			}
			result[*lay.TilesetDefUid] = ebiten.NewImageFromImage(img)
		}
	}
	return result, nil
}

// LoadLevels loads all data for levels which are stored in the provided json into memory, keyed by UID.
func LoadLevels(json *ldtk.LdtkJSON) (map[UID]*Level, error) {
	result := make(map[UID]*Level, len(json.Levels))
	for _, lvl := range json.Levels {
		level := &Level{
			ID:          lvl.Identifier,
			WorldCoords: IVec2{X: int(lvl.WorldX), Y: int(lvl.WorldY)},
			PxDims:      IDim{W: int(lvl.PxWid), H: int(lvl.PxHei)},
			layersByID:  make(map[string]*TileLayer),
		}
		n := len(lvl.LayerInstances)
		level.layers = make([]*TileLayer, n)
		for i, lay := range lvl.LayerInstances {
			layer := loadLayer(&lay)
			level.layersByID[lay.Identifier] = layer
			level.layers[n-i-1] = layer // fill in reverse to correct draw order

			// add all layer entities to level
			level.Entities = append(level.Entities, layer.Entities...)
		}
		result[lvl.Uid] = level
	}
	return result, nil
}

func loadLayer(layer *ldtk.LayerInstance) *TileLayer {
	result := &TileLayer{
		ID:         layer.Identifier,
		UID:        layer.LayerDefUid,
		Opacity:    float32(layer.Opacity),
		GridSize:   int(layer.GridSize),
		CellDims:   IDim{W: int(layer.CWid), H: int(layer.CHei)},
		PxOffsets:  IVec2{X: int(layer.PxOffsetX), Y: int(layer.PxOffsetY)},
		TileSetUID: layer.TilesetDefUid,
	}
	// load tiles
	// only one of layer.AutoLayerTiles or layer.GridTiles will be non-empty; per spec.
	result.Tiles = make([]Tile, 0, max(len(layer.AutoLayerTiles), len(layer.GridTiles)))
	loadTiles(result, layer.AutoLayerTiles)
	loadTiles(result, layer.GridTiles)

	// load int grid
	result.Grid = make([]int, len(layer.IntGridCSV))
	for i, x := range layer.IntGridCSV {
		result.Grid[i] = int(x)
	}

	// load any entities
	loadEntities(result, layer.EntityInstances)
	return result
}

func loadTiles(out *TileLayer, tiles []ldtk.TileInstance) {
	for _, tile := range tiles { // only one of AutoLayerTiles or GridTiles will be non-empty
		out.Tiles = append(out.Tiles, Tile{ // no loss-of-precision or bounds-check needed due to spec
			FlipBits:  byte(tile.F),
			PxCoords:  IVec2{X: int(tile.Px[0]), Y: int(tile.Px[1])},
			SrcCoords: IVec2{X: int(tile.Src[0]), Y: int(tile.Src[1])},
			TileID:    int(tile.T),
		})
	}
}

func loadEntities(out *TileLayer, entities []ldtk.EntityInstance) {
	for _, entity := range entities {
		out.Entities = append(out.Entities, &Entity{
			ID:       entity.Identifier,
			IID:      uuid.MustParse(entity.Iid), // safe per spec
			PxCoords: IVec2{X: int(entity.Px[0]), Y: int(entity.Px[1])},
			Dim:      IDim{W: int(entity.Width), H: int(entity.Height)},
		})
	}
}

func loadImage(path string) (image.Image, error) {
	f, err := gameData.Open("gamedata/" + path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// Level stores a layer of tiles together along with all collision elements needed.
type Level struct {
	ID          string                // ID is the user-friendly level identifier specified in the LDtk editor.
	layers      []*TileLayer          // layers is the list of layers in draw order.
	layersByID  map[string]*TileLayer // layersByID maps string IDs set by the user in LDtk to layers.
	WorldCoords IVec2                 // WorldCoords represents the level's world coordinates in pixels.
	PxDims      IDim                  // PxDims represents the dimensions of the level in pixels.
	Entities    []*Entity             // Entities is the union of all entities found in all layers in this level.
}

// A TileLayer can contain entities, tiles, or an integer Grid. When a TileLayer contains entities it will never
// contain tiles or an int Grid.
type TileLayer struct {
	ID         string
	UID        UID
	Opacity    float32   // Opacity is the opacity of this tile layer. Float32 for ease of use with ebiten.
	GridSize   int       // GridSize is the square size of each cell in pixels.
	CellDims   IDim      // CellDims is the width and height of each cell found in this layer.
	PxOffsets  IVec2     // PxOffsets stores the total pixel offset of this layer from the upper-left corner of the level.
	TileSetUID *int64    // TileSetUID is only set if this layer has an associated tileset.
	Tiles      []Tile    // Tiles per cell laid out as idx = x + y*w.
	Grid       []int     // Grid is the values of the int grid per cell laid out as idx = x + y*w.
	Entities   []*Entity // Entities is the list of entities found on this layer.
}

// Entity represents raw entity data loaded from LDtk.
type Entity struct {
	ID       string    // ID is the unique identifier corresponding to the entity's type.
	IID      uuid.UUID // IID is the instance identifier of this particular entity.
	PxCoords IVec2     // PxCoords are the pixel coordinates of this entity.
	Dim      IDim      // Dim is the dimensions of the entity in pixel coordinates.
}

// Tile represents one tile to be drawn in this layer.
type Tile struct {
	PxCoords  IVec2 // PxCoords is the pixel coordinates of the tile in its layer.
	SrcCoords IVec2 // SrcCoords are the pixel coordinates of the tile in its tileset.
	TileID    int   // TileID is the ID of this tile in its tileset.
	// FlipBits represents whether the tile is flipped horizontally or vertically or both.
	// 0 = no flip; 1 = x flip only; 2 = y flip only; 3 = x and y flip.
	FlipBits byte
}

// GeoM retrieves the world matrix for this tile.
func (t Tile) GeoM(gridSize int) ebiten.GeoM {
	result := ebiten.GeoM{}
	switch t.FlipBits {
	case 0x1: // X flip
		result.Scale(-1, 1)
		result.Translate(float64(gridSize), 0)
	case 0x2: // Y flip
		result.Scale(1, -1)
		result.Translate(0, float64(gridSize))
	case 0x3: // X and Y flip
		result.Scale(-1, -1)
		result.Translate(float64(gridSize), float64(gridSize))
	}
	result.Translate(float64(t.PxCoords.X), float64(t.PxCoords.Y))
	return result
}

// Rectangle returns the src rectangle for this tile in the tileset.
func (t Tile) Rectangle(gridSize int) image.Rectangle {
	return image.Rect(t.SrcCoords.X, t.SrcCoords.Y, t.SrcCoords.X+gridSize, t.SrcCoords.Y+gridSize)
}
