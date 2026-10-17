// Package theme contains palettes shared by the terminal and game UIs.
package theme

import "image/color"

// Palette defines one selectable visual direction.
type Palette struct {
	ID, Name                                                 string
	Background, Panel, Line, Text, Muted, Accent, Dot, Hover color.RGBA
	Players                                                  [4]color.RGBA
}

var All = []Palette{
	{ID: "modern", Name: "Modern Arcade", Background: hex(0x080b16), Panel: hex(0x101525), Line: hex(0x252d46), Text: hex(0xf5f7ff), Muted: hex(0x8390af), Accent: hex(0x61f2cc), Dot: hex(0x46516f), Hover: hex(0xffffff), Players: [4]color.RGBA{hex(0xff5e8b), hex(0x62a8ff), hex(0xffd166), hex(0x9b7bff)}},
	{ID: "paper", Name: "Graph Paper", Background: hex(0xe8dfca), Panel: hex(0xf5eedc), Line: hex(0xb6aa8e), Text: hex(0x252a2a), Muted: hex(0x756f61), Accent: hex(0xb23b35), Dot: hex(0x706a5e), Hover: hex(0x7d3cff), Players: [4]color.RGBA{hex(0xc43b35), hex(0x2569a2), hex(0xd09324), hex(0x648f55)}},
	{ID: "studio", Name: "Minimal Studio", Background: hex(0xf4f1eb), Panel: hex(0xfffefa), Line: hex(0xd9d4ca), Text: hex(0x181a1b), Muted: hex(0x787a78), Accent: hex(0x1f5cff), Dot: hex(0x73716c), Hover: hex(0xffa000), Players: [4]color.RGBA{hex(0xf04e35), hex(0x1f5cff), hex(0x09a476), hex(0xa04ccc)}},
	{ID: "retro", Name: "Retro Terminal", Background: hex(0x031209), Panel: hex(0x061b0e), Line: hex(0x18562d), Text: hex(0xa5ffb9), Muted: hex(0x4fac68), Accent: hex(0x60ff85), Dot: hex(0x347847), Hover: hex(0xff4fd8), Players: [4]color.RGBA{hex(0xf6ff66), hex(0x60ff85), hex(0x50e3ff), hex(0xff7799)}},
	{ID: "tabletop", Name: "Classic Tabletop", Background: hex(0x173b31), Panel: hex(0x1e2c29), Line: hex(0x48635b), Text: hex(0xfff7dc), Muted: hex(0xa4b9ae), Accent: hex(0xe9c46a), Dot: hex(0x709083), Hover: hex(0xffffff), Players: [4]color.RGBA{hex(0xe76f51), hex(0x49a8c8), hex(0xe9c46a), hex(0x9b7bb5)}},
	{ID: "blueprint", Name: "Blueprint", Background: hex(0x082d52), Panel: hex(0x0a3762), Line: hex(0x3f759c), Text: hex(0xe7f6ff), Muted: hex(0x8eb9d4), Accent: hex(0x8ff7ff), Dot: hex(0x5889ad), Hover: hex(0xff4df0), Players: [4]color.RGBA{hex(0xffdf6b), hex(0x7af2ff), hex(0xff8ea1), hex(0xb8ff9d)}},
	{ID: "playful", Name: "Playful Geometry", Background: hex(0xfff6ec), Panel: hex(0xfffcf7), Line: hex(0xf0cdb4), Text: hex(0x392d39), Muted: hex(0x987f8f), Accent: hex(0x7548ff), Dot: hex(0x9a8490), Hover: hex(0x111827), Players: [4]color.RGBA{hex(0xff507a), hex(0x4b79ff), hex(0xf4b52c), hex(0x36b895)}},
	{ID: "pro", Name: "Monochrome Pro", Background: hex(0x0d0d0e), Panel: hex(0x161617), Line: hex(0x353538), Text: hex(0xffffff), Muted: hex(0x929296), Accent: hex(0xffffff), Dot: hex(0x666668), Hover: hex(0x00ff88), Players: [4]color.RGBA{hex(0xff3b30), hex(0xf2f2f2), hex(0xffcc00), hex(0x5ac8fa)}},
	{ID: "synth", Name: "Synthwave Grid", Background: hex(0x100826), Panel: hex(0x1a0d35), Line: hex(0x4a2672), Text: hex(0xfff2ff), Muted: hex(0xaa87c9), Accent: hex(0xff54d8), Dot: hex(0x65517f), Hover: hex(0xffffff), Players: [4]color.RGBA{hex(0xff50c8), hex(0x3de9ff), hex(0xffe45e), hex(0x8d66ff)}},
	{ID: "cozy", Name: "Cozy Desktop", Background: hex(0xeee9e1), Panel: hex(0xfaf7f1), Line: hex(0xd7cdc0), Text: hex(0x313437), Muted: hex(0x85827d), Accent: hex(0x2e7d6e), Dot: hex(0x8b857d), Hover: hex(0x743ee8), Players: [4]color.RGBA{hex(0xd06c55), hex(0x4b79a1), hex(0xd5a43d), hex(0x856a9d)}},
}

func hex(v uint32) color.RGBA { return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255} }

// ByID returns a palette, falling back to Modern Arcade.
func ByID(id string) Palette {
	for _, p := range All {
		if p.ID == id {
			return p
		}
	}
	return All[0]
}

// Next returns the following palette, wrapping at the end.
func Next(id string) Palette {
	for i, p := range All {
		if p.ID == id {
			return All[(i+1)%len(All)]
		}
	}
	return All[0]
}
