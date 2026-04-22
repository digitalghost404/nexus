package embed

import (
	"encoding/binary"
	"math"
)

func float64SliceToBlob(vec []float64) []byte {
	blob := make([]byte, len(vec)*8)
	for i, v := range vec {
		binary.LittleEndian.PutUint64(blob[i*8:], math.Float64bits(v))
	}
	return blob
}

func BlobToFloat64Slice(blob []byte) []float64 {
	vec := make([]float64, len(blob)/8)
	for i := 0; i < len(vec); i++ {
		vec[i] = math.Float64frombits(binary.LittleEndian.Uint64(blob[i*8:]))
	}
	return vec
}
