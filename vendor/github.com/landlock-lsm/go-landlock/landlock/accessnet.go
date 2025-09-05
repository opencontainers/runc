package landlock

// AccessNetSet is a set of Landlockable network access rights.
type AccessNetSet uint64

var accessNetNames = []string{
	"bind_tcp",
	"connect_tcp",
}

var supportedAccessNet = AccessNetSet((1 << len(accessNetNames)) - 1)

func (a AccessNetSet) String() string {
	return accessSetString(uint64(a), accessNetNames)
}

func (a AccessNetSet) isSubset(b AccessNetSet) bool {
	return a&b == a
}

func (a AccessNetSet) intersect(b AccessNetSet) AccessNetSet {
	return a & b
}

func (a AccessNetSet) isEmpty() bool {
	return a == 0
}

func (a AccessNetSet) valid() bool {
	return a.isSubset(supportedAccessNet)
}
