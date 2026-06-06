package main

import (
	"image"
	"image/color"
	"math"
	"math/rand"
)

type Tile struct {
	X, Y, W, H int
	Img        image.Image
	C          color.RGBA
}

// generate lays out a Pinterest-style masonry collage inside a safe zone
// defined by per-side margins, with a constant gap between cells.
//
// Algorithm:
//  1. Pick 12-15 random images. For each image, read its aspect (w/h).
//  2. Try every column count from 2 to min(count, 10). For each
//     candidate, distribute the images as evenly as possible and
//     compute the harmonic-style aspect of each column:
//     col_aspect = 1 / sum(1 / image_aspect)
//     Sum the column aspects to get the total collage aspect.
//  3. Pick the column layout whose total aspect is closest to the safe
//     zone's aspect (i.e. the one that fits the screen best with the
//     least letterboxing).
//  4. Fit the chosen collage inside the safe zone: if it's wider than
//     the safe zone, width is binding and the collage is vertically
//     letterboxed; if it's taller, height is binding and the collage is
//     horizontally letterboxed. The collage is centered inside the
//     safe zone — the unused space becomes background.
//  5. For each column: col_w = col_aspect * collage_h. For each image:
//     h = col_w / image_aspect. The "cell" is (col_w - gap) × (h - gap),
//     inset by gap/2 from the column bounds. The photo is fit inside
//     the cell preserving its aspect, centered. Empty space inside the
//     cell becomes background (the same color as the canvas).
//
// All math is in float until the very end so column widths can vary
// naturally. The result is a list of Tiles that already carry the final
// pixel rectangles for each photo — render just pastes them in place.
func generate(width, height, count int, images []image.Image, bg color.RGBA,
	marginTop, marginBottom, marginLeft, marginRight, gap int) []Tile {

	marginT := marginTop
	marginB := marginBottom
	marginL := marginLeft
	marginR := marginRight
	if marginT < 0 {
		marginT = 0
	}
	if marginB < 0 {
		marginB = 0
	}
	if marginL < 0 {
		marginL = 0
	}
	if marginR < 0 {
		marginR = 0
	}
	if gap < 0 {
		gap = 0
	}

	safeW := width - marginL - marginR
	safeH := height - marginT - marginB
	if safeW < 1 {
		safeW = 1
	}
	if safeH < 1 {
		safeH = 1
	}

	if len(images) == 0 {
		return nil
	}
	if count < 1 {
		count = 1
	}
	if count > len(images) {
		count = len(images)
	}

	perm := rand.Perm(len(images))
	indices := perm[:count]

	aspects := make([]float64, count)
	for i, idx := range indices {
		pb := images[idx].Bounds()
		aspects[i] = float64(pb.Dx()) / float64(pb.Dy())
	}

	// Try every column count and pick the one whose total collage
	// aspect is closest to the safe zone aspect.
	targetAspect := float64(safeW) / float64(safeH)
	bestCols := 1
	bestDist := []int{count}
	bestScore := 0.0
	maxC := count
	if maxC > 10 {
		maxC = 10
	}
	for C := 2; C <= maxC; C++ {
		dist := make([]int, C)
		base := count / C
		rem := count % C
		for i := 0; i < C; i++ {
			if i < rem {
				dist[i] = base + 1
			} else {
				dist[i] = base
			}
		}

		colAspects := make([]float64, C)
		idx := 0
		for c, n := range dist {
			invSum := 0.0
			for j := idx; j < idx+n; j++ {
				invSum += 1.0 / aspects[j]
			}
			idx += n
			if invSum > 0 {
				colAspects[c] = 1.0 / invSum
			}
		}

		collageAspect := 0.0
		for _, a := range colAspects {
			collageAspect += a
		}

		var score float64
		if collageAspect > targetAspect {
			score = targetAspect / collageAspect
		} else {
			score = collageAspect / targetAspect
		}
		if score > bestScore {
			bestScore = score
			bestCols = C
			bestDist = dist
		}
	}

	// Compute column aspects for the winning distribution.
	colAspects := make([]float64, bestCols)
	idx := 0
	for c, n := range bestDist {
		invSum := 0.0
		for j := idx; j < idx+n; j++ {
			invSum += 1.0 / aspects[j]
		}
		idx += n
		if invSum > 0 {
			colAspects[c] = 1.0 / invSum
		}
	}
	collageAspect := 0.0
	for _, a := range colAspects {
		collageAspect += a
	}

	// Fit the collage inside the safe zone and center it.
	var collageW, collageH, xOffset, yOffset float64
	if collageAspect > targetAspect {
		collageW = float64(safeW)
		collageH = collageW / collageAspect
		xOffset = float64(marginL)
		yOffset = float64(marginT) + (float64(safeH)-collageH)/2.0
	} else {
		collageH = float64(safeH)
		collageW = collageH * collageAspect
		yOffset = float64(marginT)
		xOffset = float64(marginL) + (float64(safeW)-collageW)/2.0
	}

	// Group selected images by column.
	columns := make([][]int, bestCols)
	idx = 0
	for c, n := range bestDist {
		columns[c] = indices[idx : idx+n]
		idx += n
	}

	tiles := make([]Tile, 0, count)
	currentX := xOffset
	for c, colImgs := range columns {
		colW := colAspects[c] * collageH

		currentY := yOffset
		for _, imgIdx := range colImgs {
			pb := images[imgIdx].Bounds()
			imgAspect := float64(pb.Dx()) / float64(pb.Dy())
			h := colW / imgAspect

			cellX := currentX + float64(gap)/2.0
			cellY := currentY + float64(gap)/2.0
			cellW := colW - float64(gap)
			cellH := h - float64(gap)

			if cellW > 0 && cellH > 0 {
				var fitW, fitH int
				if imgAspect > cellW/cellH {
					fitW = int(math.Floor(cellW))
					fitH = int(math.Floor(cellW / imgAspect))
				} else {
					fitH = int(math.Floor(cellH))
					fitW = int(math.Floor(cellH * imgAspect))
				}
				if fitW > 0 && fitH > 0 {
					ox := cellX + (cellW-float64(fitW))/2.0
					oy := cellY + (cellH-float64(fitH))/2.0
					tiles = append(tiles, Tile{
						X: int(math.Round(ox)),
						Y: int(math.Round(oy)),
						W: fitW,
						H: fitH,
						Img: images[imgIdx],
						C:   bg,
					})
				}
			}

			currentY += h
		}
		currentX += colW
	}

	return tiles
}
