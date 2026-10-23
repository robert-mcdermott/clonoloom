package network

// A deterministic force drawing, inspired by Fruchterman and Reingold (1991,
// Software: Practice and Experience 21:1129–1164, doi:10.1002/spe.4380211102).
// ClonoLoom's choices: a fixed grid start, inverse-squared repulsion, linear
// edge attraction, bounded updates for 60 iterations, and shelf packing.
// These coordinates have no scientific distance interpretation. Whole components
// are drawn largest first, with a prefix of an oversized component when necessary;
// the graph and component membership are never truncated with the drawing.

import (
	"context"
	"math"
)

type Point struct {
	Node int     `json:"node"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

type Drawing struct {
	Points  []Point `json:"points"`
	Omitted int     `json:"omitted"`
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
}

func Layout(ctx context.Context, g *Graph, limit int) (Drawing, error) {
	out := Drawing{Points: []Point{}}
	limit = min(max(limit, 0), 2000)
	type tile struct {
		points        []Point
		width, height float64
	}
	var tiles []tile
	area := 0.0
	shown := 0
	for _, c := range g.Components {
		n := min(len(c.Nodes), limit-shown)
		if n <= 0 {
			break
		}
		width := int(math.Ceil(math.Sqrt(float64(n))))
		points := make([]Point, n)
		local := map[int]int{}
		for i, id := range c.Nodes[:n] {
			points[i] = Point{id, float64(i%width) * 20, float64(i/width) * 20}
			local[id] = i
		}
		edges := []Edge{}
		for _, e := range g.Edges {
			a, okA := local[e.A]
			b, okB := local[e.B]
			if okA && okB {
				edges = append(edges, Edge{A: a, B: b})
			}
		}
		dx, dy := make([]float64, n), make([]float64, n)
		for step := 0; step < 60; step++ {
			if err := ctx.Err(); err != nil {
				return Drawing{}, err
			}
			clear(dx)
			clear(dy)
			for i := 0; i < n; i++ {
				for j := 0; j < i; j++ {
					x, y := points[i].X-points[j].X, points[i].Y-points[j].Y
					square := float64(x*x) + float64(y*y) + 1
					force := 100 / square
					fx, fy := float64(x*force), float64(y*force)
					dx[i] += fx
					dy[i] += fy
					dx[j] -= fx
					dy[j] -= fy
				}
			}
			for _, e := range edges {
				x, y := points[e.B].X-points[e.A].X, points[e.B].Y-points[e.A].Y
				fx, fy := float64(x*0.015), float64(y*0.015)
				dx[e.A] += fx
				dy[e.A] += fy
				dx[e.B] -= fx
				dy[e.B] -= fy
			}
			for i := range points {
				points[i].X += max(-5, min(5, dx[i]))
				points[i].Y += max(-5, min(5, dy[i]))
			}
		}
		minX, maxX, minY, maxY := points[0].X, points[0].X, points[0].Y, points[0].Y
		for _, p := range points {
			minX = min(minX, p.X)
			maxX = max(maxX, p.X)
			minY = min(minY, p.Y)
			maxY = max(maxY, p.Y)
		}
		w, h := maxX-minX+40, maxY-minY+40
		for i := range points {
			points[i].X = points[i].X - minX + 20
			points[i].Y = points[i].Y - minY + 20
		}
		tiles = append(tiles, tile{points, w, h})
		area += float64(w * h)
		shown += n
	}
	// Choose a shelf width from the actual occupied area, targeting a landscape
	// canvas rather than a fixed width that squeezes many components into a strip.
	shelf := math.Sqrt(float64(area * (16.0 / 9.0)))
	xOffset, yOffset, rowHeight := 0.0, 0.0, 0.0
	for _, tile := range tiles {
		if xOffset > 0 && xOffset+tile.width > shelf {
			xOffset = 0
			yOffset += rowHeight
			rowHeight = 0
		}
		for _, p := range tile.points {
			p.X += xOffset
			p.Y += yOffset
			out.Points = append(out.Points, p)
		}
		xOffset += tile.width
		rowHeight = max(rowHeight, tile.height)
		out.Width = max(out.Width, xOffset)
		out.Height = max(out.Height, yOffset+tile.height)
	}
	out.Omitted = len(g.Membership) - len(out.Points)
	return out, ctx.Err()
}
