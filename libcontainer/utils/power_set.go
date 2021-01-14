package utils

type outputFunc func([]uint64) error

// PowerSet iterates over the power set of the provided objects (in other
// words, the set of all possible subsets of the objects). Note that for each
// object added to the set, the number of possible values doubles.
func PowerSet(objects []uint64, outputFn outputFunc) error {
	N := len(objects)
	set := make([]uint64, 0, N)
	for mask := 0; mask < (1 << N); mask++ {
		// Clear the set and compute it.
		set = set[:0]
		for i := 0; i < N; i++ {
			bit := 1 << i
			if mask&bit != 0 {
				set = append(set, objects[i])
			}
		}
		// Output the set.
		if err := outputFn(set); err != nil {
			return err
		}
	}
	return nil
}

// BitPermutations iterates through all possible values of the given bitmask.
func BitPowerSet(mask uint64, bitFn func(mask uint64) error) error {
	// Generate the bit set.
	var objects []uint64
	for bitIdx := 0; bitIdx < 64; bitIdx++ {
		bit := uint64(1 << bitIdx)
		if mask&bit != 0 {
			objects = append(objects, bit)
		}
	}
	return PowerSet(objects, func(bits []uint64) error {
		var total uint64
		for _, bit := range bits {
			total |= bit
		}
		return bitFn(total)
	})
}
