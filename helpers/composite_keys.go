package helpers

func Key16_16(key1 uint16, key2 uint16) uint64 {
	return (uint64(key1) << 48) | (uint64(key2) << 32)
}
func Key16_32(key1 uint16, key2 uint32) uint64 {
	return (uint64(key1) << 48) | (uint64(key2) << 16)
}
func Key32_32(key1 uint32, key2 uint32) uint64 {
	return (uint64(key1) << 32) | uint64(key2)
}
func Key32_16_16(key1 uint32, key2 uint16, key3 uint16) uint64 {
	return (uint64(key1) << 32) | (uint64(key2) << 16) | (uint64(key3))
}
func Key16_16_32(key1 uint16, key2 uint16, key3 uint32) uint64 {
	return (uint64(key1) << 48) | (uint64(key2) << 32) | uint64(key3)
}
func Key16_32_16(key1 uint16, key2 uint32, key3 uint16) uint64 {
	return (uint64(key1) << 48) | (uint64(key2) << 16) | (uint64(key3))
}
func Key8_16(key1 uint8, key2 uint16) uint64 {
	return (uint64(key1) << 56) | (uint64(key2) << 40)
}
func Key8_32(key1 uint8, key2 uint32) uint64 {
	return (uint64(key1) << 56) | (uint64(key2) << 24)
}

const CLEAR8 = 0xFF
const CLEAR16 = 0xFFFF
const CLEAR32 = 0xFFFFFFFF
const CLEAR48 = 0xFFFFFFFFFFFF

func ParseKey8_16(key uint64) (uint8, uint16) {
	var key1 = uint8(key >> 56)
	var key2 = uint16((key >> 40) & CLEAR8)
	return key1, key2
}
func ParseKey8_32(key uint64) (uint8, uint32) {
	var key1 = uint8(key >> 56)
	var key2 = uint32((key >> 24) & CLEAR8)
	return key1, key2
}
func ParseKey16_16(key uint64) (uint16, uint16) {
	var key1 = uint16(key >> 48)
	var key2 = uint16((key >> 32) & CLEAR16)
	return key1, key2
}
func ParseKey16_32(key uint64) (uint16, uint32) {
	var key1 = uint16(key >> 48)
	var key2 = uint32((key >> 16) & CLEAR16)
	return key1, key2
}
func ParseKey32_32(key uint64) (uint32, uint32) {
	var key1 = uint32(key >> 32)
	var key2 = uint32(key & CLEAR32)
	return key1, key2
}
func ParseKey32_16_16(key uint64) (uint32, uint16, uint16) {
	var key1 = uint32(key >> 32)
	var key2 = uint16((key >> 16) & CLEAR32)
	var key3 = uint16(key & CLEAR48)
	return key1, key2, key3
}
func ParseKey16_16_32(key uint64) (uint16, uint16, uint32) {
	var key1 = uint16(key >> 48)
	var key2 = uint16((key >> 32) & CLEAR16)
	var key3 = uint32(key & CLEAR32)
	return key1, key2, key3
}
func ParseKey16_32_16(key uint64) (uint16, uint32, uint16) {
	var key1 = uint16(key >> 48)
	var key2 = uint32((key >> 16) & CLEAR16)
	var key3 = uint16(key & CLEAR48)
	return key1, key2, key3
}
