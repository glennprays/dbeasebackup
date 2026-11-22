package build

import (
	"fmt"
	"html/template"
	"math"
	"strings"
)

// The strata artwork is the site's one decorative element: stacked wave
// bands in the accent family, echoing the logo's stacked disks and backups
// piling up over time. Paths are generated, not hand drawn, so the shape is
// reproducible and each band loops seamlessly when it drifts sideways.

const (
	strataW = 1600 // one period; the path spans two so translateX(-W) loops
	strataH = 640
)

// strataLayer describes one band: its baseline, its wave terms (integer
// cycles per period keep the loop seamless) and its gradient stops.
type strataLayer struct {
	base  float64
	waves [][3]float64 // amplitude, cycles per period, phase
	stops []string     // CSS custom properties, left to right
	speed int          // seconds per period; 0 = static
}

var strataLayers = []strataLayer{
	{base: 150, waves: [][3]float64{{38, 1, 0.3}, {14, 3, 1.9}}, stops: []string{"--strata-1", "--strata-2", "--strata-1"}, speed: 90},
	{base: 250, waves: [][3]float64{{46, 1, 2.1}, {12, 2, 0.4}}, stops: []string{"--strata-2", "--strata-3", "--strata-2"}, speed: 70},
	{base: 345, waves: [][3]float64{{40, 1, 4.0}, {16, 3, 2.6}}, stops: []string{"--strata-3", "--strata-4", "--strata-3"}, speed: 56},
	{base: 430, waves: [][3]float64{{34, 2, 1.1}, {18, 1, 5.2}}, stops: []string{"--strata-4", "--strata-5", "--strata-4"}, speed: 44},
	{base: 520, waves: [][3]float64{{30, 1, 3.3}, {10, 4, 0.8}}, stops: []string{"--strata-5", "--strata-6", "--strata-5"}, speed: 36},
}

// wavePath returns a closed path: the wave across two periods, then down
// to the bottom edge. Points every 20px, joined with straight segments
// (smooth enough at this density and cheap to render).
func wavePath(l strataLayer) string {
	var sb strings.Builder
	for x := 0; x <= 2*strataW; x += 20 {
		y := l.base
		for _, w := range l.waves {
			y += w[0] * math.Sin(2*math.Pi*w[1]*float64(x)/strataW+w[2])
		}
		cmd := "L"
		if x == 0 {
			cmd = "M"
		}
		fmt.Fprintf(&sb, "%s%d %.1f", cmd, x, y)
	}
	fmt.Fprintf(&sb, "L%d %dL0 %dZ", 2*strataW, strataH, strataH)
	return sb.String()
}

// StrataSVG renders the artwork. animated adds the drift classes; the
// static variant is used for the OG image.
func StrataSVG(animated bool) template.HTML {
	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg class="strata" viewBox="0 0 %d %d" preserveAspectRatio="xMidYMid slice" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" focusable="false">`, strataW, strataH)
	sb.WriteString("<defs>")
	for i, l := range strataLayers {
		fmt.Fprintf(&sb, `<linearGradient id="strata-g%d" x1="0" y1="0" x2="1" y2="0.35">`, i)
		for j, s := range l.stops {
			off := float64(j) / float64(len(l.stops)-1) * 100
			fmt.Fprintf(&sb, `<stop offset="%.0f%%" style="stop-color: var(%s)"/>`, off, s)
		}
		sb.WriteString("</linearGradient>")
	}
	sb.WriteString("</defs>")
	for i, l := range strataLayers {
		cls := "strata__band"
		style := ""
		if animated && l.speed > 0 {
			cls += " strata__band--drift"
			style = fmt.Sprintf(` style="animation-duration: %ds"`, l.speed)
		}
		fmt.Fprintf(&sb, `<g class="%s"%s><path d="%s" fill="url(#strata-g%d)"/></g>`, cls, style, wavePath(l), i)
	}
	sb.WriteString("</svg>")
	return template.HTML(sb.String())
}
