package clock

type LamportClock struct {
	ticks uint64
}

func NewClock() LamportClock {
	return LamportClock{
		ticks: 0,
	}
}

func (lc *LamportClock) Now() uint64 {
	return lc.ticks
}

func (lc *LamportClock) Tick() uint64 {
	lc.ticks += 1
	return lc.ticks
}

func (lc *LamportClock) Sync(otherTicks uint64) {
	lc.ticks = max(lc.ticks, otherTicks) + 1
}
