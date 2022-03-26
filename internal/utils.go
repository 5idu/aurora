package internal

// Ascii numbers 0-9
const (
	asciiZero = 48
	asciiNine = 57
)

// parseSizeFromBuf expects decimal positive numbers. We
// return -1 to signal error
func parseSizeFromBuf(d []byte) (n int) {
	if len(d) == 0 {
		return -1
	}
	for _, dec := range d {
		if dec < asciiZero || dec > asciiNine {
			return -1
		}
		n = n*10 + (int(dec) - asciiZero)
	}
	return n
}
