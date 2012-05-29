// Copyright ©2012 Dan Kortschak <dan.kortschak@adelaide.edu.au>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package pals

import (
	"fmt"
	"github.com/kortschak/biogo/feat"
	"github.com/kortschak/biogo/interval"
)

var duplicatePair = fmt.Errorf("pals: attempt to add duplicate feature pair to pile")

type Piler struct {
	intervals interval.Tree
	seen      map[string]struct{}
	overlap   int
}

func NewPiler(overlap int) *Piler {
	return &Piler{
		intervals: interval.NewTree(),
		seen:      make(map[string]struct{}),
		overlap:   overlap,
	}
}

func (self *Piler) Add(p *FeaturePair) (err error) {
	a := fmt.Sprintf("%q:[%d,%d)", p.A.Location, p.A.Start, p.A.End)
	b := fmt.Sprintf("%q:[%d,%d)", p.B.Location, p.B.Start, p.B.End)

	if _, ok := self.seen[a+b]; ok {
		return duplicatePair
	}
	if _, ok := self.seen[b+a]; ok {
		return duplicatePair
	}

	ai, err := interval.New(string(p.A.Location), p.A.Start, p.A.End, 0, []*FeaturePair{p})
	if err != nil {
		return
	}
	bi, err := interval.New(string(p.B.Location), p.B.Start, p.B.End, 0, []*FeaturePair(nil))
	if err != nil {
		return
	}

	self.merge(ai)
	self.merge(bi)
	self.seen[a+b] = struct{}{}
	self.seen[b+a] = struct{}{}

	return
}

// merge an interval into the tree adding meta data from the replaced intervals into the new interval
func (self *Piler) merge(i *interval.Interval) {
	rc := self.intervals.Merge(i, self.overlap)
	m := i.Meta.([]*FeaturePair)
	for _, ri := range rc {
		m = append(m, ri.Meta.([]*FeaturePair)...)
	}
	i.Meta = m
}

type Pile struct {
	Pile   *feat.Feature
	Images []*FeaturePair
}

// A PileFilter is used to determine whether a FeaturePair is included in a Pile
type PileFilter func(a, b *feat.Feature, pa, pb *interval.Interval) bool

// We use the Features' Meta field to point back to the containing Pile, so Meta cannot be used for other things here.
func (self *Piler) Piles(f PileFilter) (piles []*Pile, err error) {
	pm := make(map[*interval.Interval]*Pile)

	for pa := range self.intervals.TraverseAll() {
		var bi, pb *interval.Interval
		for _, p := range pa.Meta.([]*FeaturePair) {
			bi, err = interval.New(p.B.Location, p.B.Start, p.B.End, 0, nil)
			if err != nil {
				return
			}
			pb, err = self.Pile(bi)
			if err != nil {
				return
			}

			if f != nil && !f(p.A, p.B, pa, pb) {
				continue
			}
			if wp, ok := pm[pa]; !ok {
				tp := &Pile{
					Pile:   &feat.Feature{Location: pa.Segment(), Start: pa.Start(), End: pa.End()},
					Images: []*FeaturePair{p},
				}
				p.A.Meta = tp
				pm[pa] = tp
			} else {
				p.A.Meta = wp
				wp.Images = append(wp.Images, p)
			}
			if wp, ok := pm[pb]; !ok {
				tp := &Pile{
					Pile:   &feat.Feature{Location: pb.Segment(), Start: pb.Start(), End: pb.End()},
					Images: []*FeaturePair{p.Invert()},
				}
				p.B.Meta = tp
				pm[pb] = tp
			} else {
				p.B.Meta = wp
				wp.Images = append(wp.Images, p.Invert())
			}
		}
	}

	piles = make([]*Pile, 0, len(pm))
	for _, p := range pm {
		piles = append(piles, p)
	}

	return
}

func (self *Piler) Pile(i *interval.Interval) (p *interval.Interval, err error) {
	c := 0
	for p = range self.intervals.Intersect(i, 0) {
		// first sanity check: no interval described in the features should extend beyond the interval
		if i.Start() < p.Start() || i.End() > p.End() {
			return nil, fmt.Errorf("pals: internal inconsistency - escaped bounds: %s is outside %s", i, p)
		}
		c++
	}
	// second sanity check: no interval should overlap any other interval
	if c > 1 {
		return nil, fmt.Errorf("pals: internal inconsistency - too many results:", c)
	}

	return
}