package display

import (
	"bytes"
	"embed"
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// characterAssets contain purpose-built, non-overlapping vector layers. Each
// layer is rasterized independently; no finished image is cropped or inferred.
//
//go:embed assets/stylized/*.svg
var characterAssets embed.FS

type characterSprite struct {
	name                                string
	backHair, leftLeg, rightLeg         *ebiten.Image
	leftArm, rightArm, body, head, prop *ebiten.Image
}

func loadCharacterSprites() ([]characterSprite, error) {
	characters := []struct{ id, name string }{
		{"akari", "Akari"}, {"sora", "Sora"}, {"mizuki", "Mizuki"},
		{"ren", "Ren"}, {"yuna", "Yuna"}, {"kaede", "Kaede"},
		{"haru", "Haru"}, {"noa", "Noa"}, {"kira", "Kira"}, {"aoi", "Aoi"},
	}
	result := make([]characterSprite, 0, len(characters))
	for _, character := range characters {
		load := func(layer string) (*ebiten.Image, error) { return loadCharacterLayer(character.id, layer) }
		backHair, err := load("back-hair")
		if err != nil {
			return nil, err
		}
		leftLeg, err := load("leg-left")
		if err != nil {
			return nil, err
		}
		rightLeg, err := load("leg-right")
		if err != nil {
			return nil, err
		}
		leftArm, err := load("arm-left")
		if err != nil {
			return nil, err
		}
		rightArm, err := load("arm-right")
		if err != nil {
			return nil, err
		}
		body, err := load("body")
		if err != nil {
			return nil, err
		}
		head, err := load("head-group")
		if err != nil {
			return nil, err
		}
		prop, err := load("prop")
		if err != nil {
			return nil, err
		}
		result = append(result, characterSprite{name: character.name, backHair: backHair, leftLeg: leftLeg, rightLeg: rightLeg, leftArm: leftArm, rightArm: rightArm, body: body, head: head, prop: prop})
	}
	return result, nil
}

func loadCharacterLayer(id, layer string) (*ebiten.Image, error) {
	path := fmt.Sprintf("assets/stylized/%s-%s.svg", id, layer)
	data, err := characterAssets.ReadFile(path)
	if err != nil {
		return nil, err
	}
	icon, err := oksvg.ReadIconStream(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	const width, height = 128, 140
	icon.SetTarget(0, 0, width, height)
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	scanner := rasterx.NewScannerGV(width, height, rgba, rgba.Bounds())
	icon.Draw(rasterx.NewDasher(width, height, scanner), 1)
	return ebiten.NewImageFromImage(rgba), nil
}
