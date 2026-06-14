package helpers

import (
	"testing"
)

func TestParseDisposition(t *testing.T) {
	t.Run("16_16", func(t *testing.T) {
		var first = uint16(5)
		var second = uint16(10)
		var key = Key16_16(first, second)
		var pFirst, pSecond = ParseKey16_16(key)
		if pFirst != first {
			t.Errorf("returned first: %d, expected first: %d", pFirst, first)
		}
		if pSecond != second {
			t.Errorf("returned first: %d, expected first: %d", pSecond, second)
		}
	})
	t.Run("16_32", func(t *testing.T) {
		var first = uint16(5)
		var second = uint32(10)
		var key = Key16_32(first, second)
		var pFirst, pSecond = ParseKey16_32(key)
		if pFirst != first {
			t.Errorf("returned first: %d, expected first: %d", pFirst, first)
		}
		if pSecond != second {
			t.Errorf("returned first: %d, expected first: %d", pSecond, second)
		}
	})
	t.Run("32_32", func(t *testing.T) {
		var first = uint32(5)
		var second = uint32(10)
		var key = Key32_32(first, second)
		var pFirst, pSecond = ParseKey32_32(key)
		if pFirst != first {
			t.Errorf("returned first: %d, expected first: %d", pFirst, first)
		}
		if pSecond != second {
			t.Errorf("returned first: %d, expected first: %d", pSecond, second)
		}
	})
	t.Run("8_32", func(t *testing.T) {
		var first = uint8(5)
		var second = uint32(10)
		var key = Key8_32(first, second)
		var pFirst, pSecond = ParseKey8_32(key)
		if pFirst != first {
			t.Errorf("returned first: %d, expected first: %d", pFirst, first)
		}
		if pSecond != second {
			t.Errorf("returned first: %d, expected first: %d", pSecond, second)
		}
	})
	t.Run("32_16_16", func(t *testing.T) {
		var first = uint32(5)
		var second = uint16(10)
		var third = uint16(15)
		var key = Key32_16_16(first, second, third)
		var pFirst, pSecond, pThird = ParseKey32_16_16(key)
		if pFirst != first {
			t.Errorf("returned first: %d, expected first: %d", pFirst, first)
		}
		if pSecond != second {
			t.Errorf("returned first: %d, expected first: %d", pSecond, second)
		}
		if pThird != third {
			t.Errorf("returned third: %d, expected third: %d", pThird, third)

		}
	})

	t.Run("16_16_32", func(t *testing.T) {
		var first = uint16(5)
		var second = uint16(10)
		var third = uint32(15)
		var key = Key16_16_32(first, second, third)
		var pFirst, pSecond, pThird = ParseKey16_16_32(key)
		if pFirst != first {
			t.Errorf("returned first: %d, expected first: %d", pFirst, first)
		}
		if pSecond != second {
			t.Errorf("returned first: %d, expected first: %d", pSecond, second)
		}
		if pThird != third {
			t.Errorf("returned third: %d, expected third: %d", pThird, third)

		}
	})

	t.Run("16_32_16", func(t *testing.T) {
		var first = uint16(5)
		var second = uint32(10)
		var third = uint16(15)
		var key = Key16_32_16(first, second, third)
		var pFirst, pSecond, pThird = ParseKey16_32_16(key)
		if pFirst != first {
			t.Errorf("returned first: %d, expected first: %d", pFirst, first)
		}
		if pSecond != second {
			t.Errorf("returned first: %d, expected first: %d", pSecond, second)
		}
		if pThird != third {
			t.Errorf("returned third: %d, expected third: %d", pThird, third)

		}
	})

}
