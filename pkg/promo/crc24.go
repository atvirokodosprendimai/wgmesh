package promo

// CRC-24 checksum implementation (OpenPGP variant, polynomial 0x864CFB)
// Used for promo code validation to detect typos.

const (
	crc24Polynomial = 0x864CFB // OpenPGP CRC-24 polynomial
	crc24Init       = 0xB704CE // Initial value
	crc24XorOut     = 0x000000 // Final XOR value
)

// crc24Checksum computes the CRC-24 checksum of data.
func crc24Checksum(data []byte) uint32 {
	crc := uint32(crc24Init)

	for _, b := range data {
		crc ^= uint32(b) << 16

		for i := 0; i < 8; i++ {
			if (crc & 0x800000) != 0 {
				crc = ((crc << 1) ^ crc24Polynomial)
			} else {
				crc <<= 1
			}
		}
	}

	return uint32(crc & 0xFFFFFF) // Keep only 24 bits
}
