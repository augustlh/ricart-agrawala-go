package clock;

type LamportClock struct {
	time uint32;
}

func NewClock() LamportClock {
	return LamportClock {time: 0}
}

func (this *LamportClock) HappenedBefore(that *LamportClock) bool {
	return this.time < that.time
}

func (this *LamportClock) MergeWith(that *LamportClock) LamportClock {
	return LamportClock {time: max(this.time, that.time) + 1}
}

