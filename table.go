package lua

type lValueArraySorter struct {
	L      *LState
	Fn     *LFunction
	Values []LValue
}

func (lv lValueArraySorter) Len() int {
	return len(lv.Values)
}

func (lv lValueArraySorter) Swap(i, j int) {
	lv.Values[i], lv.Values[j] = lv.Values[j], lv.Values[i]
}

func (lv lValueArraySorter) Less(i, j int) bool {
	if lv.Fn != nil {
		lv.L.Push(lv.Fn)
		lv.L.Push(lv.Values[i])
		lv.L.Push(lv.Values[j])
		lv.L.Call(2, 1)
		return LVAsBool(lv.L.reg.Pop())
	}
	return lessThan(lv.L, lv.Values[i], lv.Values[j])
}

func newLTable(acap int, hcap int) *LTable {
	if acap < 0 {
		acap = 0
	}
	if hcap < 0 {
		hcap = 0
	}
	tb := &LTable{
		array:     make([]LValue, 0, acap),
		dict:      make(map[LValue]LValue, hcap),
		strdict:   make(map[string]LValue, hcap),
		keys:      nil,
		k2i:       nil,
		Metatable: LNil,
	}
	return tb
}

func (tb *LTable) Len() int {
	var prev LValue = LNil
	for i := len(tb.array) - 1; i >= 0; i-- {
		v := tb.array[i]
		if prev == LNil && v != LNil {
			return i + 1
		}
		prev = v
	}
	return 0
}

func (tb *LTable) Append(value LValue) {
	tb.array = append(tb.array, value)
}

func (tb *LTable) Insert(i int, value LValue) {
	if i > len(tb.array) {
		tb.RawSetInt(i, value)
		return
	}
	if i <= 0 {
		tb.RawSet(LNumber(i), value)
		return
	}
	i -= 1
	tb.array = append(tb.array, LNil)
	copy(tb.array[i+1:], tb.array[i:])
	tb.array[i] = value
}

func (tb *LTable) MaxN() int {
	for i := len(tb.array) - 1; i >= 0; i-- {
		if tb.array[i] != LNil {
			return i + 1
		}
	}
	return 0
}

func (tb *LTable) Remove(pos int) LValue {
	i := pos - 1
	larray := len(tb.array)
	oldval := LNil
	switch {
	case i >= larray:
		// nothing to do
	case i == larray-1 || i < 0:
		oldval = tb.array[larray-1]
		tb.array = tb.array[:larray-1]
	default:
		oldval = tb.array[i]
		copy(tb.array[i:], tb.array[i+1:])
		tb.array[larray-1] = nil
		tb.array = tb.array[:larray-1]
	}
	return oldval
}

func (tb *LTable) RawSet(key LValue, value LValue) {
	switch v := key.(type) {
	case LNumber:
		if isArrayKey(v) {
			index := int(v) - 1
			alen := len(tb.array)
			switch {
			case index == alen:
				tb.array = append(tb.array, value)
			case index > alen:
				for i := 0; i < (index - alen); i++ {
					tb.array = append(tb.array, LNil)
				}
				tb.array = append(tb.array, value)
			case index < alen:
				tb.array[index] = value
			}
			return
		}
	case LString:
		tb.RawSetString(string(v), value)
		return
	}

	tb.RawSetH(key, value)
}

func (tb *LTable) RawSetInt(key int, value LValue) {
	if key < 1 || key >= MaxArrayIndex {
		tb.RawSetH(LNumber(key), value)
		return
	}
	index := key - 1
	alen := len(tb.array)
	switch {
	case index == alen:
		tb.array = append(tb.array, value)
	case index > alen:
		for i := 0; i < (index - alen); i++ {
			tb.array = append(tb.array, LNil)
		}
		tb.array = append(tb.array, value)
	case index < alen:
		tb.array[index] = value
	}
}

func (tb *LTable) RawSetString(key string, value LValue) {
	if value == LNil {
		delete(tb.strdict, key)
	} else {
		tb.strdict[key] = value
	}
}

func (tb *LTable) RawSetH(key LValue, value LValue) {
	if s, ok := key.(LString); ok {
		tb.RawSetString(string(s), value)
		return
	}

	if value == LNil {
		delete(tb.dict, key)
	} else {
		tb.dict[key] = value
	}
}

func (tb *LTable) RawGet(key LValue) LValue {
	switch v := key.(type) {
	case LNumber:
		if isArrayKey(v) {
			index := int(v) - 1
			if index >= len(tb.array) {
				return LNil
			}
			return tb.array[index]
		}
	case LString:
		if ret, ok := tb.strdict[string(v)]; ok {
			return ret
		}
		return LNil
	}
	if v, ok := tb.dict[key]; ok {
		return v
	}
	return LNil
}

func (tb *LTable) RawGetInt(key int) LValue {
	index := int(key) - 1
	if index >= len(tb.array) || index < 0 {
		return LNil
	}
	return tb.array[index]
}

func (tb *LTable) RawGetH(key LValue) LValue {
	if s, sok := key.(LString); sok {
		if v, vok := tb.strdict[string(s)]; vok {
			return v
		}
		return LNil
	}
	if v, ok := tb.dict[key]; ok {
		return v
	}
	return LNil
}

func (tb *LTable) RawGetString(key string) LValue {
	if v, vok := tb.strdict[string(key)]; vok {
		return v
	}
	return LNil
}

func (tb *LTable) ForEach(cb func(LValue, LValue)) {
	for i, v := range tb.array {
		if v != LNil {
			cb(LNumber(i+1), v)
		}
	}
	for k, v := range tb.strdict {
		if v != LNil {
			cb(LString(k), v)
		}
	}
	for k, v := range tb.dict {
		if v != LNil {
			cb(k, v)
		}
	}
}

func (tb *LTable) Next(key LValue) (LValue, LValue) {
	// TODO: inefficient way
	if key == LNil {
		tb.keys = nil
		tb.k2i = nil
		key = LNumber(0)
	}

	if tb.keys == nil {
		tb.keys = make([]LValue, len(tb.dict)+len(tb.strdict))
		tb.k2i = make(map[LValue]int)
		i := 0
		for k, _ := range tb.dict {
			tb.keys[i] = k
			tb.k2i[k] = i
			i++
		}
		for k, _ := range tb.strdict {
			tb.keys[i] = LString(k)
			tb.k2i[LString(k)] = i
			i++
		}
	}

	if kv, ok := key.(LNumber); ok && isInteger(kv) && int(kv) >= 0 {
		index := int(kv)
		for ; index < len(tb.array); index++ {
			if v := tb.array[index]; v != LNil {
				return LNumber(index + 1), v
			}
		}
		if index == len(tb.array) {
			if len(tb.dict) == 0 && len(tb.strdict) == 0 {
				tb.keys = nil
				tb.k2i = nil
				return LNil, LNil
			}
			key = tb.keys[0]
			if skey, sok := key.(LString); sok {
				if sv, svok := tb.strdict[string(skey)]; svok && sv != LNil {
					return key, sv
				}
			} else if v, vok := tb.dict[key]; vok && v != LNil {
				return key, v
			}
		}
	}
	length := len(tb.dict) + len(tb.strdict)
	for i := tb.k2i[key] + 1; i < length; i++ {
		key = tb.keys[i]
		if skey, sok := key.(LString); sok {
			if sv, svok := tb.strdict[string(skey)]; svok && sv != LNil {
				return key, sv
			}
		} else if v, vok := tb.dict[key]; vok && v != LNil {
			return key, v
		}
	}
	tb.keys = nil
	tb.k2i = nil
	return LNil, LNil
}
