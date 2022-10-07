package utf8bom

var (
	BOM = []byte{0xEF, 0xBB, 0xBF}
)

func AddBOM(in []byte) []byte {
	return append(BOM, in...)
}

func RemoveBOM(in []byte) []byte {
	if len(in) < 3 {
		return in
	}
	if in[0] == BOM[0] && in[1] == BOM[1] && in[2] == BOM[2] {
		return in[len(BOM):]
	}
	return in
}
