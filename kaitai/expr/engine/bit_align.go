package engine

import (
	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/types"
)

// BitAlignSlot is a utility for calculating bit alignment slots. Zanbato uses
// bit alignment slots to improve round trip fidelity.
type BitAlignSlot struct {
	Index      int
	Width      int
	BeforeAttr int
}

// BitAlignSlots scans for bit alignment slots. These occur whenever there is a
// bitwise gap between two fields that can be determined statically. (This is
// not always possible due to e.g. conditional fields.)
func BitAlignSlots(ks *kaitai.Struct) []BitAlignSlot {
	var slots []BitAlignSlot
	runWidth := 0
	runDynamic := false
	runIndex := 0
	flush := func(at int) {
		if runWidth == 0 && !runDynamic {
			return
		}
		if !runDynamic && runWidth%8 == 0 {
			runWidth = 0
			return
		}
		if runDynamic {
			runDynamic = false
			runWidth = 0
			return
		}
		w := 8 - runWidth%8
		slots = append(slots, BitAlignSlot{
			Index:      runIndex,
			Width:      w,
			BeforeAttr: at,
		})
		runIndex++
		runWidth = 0
	}
	for i, attr := range ks.Seq {
		if attr == nil {
			continue
		}
		if bw, ok := staticBitWidth(attr); ok {
			runWidth += bw
			continue
		}
		if isBitAttr(attr) {
			runDynamic = true
			continue
		}
		flush(i)
	}
	flush(len(ks.Seq))
	return slots
}

func staticBitWidth(a *kaitai.Attr) (int, bool) {
	if a == nil {
		return 0, false
	}
	if a.If != nil || a.Repeat != nil {
		return 0, false
	}
	if a.Type.TypeRef == nil || a.Type.TypeRef.Kind != types.Bits {
		return 0, false
	}
	if a.Type.TypeRef.Bits == nil {
		return 0, false
	}
	w := a.Type.TypeRef.Bits.Width
	if w <= 0 || w > 64 {
		return 0, false
	}
	return w, true
}

func isBitAttr(a *kaitai.Attr) bool {
	if a == nil || a.Type.TypeRef == nil {
		return false
	}
	return a.Type.TypeRef.Kind == types.Bits
}
