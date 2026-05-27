package c

func (e *Emitter) debugAttrStart(src *buf, ksID string) {
	if !e.debug {
		return
	}
	src.pf("zb_debug_attr_start(&this_->_debug, arena, %q, (int64_t)zb_stream_pos(stream));", ksID)
}

func (e *Emitter) debugAttrEnd(src *buf, ksID string) {
	if !e.debug {
		return
	}
	src.pf("zb_debug_attr_end(&this_->_debug, arena, %q, (int64_t)zb_stream_pos(stream));", ksID)
}

func (e *Emitter) debugArrInit(src *buf, ksID string) {
	if !e.debug {
		return
	}
	src.pf("zb_debug_arr_init(&this_->_debug, arena, %q);", ksID)
}

func (e *Emitter) debugArrElemStart(src *buf, ksID string) {
	if !e.debug {
		return
	}
	src.pf("zb_debug_arr_elem_start(&this_->_debug, arena, %q, (int64_t)zb_stream_pos(stream));", ksID)
}

func (e *Emitter) debugArrElemEnd(src *buf, ksID string) {
	if !e.debug {
		return
	}
	src.pf("zb_debug_arr_elem_end(&this_->_debug, arena, %q, (int64_t)zb_stream_pos(stream));", ksID)
}
