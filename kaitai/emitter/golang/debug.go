package golang

func (e *Emitter) debugAttrStart(fn *goFunc, ksFieldName string) {
	fn.pf("{_pos, _ := stream.Pos(); this.AttrStart_[%q] = _pos}", ksFieldName)
}

func (e *Emitter) debugAttrEnd(fn *goFunc, ksFieldName string) {
	fn.pf("{_pos, _ := stream.Pos(); this.AttrEnd_[%q] = _pos}", ksFieldName)
}

func (e *Emitter) debugArrInit(fn *goFunc, ksFieldName string) {
	fn.pf("this.ArrStart_[%q] = nil", ksFieldName)
	fn.pf("this.ArrEnd_[%q] = nil", ksFieldName)
}

func (e *Emitter) debugArrElemStart(fn *goFunc, ksFieldName string) {
	fn.pf("{_pos, _ := stream.Pos(); this.ArrStart_[%q] = append(this.ArrStart_[%q], _pos)}", ksFieldName, ksFieldName)
}

func (e *Emitter) debugArrElemEnd(fn *goFunc, ksFieldName string) {
	fn.pf("{_pos, _ := stream.Pos(); this.ArrEnd_[%q] = append(this.ArrEnd_[%q], _pos)}", ksFieldName, ksFieldName)
}
