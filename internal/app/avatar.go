package app

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/ilinmu/vita/internal/model"
)

// renderMemberAvatar draws a circular vector avatar using the member's accent color:
// cute boy/girl cartoon avatars for children, and clean minimalist silhouettes for adults.
func renderMemberAvatar(c *ui.Context, m model.Member, size float32) ui.Element {
	accent := parseMemberAccent(m.ColorHex)
	isChild := m.IsChildAt(time.Now())

	return ui.Box(c).Size(size, size).Radius(size / 2).Clip().Draw(func(p *ui.Painter, r ui.Rect) {
		cx := r.X + r.W/2
		cy := r.Y + r.H/2
		rad := min(r.W, r.H) / 2
		s := rad * 2 / 48.0 // normalized to 48x48 unit space

		// Soft tinted backdrop + crisp accent ring
		bg := accent.Alpha(0.16)
		p.Fill(r, bg, rad)

		skin := ui.Hex("#FDE2C8")
		skinShadow := ui.Hex("#F7C7A3")
		hair := ui.Hex("#3B2820")
		eye := ui.Hex("#1E293B")
		cheek := ui.RGBA(244, 114, 182, 0.45)

		if !isChild {
			// Adult minimalist avatar in member's color
			head := (&ui.Path{}).Circle(cx, cy-4.5*s, 7.5*s)
			p.FillPath(head, accent)
			shoulders := (&ui.Path{}).
				MoveTo(cx-14*s, cy+21*s).
				QuadTo(cx-13*s, cy+8*s, cx, cy+8*s).
				QuadTo(cx+13*s, cy+8*s, cx+14*s, cy+21*s).
				Close()
			p.FillPath(shoulders, accent)
			if m.Gender == model.GenderFemale {
				// Subtle bob hair arc behind adult female head
				bob := (&ui.Path{}).Circle(cx, cy-5*s, 9.5*s)
				p.FillPath(bob, accent.Alpha(0.45))
				p.FillPath(head, accent)
			}
			ring := (&ui.Path{}).Circle(cx, cy, rad-1)
			p.StrokePath(ring, 1.5, accent.Alpha(0.55))
			return
		}

		// --- Child Cartoon Avatar (Boy / Girl) ---
		// Shirt / Collar at bottom
		shirt := (&ui.Path{}).
			MoveTo(cx-15*s, cy+22*s).
			QuadTo(cx-13*s, cy+10*s, cx, cy+10*s).
			QuadTo(cx+13*s, cy+10*s, cx+15*s, cy+22*s).
			Close()
		p.FillPath(shirt, accent)

		// Neck
		neck := (&ui.Path{}).
			MoveTo(cx-3.2*s, cy+8*s).LineTo(cx+3.2*s, cy+8*s).
			LineTo(cx+3.2*s, cy+13*s).LineTo(cx-3.2*s, cy+13*s).Close()
		p.FillPath(neck, skinShadow)

		if m.Gender == model.GenderFemale {
			// Twin hair buns (双丸子头)
			p.FillPath((&ui.Path{}).Circle(cx-11.5*s, cy-11.5*s, 5.2*s), hair)
			p.FillPath((&ui.Path{}).Circle(cx+11.5*s, cy-11.5*s, 5.2*s), hair)
			// Hair ties in member's color
			p.FillPath((&ui.Path{}).Circle(cx-8.8*s, cy-9.2*s, 2.0*s), accent)
			p.FillPath((&ui.Path{}).Circle(cx+8.8*s, cy-9.2*s, 2.0*s), accent)
			// Back hair crown
			p.FillPath((&ui.Path{}).Circle(cx, cy-2.5*s, 12.2*s), hair)
		} else {
			// Boy back hair cap
			p.FillPath((&ui.Path{}).Circle(cx, cy-3.2*s, 12.0*s), hair)
		}

		// Ears
		p.FillPath((&ui.Path{}).Circle(cx-11.0*s, cy+0.5*s, 2.6*s), skinShadow)
		p.FillPath((&ui.Path{}).Circle(cx+11.0*s, cy+0.5*s, 2.6*s), skinShadow)

		// Face
		p.FillPath((&ui.Path{}).Circle(cx, cy-0.5*s, 11.0*s), skin)

		if m.Gender == model.GenderFemale {
			// Girl bangs (parted rounded bangs)
			bangL := (&ui.Path{}).
				MoveTo(cx-11.2*s, cy-3.0*s).
				QuadTo(cx-7.5*s, cy-13.2*s, cx+1.0*s, cy-11.8*s).
				QuadTo(cx-4.5*s, cy-4.5*s, cx-11.2*s, cy-3.0*s).Close()
			bangR := (&ui.Path{}).
				MoveTo(cx+11.2*s, cy-3.0*s).
				QuadTo(cx+7.5*s, cy-13.2*s, cx-1.0*s, cy-11.8*s).
				QuadTo(cx+4.5*s, cy-4.5*s, cx+11.2*s, cy-3.0*s).Close()
			p.FillPath(bangL, hair)
			p.FillPath(bangR, hair)
		} else {
			// Boy playful top hair + tuft
			topHair := (&ui.Path{}).
				MoveTo(cx-11.2*s, cy-2.2*s).
				QuadTo(cx-9.5*s, cy-14.0*s, cx, cy-13.8*s).
				QuadTo(cx+9.5*s, cy-14.0*s, cx+11.2*s, cy-2.2*s).
				QuadTo(cx+5.0*s, cy-6.8*s, cx, cy-4.8*s).
				QuadTo(cx-5.5*s, cy-6.8*s, cx-11.2*s, cy-2.2*s).Close()
			p.FillPath(topHair, hair)
			// Cute top tuft
			tuft := (&ui.Path{}).
				MoveTo(cx-1.5*s, cy-12.5*s).
				QuadTo(cx+1.5*s, cy-17.2*s, cx+4.8*s, cy-12.0*s).Close()
			p.FillPath(tuft, hair)
		}

		// Rosy cheeks
		p.FillPath((&ui.Path{}).Circle(cx-6.2*s, cy+3.0*s, 2.2*s), cheek)
		p.FillPath((&ui.Path{}).Circle(cx+6.2*s, cy+3.0*s, 2.2*s), cheek)

		// Friendly eyes
		p.FillPath((&ui.Path{}).Circle(cx-4.2*s, cy+0.2*s, 1.35*s), eye)
		p.FillPath((&ui.Path{}).Circle(cx+4.2*s, cy+0.2*s, 1.35*s), eye)

		// Warm smile arc
		smile := (&ui.Path{}).
			MoveTo(cx-2.8*s, cy+4.2*s).
			QuadTo(cx, cy+6.8*s, cx+2.8*s, cy+4.2*s)
		p.StrokePath(smile, max(1.2, 1.4*s), eye)

		// Crisp outer ring in member's color
		ring := (&ui.Path{}).Circle(cx, cy, rad-1)
		p.StrokePath(ring, 1.5, accent.Alpha(0.55))
	})
}
